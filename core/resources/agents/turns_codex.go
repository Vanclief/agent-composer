package agents

import (
	"encoding/json"
	"io"
	"strings"
)

// Codex writes ~/.codex/sessions/YYYY/MM/DD/rollout-<time>-<session>.jsonl,
// one `{ timestamp, type, payload }` per line (seen from codex-cli 0.166):
//
//   - `session_meta`: payload.id is the session id.
//   - `response_item`: what the model saw and said. payload.type `message`
//     with role user (content input_text; a turn starts here unless the
//     text is context Codex injects itself, such as <environment_context>
//     or AGENTS.md), assistant (output_text: the answer) or developer;
//     `function_call` { name, arguments (a JSON string), call_id } and
//     `function_call_output` { call_id, output }; `custom_tool_call`
//     { name, input, call_id } and `custom_tool_call_output`;
//     `local_shell_call` { call_id, action }. reasoning and the rest are
//     skipped.
//   - `event_msg` with payload.type `token_count`: info.total_token_usage
//     is the session's running total { input_tokens (cached included),
//     cached_input_tokens, output_tokens }, so a turn's tokens are the
//     total at its end less the total before it.
//
// Every other kind (turn_context, world_state, other event_msg, ...) is
// skipped. Older rollouts wrote the response items bare, without the
// envelope; those are read too. Only a session without a model reply could
// be produced in the sandbox this was written in, so tool calls and token
// counts follow Codex's documented format, untested on a real file.

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Input     json.RawMessage `json:"input"`
	Action    json.RawMessage `json:"action"`
	CallID    string          `json:"call_id"`
	Output    json.RawMessage `json:"output"`
	Info      *struct {
		Total *codexUsage `json:"total_token_usage"`
	} `json:"info"`
}

type codexUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
}

// codexContext are the starts of user messages Codex writes itself.
var codexContext = []string{
	"<environment_context>",
	"<user_instructions>",
	"<user_shell_command>",
	"<turn_aborted>",
	"<skills_instructions>",
	"<subagent_notification>",
	"<permissions instructions>",
	"<collaboration_mode>",
	"<multi_agent",
	"<apps_instructions>",
	"# AGENTS.md instructions for ",
}

func codexInjected(payload codexPayload) (string, bool) {
	if payload.Type != "message" || payload.Role != "user" {
		return "", false
	}

	text := textOf(payload.Content)
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}

	for _, prefix := range codexContext {
		if strings.HasPrefix(trimmed, prefix) {
			return "", false
		}
	}

	return text, true
}

type codexTurn struct {
	turn     Turn
	calls    map[string]int
	baseline codexUsage
	last     *codexUsage
}

func parseCodex(r io.Reader, full bool) parsedFile {
	parsed := parsedFile{}
	var current *codexTurn
	var total *codexUsage

	flush := func() {
		if current != nil {
			turn := current.turn
			if current.last != nil {
				turn.Tokens = &Tokens{
					Input:     (current.last.InputTokens - current.last.CachedInputTokens) - (current.baseline.InputTokens - current.baseline.CachedInputTokens),
					Output:    current.last.OutputTokens - current.baseline.OutputTokens,
					CacheRead: current.last.CachedInputTokens - current.baseline.CachedInputTokens,
				}
			}
			parsed.turns = append(parsed.turns, turn)
		}
		current = nil
	}

	parsed.skipped = eachLine(r, func(raw []byte) {
		line := codexLine{}
		err := json.Unmarshal(raw, &line)
		if err != nil {
			return
		}

		payloadRaw := line.Payload
		kind := line.Type
		if len(payloadRaw) == 0 {
			// An older rollout: the item itself, no envelope.
			payloadRaw = raw
			kind = "response_item"
		}

		payload := codexPayload{}
		err = json.Unmarshal(payloadRaw, &payload)
		if err != nil {
			return
		}

		if kind == "session_meta" {
			if parsed.session == "" {
				parsed.session = payload.ID
			}
			return
		}

		at := parseTime(line.Timestamp)

		if kind == "event_msg" && payload.Type == "token_count" && payload.Info != nil && payload.Info.Total != nil {
			usage := *payload.Info.Total
			total = &usage
			if current != nil {
				current.last = &usage
			}
			return
		}

		if kind != "response_item" {
			return
		}

		text, starts := codexInjected(payload)
		if starts {
			flush()
			current = &codexTurn{
				turn:  Turn{StartedAt: at, EndedAt: at, Injected: text, ToolCalls: []ToolCall{}},
				calls: map[string]int{},
			}
			if total != nil {
				current.baseline = *total
			}
			return
		}

		if current == nil || !full {
			return
		}

		if !at.IsZero() {
			current.turn.EndedAt = at
		}

		current.item(payload)
	})

	flush()

	return parsed
}

func (t *codexTurn) item(payload codexPayload) {
	switch payload.Type {
	case "message":
		if payload.Role != "assistant" {
			return
		}
		text := textOf(payload.Content)
		if strings.TrimSpace(text) != "" {
			t.turn.Answer = text
		}
	case "function_call":
		t.call(payload.CallID, payload.Name, rawArgs(payload.Arguments))
	case "custom_tool_call":
		t.call(payload.CallID, payload.Name, rawArgs(payload.Input))
	case "local_shell_call":
		t.call(payload.CallID, "shell", rawArgs(payload.Action))
	case "function_call_output", "custom_tool_call_output":
		i, ok := t.calls[payload.CallID]
		if !ok {
			return
		}
		call := &t.turn.ToolCalls[i]
		call.Result, call.Truncated = cutResult(textOf(payload.Output))
	}
}

func (t *codexTurn) call(id string, name string, args json.RawMessage) {
	t.calls[id] = len(t.turn.ToolCalls)
	t.turn.ToolCalls = append(t.turn.ToolCalls, ToolCall{Name: name, Args: args})
}
