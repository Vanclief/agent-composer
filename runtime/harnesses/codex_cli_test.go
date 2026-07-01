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

func TestBuildArgsForwardsReasoningEffortConfigOverride(t *testing.T) {
	conversation := &agent.Conversation{
		Model:           "gpt-5.5",
		ReasoningEffort: "xhigh",
	}

	args := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{}, "prompt", "/tmp/last-message.txt", "")

	found := false
	for index := 0; index < len(args)-1; index++ {
		if args[index] == "-c" && args[index+1] == `model_reasoning_effort="xhigh"` {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected model_reasoning_effort config override in args: %#v", args)
	}
}

func TestBuildArgsOmitsReasoningEffortWhenUnset(t *testing.T) {
	conversation := &agent.Conversation{
		Model: "gpt-5.5",
	}

	args := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{}, "prompt", "/tmp/last-message.txt", "")

	for index := 0; index < len(args); index++ {
		if args[index] == "-c" {
			t.Fatalf("did not expect a config override when reasoning effort is unset: %#v", args)
		}
	}
}

func TestParseCodexCLIConfigRejectsLegacySandbox(t *testing.T) {
	_, err := parseCodexCLIConfig([]byte(`{"sandbox":"workspace-write"}`))
	if err == nil {
		t.Fatal("expected error for legacy sandbox key")
	}
}

func TestParseCodexCLIConfigDefaultsToReadOnly(t *testing.T) {
	cfg, err := parseCodexCLIConfig([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Permissions != PermissionsReadOnly {
		t.Fatalf("expected read_only default, got %q", cfg.Permissions)
	}
}

func TestCodexBuildArgsMapsPermissionsToSandbox(t *testing.T) {
	conversation := &agent.Conversation{Model: "gpt-5.5"}

	readOnly := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{Permissions: PermissionsReadOnly}, "prompt", "/tmp/last.txt", "")
	if !argsContainPair(readOnly, "--sandbox", "read-only") {
		t.Fatalf("expected --sandbox read-only: %#v", readOnly)
	}

	exec := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{Permissions: PermissionsExec}, "prompt", "/tmp/last.txt", "")
	if !argsContainPair(exec, "--sandbox", "workspace-write") {
		t.Fatalf("expected --sandbox workspace-write: %#v", exec)
	}

	danger := (&CodexCLI{}).buildArgs(conversation, codexCLIConfig{Permissions: PermissionsDangerouslyExec}, "prompt", "/tmp/last.txt", "")
	if !argsContain(danger, "--dangerously-bypass-approvals-and-sandbox") {
		t.Fatalf("expected --dangerously-bypass-approvals-and-sandbox: %#v", danger)
	}
	if argsContain(danger, "--sandbox") {
		t.Fatalf("did not expect --sandbox for dangerously-exec: %#v", danger)
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
