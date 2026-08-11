package harnesses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"syscall"

	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type ClaudeCode struct{}

type claudeCodeConfig struct {
	Permissions        Permissions `json:"permissions,omitempty"`
	AllowedTools       []string    `json:"allowed_tools,omitempty"`
	DisallowedTools    []string    `json:"disallowed_tools,omitempty"`
	AddDirs            []string    `json:"add_dirs,omitempty"`
	MCPConfig          []string    `json:"mcp_config,omitempty"`
	Tools              []string    `json:"tools,omitempty"`
	Settings           string      `json:"settings,omitempty"`
	SystemPrompt       string      `json:"system_prompt,omitempty"`
	AppendSystemPrompt string      `json:"append_system_prompt,omitempty"`
	Bare               bool        `json:"bare,omitempty"`
	Verbose            bool        `json:"verbose,omitempty"`
	StrictMCPConfig    bool        `json:"strict_mcp_config,omitempty"`
}

// claudeLegacyPermissionKeys are the pre-`permissions` raw fields that are now
// rejected with a migration error.
var claudeLegacyPermissionKeys = []string{
	"permission_mode",
	"dangerously_skip_permissions",
	"allow_dangerously_skip_permissions",
}

type claudeCodeResult struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	SessionID    string  `json:"session_id"`
	StopReason   string  `json:"stop_reason"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
	} `json:"usage"`
}

func (c *ClaudeCode) Validate(ctx context.Context, model string, config json.RawMessage) error {
	if strings.TrimSpace(model) == "" {
		return ez.New(ez.EINVALID, "model is required", nil)
	}

	_, err := exec.LookPath("claude")
	if err != nil {
		return ez.New(ez.EINVALID, "claude CLI is not installed or not on PATH", err)
	}

	_, err = parseClaudeCodeConfig(config)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (c *ClaudeCode) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*RunResult, error) {
	cfg, err := parseClaudeCodeConfig(conversation.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	args := c.buildArgs(conversation, cfg, prompt)

	workdir := strings.TrimSpace(conversation.ShellRoot)
	if workdir == "" {
		workdir = "."
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = workdir

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		found := errors.As(runErr, &exitErr)
		if found {
			status, ok := exitErr.Sys().(syscall.WaitStatus)
			if ok {
				exitCode = status.ExitStatus()
			}
		}
	}

	rawOutput := strings.TrimSpace(stdout.String())
	summary, parseErr := parseClaudeCodeResult(rawOutput)

	harnessError := strings.TrimSpace(stderr.String())
	if summary.IsError {
		if strings.TrimSpace(summary.ErrorMessage()) != "" {
			harnessError = strings.TrimSpace(summary.ErrorMessage())
		}
	}

	result := &RunResult{
		SessionRef:   summary.SessionID,
		RawOutput:    rawOutput,
		ExitCode:     exitCode,
		HarnessError: harnessError,
		InputTokens:  summary.Usage.InputTokens,
		OutputTokens: summary.Usage.OutputTokens,
		CachedTokens: summary.Usage.CacheReadInputTokens,
	}

	if !summary.IsError {
		result.LastAssistantMessage = summary.Result
	}

	if parseErr != nil {
		if runErr != nil {
			if ctx.Err() != nil {
				return result, ez.New(ez.EUNAVAILABLE, "claude run canceled", ctx.Err())
			}

			return result, ez.New(ez.EINTERNAL, "claude run failed", runErr)
		}

		return result, ez.New(ez.EINTERNAL, "failed to parse claude output", parseErr)
	}

	if runErr != nil {
		if ctx.Err() != nil {
			return result, ez.New(ez.EUNAVAILABLE, "claude run canceled", ctx.Err())
		}

		return result, ez.New(ez.EINTERNAL, "claude run failed", runErr)
	}

	if summary.IsError {
		return result, ez.New(ez.EINTERNAL, "claude run failed", nil)
	}

	return result, nil
}

func (c *ClaudeCode) buildArgs(conversation *agent.Conversation, cfg claudeCodeConfig, prompt string) []string {
	args := []string{
		"-p",
		"--output-format",
		"json",
	}

	sessionRef := strings.TrimSpace(conversation.HarnessSessionRef)
	if sessionRef == "" {
		args = append(args, "--model", conversation.Model)

		effort := normalizeClaudeEffort(conversation.ReasoningEffort)
		if effort != "" {
			args = append(args, "--effort", effort)
		}

		perms := resolveClaudePermissions(cfg.Permissions)

		if perms.PermissionMode != "" {
			args = append(args, "--permission-mode", perms.PermissionMode)
		}

		if perms.DangerouslySkip {
			args = append(args, "--dangerously-skip-permissions")
		}

		if cfg.Bare {
			args = append(args, "--bare")
		}

		if cfg.Verbose {
			args = append(args, "--verbose")
		}

		if cfg.StrictMCPConfig {
			args = append(args, "--strict-mcp-config")
		}

		if strings.TrimSpace(cfg.Settings) != "" {
			args = append(args, "--settings", cfg.Settings)
		}

		if strings.TrimSpace(cfg.SystemPrompt) != "" {
			args = append(args, "--system-prompt", cfg.SystemPrompt)
		}

		if strings.TrimSpace(cfg.AppendSystemPrompt) != "" {
			args = append(args, "--append-system-prompt", cfg.AppendSystemPrompt)
		}

		for _, dir := range cfg.AddDirs {
			trimmed := strings.TrimSpace(dir)
			if trimmed == "" {
				continue
			}

			args = append(args, "--add-dir", trimmed)
		}

		for _, tool := range cfg.AllowedTools {
			trimmed := strings.TrimSpace(tool)
			if trimmed == "" {
				continue
			}

			args = append(args, "--allowedTools", trimmed)
		}

		disallowed := append([]string{}, perms.DisallowedTools...)
		disallowed = append(disallowed, cfg.DisallowedTools...)
		for _, tool := range disallowed {
			trimmed := strings.TrimSpace(tool)
			if trimmed == "" {
				continue
			}

			args = append(args, "--disallowedTools", trimmed)
		}

		for _, config := range cfg.MCPConfig {
			trimmed := strings.TrimSpace(config)
			if trimmed == "" {
				continue
			}

			args = append(args, "--mcp-config", trimmed)
		}

		for _, tool := range cfg.Tools {
			trimmed := strings.TrimSpace(tool)
			if trimmed == "" {
				continue
			}

			args = append(args, "--tools", trimmed)
		}
	} else {
		args = append(args, "--resume", sessionRef)
	}

	// Terminate option parsing before the positional prompt. Variadic flags such
	// as --disallowedTools/--allowedTools/--tools would otherwise greedily consume
	// the prompt as additional tool values.
	args = append(args, "--")

	if sessionRef == "" {
		args = append(args, buildInitialPrompt(conversation))
	} else {
		args = append(args, prompt)
	}

	return args
}

func parseClaudeCodeConfig(raw json.RawMessage) (claudeCodeConfig, error) {
	if len(raw) == 0 {
		return claudeCodeConfig{}, nil
	}

	var probe map[string]any
	err := json.Unmarshal(raw, &probe)
	if err != nil {
		return claudeCodeConfig{}, ez.New(ez.EINVALID, "invalid claude harness_config", err)
	}

	err = rejectLegacyPermissionKeys(probe, claudeLegacyPermissionKeys)
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	var cfg claudeCodeConfig
	err = json.Unmarshal(raw, &cfg)
	if err != nil {
		return claudeCodeConfig{}, ez.New(ez.EINVALID, "invalid claude harness_config", err)
	}

	normalizedPermissions, err := ParsePermissions(string(cfg.Permissions))
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}
	cfg.Permissions = normalizedPermissions

	if strings.TrimSpace(cfg.Settings) == "" && cfg.Settings != "" {
		return claudeCodeConfig{}, ez.New(ez.EINVALID, "settings cannot be empty", nil)
	}

	err = validateStringList(cfg.AddDirs, "add_dirs")
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	err = validateStringList(cfg.AllowedTools, "allowed_tools")
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	err = validateStringList(cfg.DisallowedTools, "disallowed_tools")
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	err = validateStringList(cfg.MCPConfig, "mcp_config")
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	err = validateStringList(cfg.Tools, "tools")
	if err != nil {
		return claudeCodeConfig{}, ez.Wrap(err)
	}

	return cfg, nil
}

func validateStringList(values []string, field string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return ez.New(ez.EINVALID, field+" cannot contain empty values", nil)
		}
	}

	return nil
}

func parseClaudeCodeResult(rawOutput string) (claudeCodeResult, error) {
	trimmed := strings.TrimSpace(rawOutput)
	if trimmed == "" {
		return claudeCodeResult{}, ez.New(ez.EINVALID, "empty claude output", nil)
	}

	var result claudeCodeResult
	err := json.Unmarshal([]byte(trimmed), &result)
	if err != nil {
		return claudeCodeResult{}, ez.New(ez.EINVALID, "invalid claude output", err)
	}

	if strings.TrimSpace(result.Type) == "" {
		return claudeCodeResult{}, ez.New(ez.EINVALID, "claude output is missing type", nil)
	}

	return result, nil
}

func (r claudeCodeResult) ErrorMessage() string {
	if !r.IsError {
		return ""
	}

	message := strings.TrimSpace(r.Result)
	if message != "" {
		return message
	}

	if strings.TrimSpace(r.Subtype) != "" {
		return "claude error: " + strings.TrimSpace(r.Subtype)
	}

	return "claude reported an error"
}

func normalizeClaudeEffort(effort runtimetypes.ReasoningEffort) string {
	text := strings.TrimSpace(string(effort))
	if text == "" {
		return ""
	}

	return text
}
