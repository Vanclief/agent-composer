package harnesses

import (
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
)

func TestParseCodexTrace(t *testing.T) {
	raw := `{"type":"thread.started","thread_id":"t1"}
{"type":"item.started","item":{"id":"i1","item_type":"agent_reasoning","text":"Thinking about"}}
{"type":"item.completed","item":{"id":"i1","item_type":"agent_reasoning","text":"Thinking about the diff"}}
{"type":"item.completed","item":{"id":"i2","item_type":"command_execution","command":"git diff --stat","aggregated_output":"3 files changed","exit_code":0}}
{"type":"error","message":"stream hiccup"}
{"type":"item.completed","item":{"id":"i3","item_type":"agent_message","text":"Found two issues."}}
not json
`

	events := ParseTrace(agent.HarnessCodexCLI, raw)
	if len(events) != 4 {
		t.Fatalf("want 4 events, got %d: %+v", len(events), events)
	}
	if events[0].Kind != "reasoning" ||
		events[0].Content != "Thinking about the diff" {
		t.Fatalf("reasoning event wrong (dedupe by id failed): %+v", events[0])
	}
	if events[1].Kind != "command" ||
		events[1].Content != "$ git diff --stat" ||
		events[1].Detail != "3 files changed" {
		t.Fatalf("command event wrong: %+v", events[1])
	}
	if events[2].Kind != "error" || events[2].Content != "stream hiccup" {
		t.Fatalf("error event wrong: %+v", events[2])
	}
	if events[3].Kind != "message" || events[3].Content != "Found two issues." {
		t.Fatalf("message event wrong: %+v", events[3])
	}
}

func TestParseTraceClaudeIsEmpty(t *testing.T) {
	events := ParseTrace(agent.HarnessClaudeCode, `{"result":"done"}`)
	if len(events) != 0 {
		t.Fatalf("claude trace should be empty, got %+v", events)
	}
}
