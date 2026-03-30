package harnesses

import "testing"

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
