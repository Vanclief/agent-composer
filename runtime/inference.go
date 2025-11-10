package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models/agent"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type toolCallKey struct{ name, args string }

func (rt *Runtime) RunConversationInstance(ci *ConversationInstance, prompt string) error {
	const op = "runtime.RunConversationInstance"

	sessionID := fmt.Sprintf("agent:%s", ci.ID)

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

	// Step 1: Append the user prompt to the messages and update the status
	ci.AddMessage(types.MessageRoleUser, prompt)
	ci.Status = agent.ConversationStatusRunning

	err := ci.Update(ctx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	// Step 2: Run any session started hooks
	ci.RunConversationStartedHook(ctx)

	// Step 3: Run the inference
	inferenceErr := rt.runInference(ctx, ci)
	if inferenceErr != nil {
		if strings.Contains(inferenceErr.Error(), "context canceled") {
			ci.Status = agent.ConversationStatusCanceled
		} else {
			ci.Status = agent.ConversationStatusFailed
		}
	}

	pCtx := context.WithoutCancel(ctx)
	err = ci.Update(pCtx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	if inferenceErr != nil {
		return ez.Wrap(op, inferenceErr)
	}

	// Step 4: If there was no error, return the last message content
	log.Info().Str("conversation_id", ci.ID.String()).Msg("Finished running inference")

	return nil
}

func (rt *Runtime) runInference(ctx context.Context, ci *ConversationInstance) error {
	const op = "runtime.ConversationInstance.runInference"

	const maxSteps = 300

	toolCalls := map[toolCallKey]int{}
	var prevResponseID string // This is for OpenAI

	for step := 0; step < maxSteps; step++ {

		inputTokens, err := ci.provider.EstimateInputTokens(ci.Model, ci.Messages)
		if err != nil {
			return ez.Wrap(op, err)
		}

		compactAtPercent := 100
		if ci.AutoCompact {
			compactAtPercent = ci.CompactAtPercent
		}

		err = ci.provider.CheckContextWindow(ci.Model, inputTokens, compactAtPercent)
		if err != nil {
			// If we exceed context, run any hooks and compact if autoCompact is set
			if ci.AutoCompact {

				err = ci.RunPreContextCompactionHook(ctx, uuid.Nil)
				if err != nil {
					return ez.Wrap(op, err)
				}

				ci.AddMessage(types.MessageRoleUser, ci.CompactionPrompt)

				chatRequest := types.ChatRequest{
					Messages:           ci.Messages,
					PreviousResponseID: prevResponseID,
					ThinkingEffort:     string(ci.ReasoningEffort),
				}

				compactingResponse, err := ci.provider.Chat(ctx, ci.Model, &chatRequest)
				if err != nil {
					return ez.Wrap(op, err)
				}

				newInputTokens := compactingResponse.TokenUsage.InputTokens - compactingResponse.TokenUsage.CacheReadInputTokens
				if newInputTokens < 0 {
					newInputTokens = 0
				}
				ci.InputTokens += newInputTokens
				ci.OutputTokens += compactingResponse.TokenUsage.OutputTokens
				ci.CachedTokens += compactingResponse.TokenUsage.CacheReadInputTokens

				newConversation, err := ci.Clone(ctx, rt.db, true)
				if err != nil {
					return ez.Wrap(op, err)
				}

				newConversation.CompactCount = ci.CompactCount + 1

				newInstance, err := rt.NewConversationInstance(ctx, newConversation.ID)
				if err != nil {
					return ez.Wrap(op, err)
				}

				rt.RunConversationInstance(newInstance, compactingResponse.Text)

				ci.RunPostContextCompactionHook(ctx, newConversation.ID)

				return ez.New(op, ez.EINVALID, "Context window exceeded, compacted in new conversation", nil)
			}

			return ez.Wrap(op, err)
		}

		log.Info().Int("input_tokens", inputTokens).Msg("Estimated input tokens")

		// Step 2: Make the LLM call
		chatRequest := types.ChatRequest{
			Messages:               ci.Messages,
			Tools:                  ci.Tools,
			PreviousResponseID:     prevResponseID,
			ThinkingEffort:         string(ci.ReasoningEffort),
			WebSearch:              ci.WebSearch,
			StructuredOutputs:      ci.StructuredOutput,
			StructuredOutputSchema: ci.StructuredOutputSchema,
		}

		response, err := ci.provider.Chat(ctx, ci.Model, &chatRequest)
		if err != nil {
			return ez.Wrap(op, err)
		}

		prevResponseID = response.ID // NOTE: This only applies to OpenAI

		newInputTokens := response.TokenUsage.InputTokens - response.TokenUsage.CacheReadInputTokens
		if newInputTokens < 0 {
			newInputTokens = 0
		}
		ci.InputTokens += newInputTokens
		ci.OutputTokens += response.TokenUsage.OutputTokens
		ci.CachedTokens += response.TokenUsage.CacheReadInputTokens

		// Step 3: If we do have tool calls, execute them
		for _, toolCall := range response.ToolCalls {

			log.Info().
				Str("Name", ci.AgentName).
				Str("ID", ci.ID.String()).
				Str("tool", toolCall.Name).
				Str("args", toolCall.Arguments).
				Int("step", step).
				Msg("Agent made tool call")

			// 3.1 Create a tool call key
			callKey := toolCallKey{name: toolCall.Name, args: toolCall.Arguments}

			// 3.2 Check that we are not in an infinite loop of tool calls with identical arguments
			lastStepCall, found := toolCalls[callKey]

			if found && step-lastStepCall <= 1 {
				toolCalls[callKey] = step

				log.Warn().Str("tool", toolCall.Name).Str("args", toolCall.Arguments).Msg("Skipping tool call due to anti-loop policy")

				// IMPORTANT: Always satisfy the protocol with a ToolMessage for this call_id.
				// We send a synthetic error payload instead of executing the tool again.
				syntheticError := `{"error":"duplicate_tool_call","policy":"anti-loop","message":"Duplicate tool call with identical arguments within one step; tool execution skipped."}`
				ci.AddToolMessage(toolCall.Name, toolCall.CallID, syntheticError)

				continue
			}

			// Persist the assistant-issued tool call so resumes have the full transcript.
			ci.AddAssistantToolCall(toolCall)

			// 3.3 Run any pre-tool-use hooks
			err = ci.RunPreToolUseHook(ctx, &toolCall, "")
			if err != nil {
				// Record the step to help the anti-loop policy.
				toolCalls[callKey] = step

				continue
			}

			// 3.4 Call the tool
			toolCallResponse, err := ci.mcpMux.CallTool(ctx, &toolCall)
			if err != nil {
				return ez.Wrap("agent.ExecuteTool", err)
			}

			log.Info().
				Str("Name", ci.AgentName).
				Str("ID", ci.ID.String()).
				Str("tool", toolCall.Name).
				Str("args", toolCall.Arguments).
				Str("tool_response", toolCallResponse).
				Int("step", step).
				Msg("Tool Call Response")

			// 3.5 Run any post-tool-use hooks
			err = ci.RunPostToolUseHook(ctx, &toolCall, toolCallResponse)
			if err != nil {
				// Record the step to help the anti-loop policy.
				toolCalls[callKey] = step

				continue
			}

			// 3.6 Record the tool call step
			toolCalls[callKey] = step

			ci.AddToolMessage(toolCall.Name, toolCall.CallID, toolCallResponse)
		}

		// Step 4: If we don't have any tool calls
		if len(response.ToolCalls) == 0 {

			log.Info().
				Str("Name", ci.AgentName).
				Str("ID", ci.ID.String()).
				Str("Response", response.Text).
				Int("step", step).
				Msg("Agent response")

			ci.AddMessage(types.MessageRoleAssistant, response.Text)

			// 4.1 Update the conversation status to succeeded so that hook don't see it as running
			ci.Status = agent.ConversationStatusSucceeded
			err = ci.Update(ctx, rt.db)
			if err != nil {
				return ez.Wrap(op, err)
			}

			// 4.2 Check if any hooks want to block the stop
			blockStop := false

			err = ci.RunConversationEndedHook(ctx)
			if err != nil {

				blockStop = true

				// Since a hook has requested more work, we set the conversation back to running
				ci.Status = agent.ConversationStatusRunning
				err = ci.Update(ctx, rt.db)
				if err != nil {
					return ez.Wrap(op, err)
				}
			}

			if !blockStop {

				ci.Cost = ci.provider.CalculateCost(ci.Model, ci.InputTokens, ci.OutputTokens, ci.CachedTokens)

				log.Info().
					Str("Name", ci.AgentName).
					Str("ID", ci.ID.String()).
					Int("step", step).
					Msg("Agent finished")
				return nil
			}
		}

		err = ci.Update(ctx, rt.db)
		if err != nil {
			return ez.Wrap(op, err)
		}
	}

	return ez.Root(op, ez.ERESOURCEEXHAUSTED, "exceeded maximum inference steps")
}
