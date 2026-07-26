package harnesses

import (
	"encoding/json"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
)

// TraceEvent is one step of a harness run, parsed from the raw output
// stream: the model's reasoning summaries, executed commands, emitted
// messages, and errors.
type TraceEvent struct {
	Kind    string `json:"kind"` // reasoning | message | command | tool | error
	Content string `json:"content"`
	Detail  string `json:"detail,omitempty"`
}

const traceDetailLimit = 4000

// ParseTrace extracts trace events from a conversation's raw harness
// output. Codex emits a JSONL event stream (`codex exec --json`).
// Claude Code currently runs with a single JSON result — no per-step
// events are recorded, so its trace is empty.
func ParseTrace(harness agent.Harness, rawOutput string) []TraceEvent {
	if harness != agent.HarnessCodexCLI {
		return nil
	}
	return parseCodexTrace(rawOutput)
}

func parseCodexTrace(rawOutput string) []TraceEvent {
	events := []TraceEvent{}
	// item.started/updated/completed repeat the same item id; the last
	// version wins but keeps its original position in the stream.
	positionByID := map[string]int{}

	upsert := func(id string, event TraceEvent) {
		if id == "" {
			events = append(events, event)
			return
		}
		if position, seen := positionByID[id]; seen {
			events[position] = event
			return
		}
		positionByID[id] = len(events)
		events = append(events, event)
	}

	for _, line := range strings.Split(rawOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var payload map[string]any
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			continue
		}

		if event, id, ok := codexTraceEvent(payload); ok {
			upsert(id, event)
		}
	}

	return events
}

func codexTraceEvent(payload map[string]any) (TraceEvent, string, bool) {
	// The item event shape: {"type":"item.*","item":{...}}; some codex
	// versions emit the item fields at the top level instead.
	item := payload
	if child, ok := payload["item"].(map[string]any); ok {
		item = child
	}

	id, _ := item["id"].(string)
	itemType := findFirstString(item, "item_type", "type")

	switch {
	case strings.Contains(itemType, "reasoning"):
		text := findFirstString(item, "text", "summary")
		if text == "" {
			return TraceEvent{}, "", false
		}
		return TraceEvent{Kind: "reasoning", Content: text}, id, true

	case itemType == "agent_message" || itemType == "assistant_message":
		text := findFirstString(item, "text")
		if text == "" {
			return TraceEvent{}, "", false
		}
		return TraceEvent{Kind: "message", Content: text}, id, true

	case strings.Contains(itemType, "command"):
		command := findFirstString(item, "command")
		if command == "" {
			return TraceEvent{}, "", false
		}
		detail := findFirstString(item, "aggregated_output", "output")
		if len(detail) > traceDetailLimit {
			detail = detail[:traceDetailLimit] + "\n… (truncated)"
		}
		return TraceEvent{
			Kind:    "command",
			Content: "$ " + command,
			Detail:  detail,
		}, id, true

	case strings.Contains(itemType, "mcp_tool") || strings.Contains(itemType, "tool_call"):
		name := findFirstString(item, "tool", "name", "server")
		if name == "" {
			return TraceEvent{}, "", false
		}
		return TraceEvent{Kind: "tool", Content: name}, id, true

	case itemType == "error":
		message := findFirstString(payload, "message")
		if message == "" {
			return TraceEvent{}, "", false
		}
		return TraceEvent{Kind: "error", Content: message}, id, true
	}

	return TraceEvent{}, "", false
}
