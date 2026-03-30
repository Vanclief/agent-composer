package runtime

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models/agent"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

func (rt *Runtime) RunConversationInstance(ci *ConversationInstance, prompt string) error {
	const op = "runtime.RunConversationInstance"

	sessionID := "agent:" + ci.ID.String()

	rt.scheduler.RunOnce(rt.rootCtx, sessionID, func(jobCtx context.Context) {
		err := rt.runConversationInstance(jobCtx, ci, prompt)
		if err != nil {
			log.Error().Err(err).Str("conversation_id", ci.ID.String()).Msg("conversation failed")
		}
	})

	return nil
}

func (rt *Runtime) runConversationInstance(ctx context.Context, ci *ConversationInstance, prompt string) error {
	const op = "runtime.runConversationInstance"

	ci.AddMessage(types.MessageRoleUser, prompt)
	ci.Status = agent.ConversationStatusRunning
	ci.HarnessError = ""
	ci.HarnessExitCode = 0
	ci.RawHarnessOutput = ""

	err := ci.Update(ctx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	result, runErr := ci.harness.Run(ctx, ci.Conversation, prompt)
	if result != nil {
		if strings.TrimSpace(result.LastAssistantMessage) != "" {
			ci.AddMessage(types.MessageRoleAssistant, result.LastAssistantMessage)
		}
		if strings.TrimSpace(result.SessionRef) != "" {
			ci.HarnessSessionRef = result.SessionRef
		}
		ci.HarnessState = result.State
		ci.RawHarnessOutput = result.RawOutput
		ci.HarnessExitCode = result.ExitCode
		ci.HarnessError = strings.TrimSpace(result.HarnessError)
		ci.InputTokens += result.InputTokens
		ci.OutputTokens += result.OutputTokens
		ci.CachedTokens += result.CachedTokens
	}

	if runErr != nil {
		if strings.Contains(runErr.Error(), "context canceled") || strings.Contains(runErr.Error(), "canceled") {
			ci.Status = agent.ConversationStatusCanceled
		} else {
			ci.Status = agent.ConversationStatusFailed
		}

		if ci.HarnessError == "" {
			ci.HarnessError = runErr.Error()
		}
	} else {
		ci.Status = agent.ConversationStatusSucceeded
	}

	pCtx := context.WithoutCancel(ctx)
	err = ci.Update(pCtx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	if runErr != nil {
		return ez.Wrap(op, runErr)
	}

	log.Info().Str("conversation_id", ci.ID.String()).Msg("Finished running inference")

	return nil
}
