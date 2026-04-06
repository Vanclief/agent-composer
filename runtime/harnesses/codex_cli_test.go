package harnesses

import (
	"encoding/json"
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
)

func TestParseCodexRunSummaryParsesCachedInputTokens(t *testing.T) {
	rawOutput := "{\"type\":\"thread.started\",\"thread_id\":\"019d4368-c682-7892-8151-16665ad317e2\"}\n" +
		"{\"type\":\"turn.started\"}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"id\":\"item_0\",\"type\":\"agent_message\",\"text\":\"Bonjour\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":16069,\"cached_input_tokens\":6528,\"output_tokens\":219}}\n"

	summary := parseCodexRunSummary(rawOutput)

	if summary.SessionRef != "019d4368-c682-7892-8151-16665ad317e2" {
		t.Fatalf("unexpected session ref: %q", summary.SessionRef)
	}

	if summary.InputTokens != 16069 {
		t.Fatalf("unexpected input tokens: %d", summary.InputTokens)
	}

	if summary.CachedTokens != 6528 {
		t.Fatalf("unexpected cached tokens: %d", summary.CachedTokens)
	}

	if summary.OutputTokens != 219 {
		t.Fatalf("unexpected output tokens: %d", summary.OutputTokens)
	}
}

func TestBuildArgsIncludesOutputSchemaWhenStructuredOutputSchemaIsPresent(t *testing.T) {
	conversation := &agent.Conversation{
		Model:                  "gpt-5.4-mini",
		StructuredOutput:       true,
		StructuredOutputSchema: json.RawMessage(`{"type":"object"}`),
	}

	args := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{}, "prompt", "/tmp/last-message.txt", "/tmp/schema.json")

	found := false
	for index := 0; index < len(args)-1; index++ {
		if args[index] == "--output-schema" && args[index+1] == "/tmp/schema.json" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected --output-schema in args: %#v", args)
	}
}

func TestSelectCodexHarnessErrorPrefersStructuredSummaryErrors(t *testing.T) {
	summary := codexRunSummary{
		Errors: []string{
			`{"type":"error","error":{"message":"Invalid schema for response_format 'codex_output_schema'"}}`,
		},
	}

	harnessError := selectCodexHarnessError("Reading additional input from stdin...", summary)

	if harnessError != "Invalid schema for response_format 'codex_output_schema'" {
		t.Fatalf("unexpected harness error: %q", harnessError)
	}
}
