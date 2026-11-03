package cli

import (
	"context"
	"reflect"
	"strings"
	"testing"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"
)

// runAgentFlags parses args with agc agent run's own flags and returns the
// harness they name.
func runAgentFlags(t *testing.T, args ...string) (agentHarness, error) {
	t.Helper()

	command := agentCommand()
	var harness agentHarness
	var harnessErr error
	command.Subcommands[0].Action = func(c *cli.Context) error {
		harness, harnessErr = agentHarnessFor(c)
		return nil
	}

	app := &cli.App{Name: "agc", Commands: []*cli.Command{command}}
	err := app.RunContext(context.Background(), append([]string{"agc", "agent", "run"}, args...))
	if err != nil {
		t.Fatal(err)
	}

	return harness, harnessErr
}

func TestAgentRunEffortReachesClaudeCode(t *testing.T) {
	harness, err := runAgentFlags(t, "--effort", "high", "--harness", "claude_code", "--model", "claude-opus-5-5")
	if err != nil {
		t.Fatal(err)
	}

	claude, ok := harness.(claudeAgent)
	if !ok {
		t.Fatalf("harness = %T, want claudeAgent", harness)
	}

	got := claude.args("hello", "s1")
	want := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions", "--model", "claude-opus-5-5", "--effort", "high", "--resume", "s1", "--", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("claude args = %q, want %q", got, want)
	}

	// No --effort keeps the harness's default.
	harness, err = runAgentFlags(t, "--harness", "claude_code")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range harness.(claudeAgent).args("hello", "") {
		if arg == "--effort" {
			t.Fatal("claude args carry --effort with none given")
		}
	}

	harness, err = runAgentFlags(t, "--effort", "xhigh", "--harness", "codex_cli")
	if err != nil {
		t.Fatal(err)
	}
	if harness.(codexAgent).effort != "xhigh" {
		t.Fatalf("codex effort = %q, want xhigh", harness.(codexAgent).effort)
	}

	_, err = runAgentFlags(t, "--effort", "huge", "--harness", "claude_code")
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "low, medium, high, xhigh") {
		t.Fatalf("invalid effort: %v, want an error naming the allowed values", err)
	}
}
