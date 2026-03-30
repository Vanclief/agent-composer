package harnesses

import (
	"context"
	"encoding/json"

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

func New(kind agent.Harness) (Harness, error) {
	switch kind {
	case agent.HarnessCodexCLI:
		return &CodexCLI{}, nil
	case agent.HarnessClaudeCode:
		return &ClaudeCode{}, nil
	default:
		return nil, ez.New("harnesses.New", ez.EINVALID, "unsupported harness: "+string(kind), nil)
	}
}
