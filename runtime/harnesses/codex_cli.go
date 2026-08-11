package harnesses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/vanclief/agent-composer/core/helpers/jsonutil"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type CodexCLI struct{}

type codexCLIConfig struct {
	Profile          string      `json:"profile,omitempty"`
	Permissions      Permissions `json:"permissions,omitempty"`
	SkipGitRepoCheck bool        `json:"skip_git_repo_check,omitempty"`
	Ephemeral        bool        `json:"ephemeral,omitempty"`
	AddDirs          []string    `json:"add_dirs,omitempty"`
	ConfigOverrides  []string    `json:"config_overrides,omitempty"`
}

// codexLegacyPermissionKeys are the pre-`permissions` raw fields that are now
// rejected with a migration error.
var codexLegacyPermissionKeys = []string{
	"sandbox",
	"full_auto",
	"dangerously_bypass_approvals_and_sandbox",
}

type codexRunSummary struct {
	SessionRef   string
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
	Errors       []string
}

func (c *CodexCLI) Validate(ctx context.Context, model string, config json.RawMessage) error {
	if strings.TrimSpace(model) == "" {
		return ez.New(ez.EINVALID, "model is required", nil)
	}

	_, err := exec.LookPath("codex")
	if err != nil {
		return ez.New(ez.EINVALID, "codex CLI is not installed or not on PATH", err)
	}

	_, err = parseCodexCLIConfig(config)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (c *CodexCLI) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*RunResult, error) {
	cfg, err := parseCodexCLIConfig(conversation.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	lastMessageFile, err := os.CreateTemp("", "agent-composer-codex-last-message-*.txt")
	if err != nil {
		return nil, ez.Wrap(err)
	}
	lastMessagePath := lastMessageFile.Name()

	err = lastMessageFile.Close()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	defer os.Remove(lastMessagePath)

	schemaPath := ""
	if hasStructuredOutputSchema(conversation.StructuredOutputSchema) {
		schemaFile, err := os.CreateTemp("", "agent-composer-codex-output-schema-*.json")
		if err != nil {
			return nil, ez.Wrap(err)
		}
		schemaPath = schemaFile.Name()

		_, err = schemaFile.Write(conversation.StructuredOutputSchema)
		if err != nil {
			schemaFile.Close()
			return nil, ez.Wrap(err)
		}

		err = schemaFile.Close()
		if err != nil {
			return nil, ez.Wrap(err)
		}

		defer os.Remove(schemaPath)
	}

	workdir := strings.TrimSpace(conversation.ShellRoot)
	if workdir == "" {
		workdir = "."
	}

	args := c.buildArgs(conversation, cfg, prompt, lastMessagePath, schemaPath)

	cmd := exec.CommandContext(ctx, "codex", args...)
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

	lastAssistantMessage := ""
	content, readErr := os.ReadFile(lastMessagePath)
	if readErr == nil {
		lastAssistantMessage = strings.TrimSpace(string(content))
	}

	rawOutput := stdout.String()
	summary := parseCodexRunSummary(rawOutput)

	harnessError := selectCodexHarnessError(strings.TrimSpace(stderr.String()), summary)

	result := &RunResult{
		LastAssistantMessage: lastAssistantMessage,
		SessionRef:           summary.SessionRef,
		RawOutput:            rawOutput,
		ExitCode:             exitCode,
		HarnessError:         harnessError,
		InputTokens:          summary.InputTokens,
		OutputTokens:         summary.OutputTokens,
		CachedTokens:         summary.CachedTokens,
	}

	if runErr != nil {
		if ctx.Err() != nil {
			return result, ez.New(ez.EUNAVAILABLE, "codex run canceled", ctx.Err())
		}

		message := "codex run failed"
		if strings.TrimSpace(harnessError) != "" {
			message = message + ": " + strings.TrimSpace(harnessError)
		}

		return result, ez.New(ez.EINTERNAL, message, runErr)
	}

	return result, nil
}

func selectCodexHarnessError(stderrText string, summary codexRunSummary) string {
	if len(summary.Errors) > 0 {
		normalized := make([]string, 0, len(summary.Errors))
		for _, rawError := range summary.Errors {
			trimmed := strings.TrimSpace(rawError)
			if trimmed == "" {
				continue
			}

			normalized = append(normalized, normalizeCodexHarnessError(trimmed))
		}

		if len(normalized) > 0 {
			return strings.Join(normalized, "\n")
		}
	}

	return strings.TrimSpace(stderrText)
}

func normalizeCodexHarnessError(raw string) string {
	var payload map[string]any
	err := json.Unmarshal([]byte(raw), &payload)
	if err != nil {
		return raw
	}

	message := findFirstString(payload, "message")
	if strings.TrimSpace(message) == "" {
		return raw
	}

	return strings.TrimSpace(message)
}

func (c *CodexCLI) buildArgs(conversation *agent.Conversation, cfg codexCLIConfig, prompt string, lastMessagePath string, schemaPath string) []string {
	args := []string{"exec"}

	if strings.TrimSpace(conversation.HarnessSessionRef) != "" {
		args = append(args, "resume", conversation.HarnessSessionRef)
	} else {
		if cfg.Profile != "" {
			args = append(args, "--profile", cfg.Profile)
		}

		perms := resolveCodexPermissions(cfg.Permissions)
		if perms.Sandbox != "" {
			args = append(args, "--sandbox", perms.Sandbox)
		}
		if perms.DangerousBypass {
			args = append(args, "--dangerously-bypass-approvals-and-sandbox")
		}

		for _, dir := range cfg.AddDirs {
			trimmed := strings.TrimSpace(dir)
			if trimmed == "" {
				continue
			}
			args = append(args, "--add-dir", trimmed)
		}
	}

	args = append(args, "--json")
	args = append(args, "-m", conversation.Model)
	effort := strings.TrimSpace(string(conversation.ReasoningEffort))
	if effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(effort))
	}
	args = append(args, "-o", lastMessagePath)
	if strings.TrimSpace(schemaPath) != "" {
		args = append(args, "--output-schema", schemaPath)
	}

	if cfg.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}

	if cfg.Ephemeral {
		args = append(args, "--ephemeral")
	}

	for _, override := range cfg.ConfigOverrides {
		trimmed := strings.TrimSpace(override)
		if trimmed == "" {
			continue
		}
		args = append(args, "-c", trimmed)
	}

	if strings.TrimSpace(conversation.HarnessSessionRef) == "" {
		args = append(args, buildInitialPrompt(conversation))
	} else {
		args = append(args, prompt)
	}

	return args
}

func hasStructuredOutputSchema(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || jsonutil.IsNullRawMessage(trimmed) {
		return false
	}

	return json.Valid(trimmed)
}

func parseCodexCLIConfig(raw json.RawMessage) (codexCLIConfig, error) {
	if len(raw) == 0 {
		return codexCLIConfig{}, nil
	}

	var probe map[string]any
	err := json.Unmarshal(raw, &probe)
	if err != nil {
		return codexCLIConfig{}, ez.New(ez.EINVALID, "invalid codex harness_config", err)
	}

	err = rejectLegacyPermissionKeys(probe, codexLegacyPermissionKeys)
	if err != nil {
		return codexCLIConfig{}, ez.Wrap(err)
	}

	var cfg codexCLIConfig
	err = json.Unmarshal(raw, &cfg)
	if err != nil {
		return codexCLIConfig{}, ez.New(ez.EINVALID, "invalid codex harness_config", err)
	}

	normalizedPermissions, err := ParsePermissions(string(cfg.Permissions))
	if err != nil {
		return codexCLIConfig{}, ez.Wrap(err)
	}
	cfg.Permissions = normalizedPermissions

	for _, override := range cfg.ConfigOverrides {
		if strings.TrimSpace(override) == "" {
			return codexCLIConfig{}, ez.New(ez.EINVALID, "config_overrides cannot contain empty values", nil)
		}
	}

	return cfg, nil
}

func parseCodexRunSummary(rawOutput string) codexRunSummary {
	summary := codexRunSummary{}

	lines := strings.Split(rawOutput, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var payload map[string]any
		err := json.Unmarshal([]byte(trimmed), &payload)
		if err != nil {
			continue
		}

		if summary.SessionRef == "" {
			sessionRef := findFirstString(payload, "thread_id", "session_id")
			if sessionRef != "" {
				summary.SessionRef = sessionRef
			}
		}

		eventType := findFirstString(payload, "type")
		if eventType != "" && eventType == "error" {
			message := findFirstString(payload, "message")
			if message != "" {
				summary.Errors = append(summary.Errors, message)
			}
		}

		inputTokens := findFirstInt64(payload, "input_tokens")
		if inputTokens > 0 {
			summary.InputTokens = inputTokens
		}

		outputTokens := findFirstInt64(payload, "output_tokens")
		if outputTokens > 0 {
			summary.OutputTokens = outputTokens
		}

		cachedTokens := findFirstInt64(payload, "cached_tokens", "cached_input_tokens", "cache_read_input_tokens")
		if cachedTokens > 0 {
			summary.CachedTokens = cachedTokens
		}
	}

	return summary
}

func findFirstString(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			raw, found := typed[key]
			if found {
				text, ok := raw.(string)
				if ok && strings.TrimSpace(text) != "" {
					return text
				}
			}
		}

		for _, child := range typed {
			text := findFirstString(child, keys...)
			if text != "" {
				return text
			}
		}
	case []any:
		for _, child := range typed {
			text := findFirstString(child, keys...)
			if text != "" {
				return text
			}
		}
	}

	return ""
}

func findFirstInt64(value any, keys ...string) int64 {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			raw, found := typed[key]
			if found {
				number := toInt64(raw)
				if number > 0 {
					return number
				}
			}
		}

		for _, child := range typed {
			number := findFirstInt64(child, keys...)
			if number > 0 {
				return number
			}
		}
	case []any:
		for _, child := range typed {
			number := findFirstInt64(child, keys...)
			if number > 0 {
				return number
			}
		}
	}

	return 0
}

func toInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		number, err := typed.Int64()
		if err == nil {
			return number
		}
	case string:
		number, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil {
			return number
		}
	}

	return 0
}
