package harnesses

import (
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
)

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

func TestParseClaudeCodeConfigRejectsLegacyPermissionMode(t *testing.T) {
	_, err := parseClaudeCodeConfig([]byte(`{"permission_mode":"bypassPermissions"}`))
	if err == nil {
		t.Fatal("expected error for legacy permission_mode key")
	}
}

func TestParseClaudeCodeConfigRejectsInvalidPermissions(t *testing.T) {
	_, err := parseClaudeCodeConfig([]byte(`{"permissions":"write-everything"}`))
	if err == nil {
		t.Fatal("expected error for invalid permissions tier")
	}
}

func TestParseClaudeCodeConfigDefaultsToReadOnly(t *testing.T) {
	cfg, err := parseClaudeCodeConfig([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Permissions != PermissionsReadOnly {
		t.Fatalf("expected read_only default, got %q", cfg.Permissions)
	}
}

func TestClaudeBuildArgsReadOnlyDisallowsWriteTools(t *testing.T) {
	conversation := &agent.Conversation{Model: "claude-opus-4-8"}

	args := (&ClaudeCode{}).buildArgs(conversation, claudeCodeConfig{Permissions: PermissionsReadOnly}, "prompt")

	if !argsContainPair(args, "--permission-mode", "default") {
		t.Fatalf("expected --permission-mode default: %#v", args)
	}
	if !argsContainPair(args, "--disallowedTools", "Bash") {
		t.Fatalf("expected Bash to be disallowed: %#v", args)
	}
	if argsContain(args, "--dangerously-skip-permissions") {
		t.Fatalf("did not expect skip-permissions for read_only: %#v", args)
	}
}

func TestClaudeBuildArgsExecBypassesPermissions(t *testing.T) {
	conversation := &agent.Conversation{Model: "claude-opus-4-8"}

	args := (&ClaudeCode{}).buildArgs(conversation, claudeCodeConfig{Permissions: PermissionsExec}, "prompt")

	if !argsContainPair(args, "--permission-mode", "bypassPermissions") {
		t.Fatalf("expected --permission-mode bypassPermissions: %#v", args)
	}
	if argsContain(args, "--disallowedTools") {
		t.Fatalf("did not expect disallowed tools for exec: %#v", args)
	}
}

func TestClaudeBuildArgsDangerouslyExecSkipsPermissions(t *testing.T) {
	conversation := &agent.Conversation{Model: "claude-opus-4-8"}

	args := (&ClaudeCode{}).buildArgs(conversation, claudeCodeConfig{Permissions: PermissionsDangerouslyExec}, "prompt")

	if !argsContain(args, "--dangerously-skip-permissions") {
		t.Fatalf("expected --dangerously-skip-permissions: %#v", args)
	}
	if argsContain(args, "--permission-mode") {
		t.Fatalf("did not expect --permission-mode for dangerously-exec: %#v", args)
	}
}

func TestClaudeBuildArgsTerminatesOptionsBeforePrompt(t *testing.T) {
	conversation := &agent.Conversation{Model: "claude-opus-4-8", Instructions: "do the thing"}

	args := (&ClaudeCode{}).buildArgs(conversation, claudeCodeConfig{Permissions: PermissionsReadOnly}, "prompt")

	// The prompt must be the final arg and be preceded by "--" so variadic tool
	// flags (e.g. --disallowedTools) cannot swallow it.
	if len(args) < 2 {
		t.Fatalf("unexpected args: %#v", args)
	}
	if args[len(args)-2] != "--" {
		t.Fatalf("expected -- immediately before the prompt: %#v", args)
	}
}

func argsContain(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func argsContainPair(args []string, flag string, value string) bool {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == flag && args[index+1] == value {
			return true
		}
	}
	return false
}
