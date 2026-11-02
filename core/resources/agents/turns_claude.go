package agents

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// Claude Code writes one JSON object per line to
// ~/.claude/projects/<cwd>/<session>.jsonl. The lines that make turns:
//
//   - `type: user` with message.content a string or text blocks: what was
//     sent, a turn starts here. With tool_result blocks instead it carries
//     results, keyed by tool_use_id. `isMeta`, `isCompactSummary` and
//     `isSidechain` (a subagent's line) lines are not turns, nor is the
//     harness's own "[Request interrupted by user]".
//   - `type: assistant`: one line per content block (thinking, text,
//     tool_use{id,name,input}), the lines of one model call sharing
//     message.id and repeating its message.usage, so usage is counted once
//     per id.
//
// Every other kind (attachment, queue-operation, api-request*, last-prompt,
// summary, system, ...) is skipped.

type claudeLine struct {
	Type             string        `json:"type"`
	SessionID        string        `json:"sessionId"`
	Timestamp        string        `json:"timestamp"`
	IsSidechain      bool          `json:"isSidechain"`
	IsMeta           bool          `json:"isMeta"`
	IsCompactSummary bool          `json:"isCompactSummary"`
	Message          claudeMessage `json:"message"`
}

type claudeMessage struct {
	ID      string          `json:"id"`
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Usage   *claudeUsage    `json:"usage"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// claudeBlocks reads message.content: a string is one text block.
func claudeBlocks(content json.RawMessage) []claudeBlock {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return nil
	}

	if trimmed[0] == '"' {
		text := ""
		_ = json.Unmarshal(trimmed, &text)
		return []claudeBlock{{Type: "text", Text: text}}
	}

	blocks := []claudeBlock{}
	_ = json.Unmarshal(trimmed, &blocks)

	return blocks
}

// claudeInjected is the text a user line sent, and whether it starts a
// turn: it has text or an image and carries no tool result.
func claudeInjected(line claudeLine) (string, bool) {
	if line.Type != "user" || line.IsSidechain || line.IsMeta || line.IsCompactSummary {
		return "", false
	}

	parts := []string{}
	for _, block := range claudeBlocks(line.Message.Content) {
		switch block.Type {
		case "tool_result":
			return "", false
		case "text":
			parts = append(parts, block.Text)
		case "image":
			parts = append(parts, "[image]")
		}
	}

	text := strings.Join(parts, "\n\n")
	if strings.TrimSpace(text) == "" || strings.HasPrefix(text, "[Request interrupted by user") {
		return "", false
	}

	return text, true
}

// claudeTurn is a turn being read, with what pairs its tool calls and
// counts its usage once per model call.
type claudeTurn struct {
	turn     Turn
	calls    map[string]int
	usage    map[string]claudeUsage
	order    []string
	answerID string
}

func (t *claudeTurn) finish() Turn {
	turn := t.turn
	if len(t.order) > 0 {
		tokens := Tokens{}
		for _, id := range t.order {
			u := t.usage[id]
			tokens.add(Tokens{Input: u.InputTokens, Output: u.OutputTokens, CacheRead: u.CacheReadInputTokens, CacheWrite: u.CacheCreationInputTokens})
		}
		turn.Tokens = &tokens
	}

	return turn
}

func parseClaude(r io.Reader, full bool) parsedFile {
	parsed := parsedFile{}
	var current *claudeTurn

	flush := func() {
		if current != nil {
			parsed.turns = append(parsed.turns, current.finish())
		}
		current = nil
	}

	parsed.skipped = eachLine(r, func(raw []byte) {
		line := claudeLine{}
		err := json.Unmarshal(raw, &line)
		if err != nil {
			// A kind whose fields have other shapes: not one of ours.
			return
		}

		if line.Type != "user" && line.Type != "assistant" {
			return
		}

		if parsed.session == "" {
			parsed.session = line.SessionID
		}

		at := parseTime(line.Timestamp)
		text, starts := claudeInjected(line)
		if starts {
			flush()
			current = &claudeTurn{
				turn:  Turn{StartedAt: at, EndedAt: at, Session: line.SessionID, Injected: text, ToolCalls: []ToolCall{}},
				calls: map[string]int{},
				usage: map[string]claudeUsage{},
			}
			return
		}

		if current == nil || line.IsSidechain || !full {
			return
		}

		if !at.IsZero() {
			current.turn.EndedAt = at
		}

		if line.Type == "assistant" {
			current.assistant(line)
			return
		}

		current.results(line)
	})

	flush()

	return parsed
}

func (t *claudeTurn) assistant(line claudeLine) {
	id := line.Message.ID
	if line.Message.Usage != nil && id != "" {
		_, seen := t.usage[id]
		if !seen {
			t.order = append(t.order, id)
		}
		t.usage[id] = *line.Message.Usage
	}

	for _, block := range claudeBlocks(line.Message.Content) {
		switch block.Type {
		case "text":
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			// The answer is the last model call's text; the blocks of one
			// call, on separate lines, join.
			if t.answerID == id && id != "" && t.turn.Answer != "" {
				t.turn.Answer += "\n\n" + block.Text
			} else {
				t.turn.Answer = block.Text
			}
			t.answerID = id
		case "tool_use":
			t.calls[block.ID] = len(t.turn.ToolCalls)
			t.turn.ToolCalls = append(t.turn.ToolCalls, ToolCall{Name: block.Name, Args: rawArgs(block.Input)})
		}
	}
}

func (t *claudeTurn) results(line claudeLine) {
	for _, block := range claudeBlocks(line.Message.Content) {
		if block.Type != "tool_result" {
			continue
		}

		i, ok := t.calls[block.ToolUseID]
		if !ok {
			continue
		}

		call := &t.turn.ToolCalls[i]
		call.Result, call.Truncated = cutResult(textOf(block.Content))
		call.Error = block.IsError
	}
}
