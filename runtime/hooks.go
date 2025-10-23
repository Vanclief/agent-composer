package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"syscall"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/ez"
)

type HookInput struct {
	ID            string         `json:"id"`
	RunID         string         `json:"run_id"`
	EventType     hook.EventType `json:"event_type"`
	TemplateName  string         `json:"template_name"`
	LastResponse  string         `json:"last_response,omitempty"`
	ToolName      string         `json:"tool_name,omitempty"`
	ToolArguments string         `json:"tool_arguments,omitempty"`
}

func RunHook(ctx context.Context, hook hook.Hook, runID uuid.UUID, templateName, lastResponse, toolName, toolArguments string) (HookResult, error) {
	const op = "runtime.RunHook"

	e := HookInput{
		ID:            hook.ID.String(),
		RunID:         runID.String(),
		TemplateName:  templateName,
		EventType:     hook.EventType,
		LastResponse:  lastResponse,
		ToolName:      toolName,
		ToolArguments: toolArguments,
	}

	payload, _ := json.Marshal(e)

	result, err := executeHook(ctx, hook.Command, hook.Args, payload)
	if err != nil {
		log.Error().Err(err).
			Str("EventType", string(hook.EventType)).
			Str("stdout", string(result.Stderr)).
			Msg("Hook execution failed")
		return result, ez.Wrap("RunHook", err)
	}

	log.Info().
		Str("EventType", string(hook.EventType)).
		Str("stdout", string(result.Stdout)).
		Msg("Hook executed")

	return result, nil
}

type HookResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

// executeHook runs an external command with args, piping stdin to the process.
// It respects ctx (cancel/timeout). Non-zero exit codes are returned in result.ExitCode and err.
func executeHook(ctx context.Context, command string, args []string, stdin []byte) (HookResult, error) {
	const op = "runtime.executeHook"

	if command == "" {
		return HookResult{}, ez.New(op, ez.EINVALID, "empty command", nil)
	}

	cmd := exec.CommandContext(ctx, command, args...)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	err := cmd.Run()

	result := HookResult{
		ExitCode: 0,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
	}

	if err == nil {
		return result, nil
	}

	// Derive exit code when possible
	var exitErr *exec.ExitError
	exitCode := -1
	found := errors.As(err, &exitErr)
	if found {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			exitCode = status.ExitStatus()
		}
	}
	result.ExitCode = exitCode

	// Distinguish ctx errors vs process errors
	if ctx.Err() != nil {
		return result, ez.New(op, ez.EUNAVAILABLE, "hook canceled or timed out", ctx.Err())
	}

	return result, ez.New(op, ez.EINTERNAL, "hook process failed", err)
}

func loadInstanceHooks(ctx context.Context, db bun.IDB, templateName string) (map[hook.EventType][]hook.Hook, error) {
	const op = "runtime.loadInstanceHooks"

	var hooks []hook.Hook
	err := db.NewSelect().
		Model(&hooks).
		Where("template_name = ?", templateName).
		Where("enabled = ?", true).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	hookMap := make(map[hook.EventType][]hook.Hook)

	for i := range hooks {
		if hooks[i].Args == nil {
			hooks[i].Args = make([]string, 0)
		}
		hookMap[hooks[i].EventType] = append(hookMap[hooks[i].EventType], hooks[i])
	}

	return hookMap, nil
}
