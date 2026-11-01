package harnesses

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type RunResult struct {
	LastAssistantMessage string
	SessionRef           string
	State                json.RawMessage
	RawOutput            string
	ExitCode             int
	HarnessError         string
	InputTokens          int64
	OutputTokens         int64
	CachedTokens         int64
}

type Harness interface {
	Validate(ctx context.Context, model string, config json.RawMessage) error
	Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*RunResult, error)
}

// New returns the harness for kind, running its programs through exec.
func New(kind agent.Harness, exec executor.Executor) (Harness, error) {
	switch kind {
	case agent.HarnessCodexCLI:
		return &CodexCLI{exec: exec}, nil
	case agent.HarnessClaudeCode:
		return &ClaudeCode{exec: exec}, nil
	case agent.HarnessPi:
		return &PiCLI{exec: exec}, nil
	default:
		return nil, ez.New(ez.EINVALID, "unsupported harness: "+string(kind), nil)
	}
}

// runCommand runs cmd through exec and returns a non-nil error when the
// program could not run, was canceled, or exited non-zero. A non-zero exit
// reads "exit status N", as os/exec reported it, because harness errors fall
// back to that text. The result is never nil.
func runCommand(ctx context.Context, exec executor.Executor, cmd executor.Command) (*executor.Result, error) {
	result, err := exec.Run(ctx, cmd)
	if result == nil {
		result = &executor.Result{}
	}

	if err != nil {
		return result, err
	}

	if result.ExitCode != 0 {
		return result, errors.New("exit status " + strconv.Itoa(result.ExitCode))
	}

	return result, nil
}
