package harnesses

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

// fakeExecutor records what a harness asks it to run and answers with a
// canned result. onRun, when set, sees the files as they are during the run.
type fakeExecutor struct {
	result      executor.Result
	lookPathErr error
	onRun       func(cmd executor.Command)
	commands    []executor.Command
	lookups     []string
	files       map[string][]byte
}

func newFakeExecutor(result executor.Result) *fakeExecutor {
	return &fakeExecutor{result: result, files: map[string][]byte{}}
}

func (f *fakeExecutor) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	f.commands = append(f.commands, cmd)
	if f.onRun != nil {
		f.onRun(cmd)
	}

	result := f.result
	return &result, nil
}

func (f *fakeExecutor) LookPath(ctx context.Context, name string) error {
	f.lookups = append(f.lookups, name)
	return f.lookPathErr
}

func (f *fakeExecutor) WriteFile(ctx context.Context, path string, data []byte) error {
	f.files[path] = data
	return nil
}

func (f *fakeExecutor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, found := f.files[path]
	if !found {
		return nil, ez.New(ez.ENOTFOUND, "no such file: "+path, nil)
	}

	return data, nil
}

func (f *fakeExecutor) RemoveFile(ctx context.Context, path string) error {
	delete(f.files, path)
	return nil
}

func (f *fakeExecutor) TempDir() string {
	return "/fake-tmp"
}

func argAfter(args []string, flag string) string {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == flag {
			return args[index+1]
		}
	}

	return ""
}

func TestClaudeCodeRunsThroughExecutor(t *testing.T) {
	fake := newFakeExecutor(executor.Result{
		Stdout: []byte("{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"Bonjour\",\"session_id\":\"bbc1f8cc-1911-4435-9d66-a83cbe5d4531\",\"usage\":{\"input_tokens\":101,\"cache_read_input_tokens\":12,\"output_tokens\":7}}\n"),
	})
	harness, err := New(agent.HarnessClaudeCode, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = harness.Validate(context.Background(), "claude-opus-4-8", nil)
	if err != nil {
		t.Fatalf("unexpected validate error: %v", err)
	}

	if len(fake.lookups) != 1 || fake.lookups[0] != "claude" {
		t.Fatalf("expected LookPath(claude), got %#v", fake.lookups)
	}

	conversation := &agent.Conversation{Model: "claude-opus-4-8", ProjectDir: "/repo"}
	result, err := harness.Run(context.Background(), conversation, "prompt")
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if len(fake.commands) != 1 {
		t.Fatalf("expected one command, got %d", len(fake.commands))
	}

	cmd := fake.commands[0]
	if cmd.Name != "claude" || cmd.Dir != "/repo" {
		t.Fatalf("unexpected command: %#v", cmd)
	}

	if !argsContainPair(cmd.Args, "--model", "claude-opus-4-8") {
		t.Fatalf("expected --model in args: %#v", cmd.Args)
	}

	if result.LastAssistantMessage != "Bonjour" || result.SessionRef != "bbc1f8cc-1911-4435-9d66-a83cbe5d4531" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if result.InputTokens != 101 || result.OutputTokens != 7 || result.CachedTokens != 12 {
		t.Fatalf("unexpected tokens: %#v", result)
	}
}

func TestClaudeCodeValidateReportsMissingBinary(t *testing.T) {
	fake := newFakeExecutor(executor.Result{})
	fake.lookPathErr = ez.New(ez.ENOTFOUND, "claude is not installed", nil)
	harness, err := New(agent.HarnessClaudeCode, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = harness.Validate(context.Background(), "claude-opus-4-8", nil)
	if ez.ErrorCode(err) != ez.EINVALID || ez.ErrorMessage(err) != "claude CLI is not installed or not on PATH" {
		t.Fatalf("unexpected validate error: %v", err)
	}
}

func TestCodexCLIRunsThroughExecutor(t *testing.T) {
	fake := newFakeExecutor(executor.Result{
		Stdout: []byte("{\"type\":\"thread.started\",\"thread_id\":\"019d4368-c682-7892-8151-16665ad317e2\"}\n" +
			"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":16069,\"cached_input_tokens\":6528,\"output_tokens\":219}}\n"),
	})

	schemaDuringRun := ""
	fake.onRun = func(cmd executor.Command) {
		schemaDuringRun = string(fake.files[argAfter(cmd.Args, "--output-schema")])
		fake.files[argAfter(cmd.Args, "-o")] = []byte("Bonjour\n")
	}

	harness, err := New(agent.HarnessCodexCLI, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conversation := &agent.Conversation{
		Model:                  "gpt-5.5",
		ProjectDir:             "/repo",
		StructuredOutput:       true,
		StructuredOutputSchema: json.RawMessage(`{"type":"object"}`),
	}
	result, err := harness.Run(context.Background(), conversation, "prompt")
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if len(fake.commands) != 1 {
		t.Fatalf("expected one command, got %d", len(fake.commands))
	}

	cmd := fake.commands[0]
	if cmd.Name != "codex" || cmd.Dir != "/repo" {
		t.Fatalf("unexpected command: %#v", cmd)
	}

	if !strings.HasPrefix(argAfter(cmd.Args, "-o"), "/fake-tmp/") || !strings.HasPrefix(argAfter(cmd.Args, "--output-schema"), "/fake-tmp/") {
		t.Fatalf("expected temp files under executor TempDir: %#v", cmd.Args)
	}

	if schemaDuringRun != `{"type":"object"}` {
		t.Fatalf("unexpected schema file content: %q", schemaDuringRun)
	}

	if len(fake.files) != 0 {
		t.Fatalf("expected temp files removed, got %#v", fake.files)
	}

	if result.LastAssistantMessage != "Bonjour" || result.SessionRef != "019d4368-c682-7892-8151-16665ad317e2" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if result.InputTokens != 16069 || result.OutputTokens != 219 || result.CachedTokens != 6528 {
		t.Fatalf("unexpected tokens: %#v", result)
	}
}

func TestCodexCLIReportsNonZeroExit(t *testing.T) {
	fake := newFakeExecutor(executor.Result{Stderr: []byte("boom\n"), ExitCode: 2})
	harness, err := New(agent.HarnessCodexCLI, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := harness.Run(context.Background(), &agent.Conversation{Model: "gpt-5.5"}, "prompt")
	if ez.ErrorCode(err) != ez.EINTERNAL || ez.ErrorMessage(err) != "codex run failed: boom" {
		t.Fatalf("unexpected run error: %v", err)
	}

	if !strings.HasSuffix(err.Error(), "exit status 2") {
		t.Fatalf("expected os/exec style cause, got %q", err.Error())
	}

	if result.ExitCode != 2 || result.HarnessError != "boom" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if fake.commands[0].Dir != "." {
		t.Fatalf("expected default workdir, got %q", fake.commands[0].Dir)
	}
}

func TestPiCLIRunsThroughExecutor(t *testing.T) {
	fake := newFakeExecutor(executor.Result{
		Stdout: []byte("{\"type\":\"message_end\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":\"Bonjour\"}],\"stopReason\":\"stop\",\"usage\":{\"input\":10,\"output\":3,\"cacheRead\":2}}}\n"),
	})
	harness, err := New(agent.HarnessPi, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conversation := &agent.Conversation{Model: "anthropic/claude-opus-4-8", ProjectDir: "/repo", HarnessSessionRef: "session-1"}
	result, err := harness.Run(context.Background(), conversation, "next prompt")
	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if len(fake.commands) != 1 {
		t.Fatalf("expected one command, got %d", len(fake.commands))
	}

	cmd := fake.commands[0]
	if cmd.Name != "pi" || cmd.Dir != "/repo" {
		t.Fatalf("unexpected command: %#v", cmd)
	}

	if !argsContainPair(cmd.Args, "--session-id", "session-1") || cmd.Args[len(cmd.Args)-1] != "next prompt" {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}

	if result.LastAssistantMessage != "Bonjour" || result.SessionRef != "session-1" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if result.InputTokens != 10 || result.OutputTokens != 3 || result.CachedTokens != 2 {
		t.Fatalf("unexpected tokens: %#v", result)
	}
}
