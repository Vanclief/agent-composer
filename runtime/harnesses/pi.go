package harnesses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

// PiCLI runs conversations through the pi coding agent
// (`pi -p --mode json`), which emits a JSONL event stream.
type PiCLI struct{}

type piConfig struct {
	// Provider forces a provider (pi also accepts "provider/id" models).
	Provider string `json:"provider,omitempty"`
	// ExtraArgs are appended verbatim to the pi invocation.
	ExtraArgs []string `json:"extra_args,omitempty"`
}

func parsePiConfig(raw json.RawMessage) (piConfig, error) {
	if len(raw) == 0 {
		return piConfig{}, nil
	}

	var cfg piConfig
	err := json.Unmarshal(raw, &cfg)
	if err != nil {
		return piConfig{}, ez.New(ez.EINVALID, "invalid pi harness_config", err)
	}
	for _, arg := range cfg.ExtraArgs {
		if strings.TrimSpace(arg) == "" {
			return piConfig{}, ez.New(ez.EINVALID, "extra_args cannot contain empty values", nil)
		}
	}

	return cfg, nil
}

func (c *PiCLI) Validate(ctx context.Context, model string, config json.RawMessage) error {
	if strings.TrimSpace(model) == "" {
		return ez.New(ez.EINVALID, "model is required", nil)
	}
	_, err := parsePiConfig(config)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (c *PiCLI) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*RunResult, error) {
	cfg, err := parsePiConfig(conversation.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	workdir := strings.TrimSpace(conversation.ProjectDir)
	if workdir == "" {
		workdir = "."
	}

	// We choose the session id ourselves: pi creates it if missing and
	// resumes it otherwise, so no output parsing is needed for resume.
	sessionRef := strings.TrimSpace(conversation.HarnessSessionRef)
	newSession := sessionRef == ""
	if newSession {
		sessionRef = uuid.NewString()
	}

	args := []string{"-p", "--mode", "json", "--session-id", sessionRef}
	if newSession {
		args = append(args, "--model", conversation.Model)
		if cfg.Provider != "" {
			args = append(args, "--provider", cfg.Provider)
		}
		effort := strings.TrimSpace(string(conversation.ReasoningEffort))
		if effort != "" {
			args = append(args, "--thinking", effort)
		}
		if strings.TrimSpace(conversation.Instructions) != "" {
			args = append(args, "--system-prompt", conversation.Instructions)
		}
	}
	args = append(args, cfg.ExtraArgs...)

	if newSession {
		args = append(args, buildInitialPrompt(conversation))
	} else {
		args = append(args, prompt)
	}

	cmd := exec.CommandContext(ctx, "pi", args...)
	cmd.Dir = workdir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	rawOutput := stdout.String()
	summary := parsePiRunSummary(rawOutput)

	harnessError := summary.Error
	if harnessError == "" {
		harnessError = strings.TrimSpace(stderr.String())
	}

	result := &RunResult{
		LastAssistantMessage: summary.LastAssistantMessage,
		SessionRef:           sessionRef,
		RawOutput:            rawOutput,
		ExitCode:             exitCode,
		HarnessError:         harnessError,
		InputTokens:          summary.InputTokens,
		OutputTokens:         summary.OutputTokens,
		CachedTokens:         summary.CachedTokens,
	}

	if runErr != nil {
		if ctx.Err() != nil {
			return result, ez.New(ez.EUNAVAILABLE, "pi run canceled", ctx.Err())
		}
		message := "pi run failed"
		if strings.TrimSpace(harnessError) != "" {
			message = message + ": " + strings.TrimSpace(harnessError)
		}
		return result, ez.New(ez.EINTERNAL, message, runErr)
	}

	return result, nil
}

type piRunSummary struct {
	LastAssistantMessage string
	Error                string
	InputTokens          int64
	OutputTokens         int64
	CachedTokens         int64
}

func parsePiRunSummary(rawOutput string) piRunSummary {
	summary := piRunSummary{}

	for _, line := range strings.Split(rawOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var payload map[string]any
		if json.Unmarshal([]byte(trimmed), &payload) != nil {
			continue
		}

		if message := findAssistantMessage(payload); message != nil {
			if text := assistantText(message); text != "" {
				summary.LastAssistantMessage = text
			}
			stopReason := findFirstString(message, "stopReason")
			if stopReason == "error" || stopReason == "aborted" {
				errorMessage := findFirstString(message, "errorMessage")
				if errorMessage == "" {
					errorMessage = "pi request " + stopReason
				}
				summary.Error = errorMessage
			} else if stopReason != "" {
				summary.Error = ""
			}
			if usage, ok := message["usage"].(map[string]any); ok {
				summary.InputTokens = int64(numberValue(usage["input"]))
				summary.OutputTokens = int64(numberValue(usage["output"]))
				summary.CachedTokens = int64(numberValue(usage["cacheRead"]))
			}
		}
	}

	return summary
}

// findAssistantMessage locates an assistant message object anywhere in
// the event payload (events wrap messages differently per type).
func findAssistantMessage(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		if role, _ := typed["role"].(string); role == "assistant" {
			if _, hasContent := typed["content"]; hasContent {
				return typed
			}
		}
		for _, child := range typed {
			if found := findAssistantMessage(child); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findAssistantMessage(child); found != nil {
				return found
			}
		}
	}
	return nil
}

func assistantText(message map[string]any) string {
	content, ok := message["content"].([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range content {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if blockType, _ := block["type"].(string); blockType == "text" {
			if text, _ := block["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func numberValue(value any) float64 {
	number, _ := value.(float64)
	return number
}
