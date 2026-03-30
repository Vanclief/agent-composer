package harnesses

import "testing"

func TestParseClaudeCodeResultParsesUsageAndSession(t *testing.T) {
	rawOutput := "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"duration_ms\":29,\"duration_api_ms\":0,\"num_turns\":1,\"result\":\"Bonjour\",\"stop_reason\":\"stop_sequence\",\"session_id\":\"bbc1f8cc-1911-4435-9d66-a83cbe5d4531\",\"total_cost_usd\":0.003,\"usage\":{\"input_tokens\":101,\"cache_creation_input_tokens\":0,\"cache_read_input_tokens\":12,\"output_tokens\":7}}\n"

	result, err := parseClaudeCodeResult(rawOutput)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if result.SessionID != "bbc1f8cc-1911-4435-9d66-a83cbe5d4531" {
		t.Fatalf("unexpected session id: %q", result.SessionID)
	}

	if result.Result != "Bonjour" {
		t.Fatalf("unexpected result: %q", result.Result)
	}

	if result.Usage.InputTokens != 101 {
		t.Fatalf("unexpected input tokens: %d", result.Usage.InputTokens)
	}

	if result.Usage.CacheReadInputTokens != 12 {
		t.Fatalf("unexpected cached tokens: %d", result.Usage.CacheReadInputTokens)
	}

	if result.Usage.OutputTokens != 7 {
		t.Fatalf("unexpected output tokens: %d", result.Usage.OutputTokens)
	}
}

func TestParseClaudeCodeConfigRejectsInvalidPermissionMode(t *testing.T) {
	_, err := parseClaudeCodeConfig([]byte(`{"permission_mode":"forbidden"}`))
	if err == nil {
		t.Fatal("expected error for invalid permission mode")
	}
}
