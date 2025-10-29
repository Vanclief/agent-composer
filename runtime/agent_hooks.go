package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	shellmcp "github.com/vanclief/agent-composer/mcp/shell"
	"github.com/vanclief/agent-composer/models/hook"
	types "github.com/vanclief/agent-composer/runtime/types"
)

func (ai *AgentInstance) RunHooks(ctx context.Context, event hook.EventType, toolCall *types.ToolCall, toolCallResponse string) error {
	for _, h := range ai.hooks[event] {
		_, err := ai.useHook(ctx, h, toolCall, toolCallResponse)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) useHook(ctx context.Context, h hook.Hook, toolCall *types.ToolCall, toolCallResponse string) (HookResult, error) {
	out, err := RunHook(ctx, h, ai, toolCall, toolCallResponse)
	if out.ExitCode == 2 {
		stderrText := strings.TrimSpace(string(out.Stderr))
		if stderrText == "" {
			stderrText = "hook failed"
		}

		if toolCall != nil {
			payload := shellmcp.ShellRunResult{
				ExitCode: 1,
				Stderr:   stderrText,
				Command:  toolCall.CommandString(),
			}

			encoded, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				log.Error().Err(marshalErr).Msg("Failed to marshal hook error payload")
				ai.AddToolMessage(toolCall.Name, toolCall.CallID, stderrText)
			} else {
				ai.AddToolMessage(toolCall.Name, toolCall.CallID, string(encoded))
			}
		} else {
			ai.AddMessage(types.MessageRoleUser, stderrText)
		}
		return out, err // Return on first exit code 2
	}

	return out, nil
}
