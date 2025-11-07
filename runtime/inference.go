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

func (rt *Runtime) RunAgentInstance(ai *AgentInstance, prompt string) error {
	const op = "runtime.RunConversations"

	sessionID := fmt.Sprintf("agent:%s", ai.ID)

	rt.scheduler.RunOnce(rt.rootCtx, sessionID, func(jobCtx context.Context) {
		err := rt.runAgentInstance(jobCtx, ai, prompt)
		if err != nil {
			log.Error().Err(err).Str("conversation_id", ai.ID.String()).Msg("conversation failed")
		}
	})

	return nil
}

func (rt *Runtime) runAgentInstance(ctx context.Context, ai *AgentInstance, prompt string) error {
	const op = "runtime.AgentInstance.Run"

	// Step 1: Append the user prompt to the messages and update the status
	ai.AddMessage(types.MessageRoleUser, prompt)
	ai.Status = agent.ConversationStatusRunning

	err := ai.Update(ctx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	// Step 2: Run any session started hooks
	ai.RunConversationStartedHook(ctx)

	// Step 3: Run the inference
	inferenceErr := rt.runInference(ctx, ai)
	if inferenceErr != nil {
		if strings.Contains(inferenceErr.Error(), "context canceled") {
			ai.Status = agent.ConversationStatusCanceled
		} else {
			ai.Status = agent.ConversationStatusFailed
		}
	}

	pCtx := context.WithoutCancel(ctx)
	err = ai.Update(pCtx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	if inferenceErr != nil {
		return ez.Wrap(op, inferenceErr)
	}

	// Step 4: If there was no error, return the last message content
	log.Info().Str("conversation_id", ai.ID.String()).Msg("Finished running inference")

	return nil
}

func (rt *Runtime) runInference(ctx context.Context, ai *AgentInstance) error {
	const op = "runtime.AgentInstance.runInference"

	const maxSteps = 300

	toolCalls := map[toolCallKey]int{}
	var prevResponseID string // This is for OpenAI

	for step := 0; step < maxSteps; step++ {

		inputTokens, err := ai.provider.EstimateInputTokens(ai.Model, ai.Messages)
		if err != nil {
			return ez.Wrap(op, err)
		}

		compactAtPercent := 100
		if ai.AutoCompact {
			compactAtPercent = ai.CompactAtPercent
		}

		err = ai.provider.CheckContextWindow(ai.Model, inputTokens, compactAtPercent)
		if err != nil {
			// If we exceed context, run any hooks and compact if autoCompact is set
			if ai.AutoCompact {

				err = ai.RunPreContextCompactionHook(ctx, uuid.Nil)
				if err != nil {
					return ez.Wrap(op, err)
				}

				ai.AddMessage(types.MessageRoleUser, ai.CompactionPrompt)

				chatRequest := types.ChatRequest{
					Messages:           ai.Messages,
					PreviousResponseID: prevResponseID,
					ThinkingEffort:     string(ai.ReasoningEffort),
				}

				compactingResponse, err := ai.provider.Chat(ctx, ai.Model, &chatRequest)
				if err != nil {
					return ez.Wrap(op, err)
				}

				newInputTokens := compactingResponse.TokenUsage.InputTokens - compactingResponse.TokenUsage.CacheReadInputTokens
				if newInputTokens < 0 {
					newInputTokens = 0
				}
				ai.InputTokens += newInputTokens
				ai.OutputTokens += compactingResponse.TokenUsage.OutputTokens
				ai.CachedTokens += compactingResponse.TokenUsage.CacheReadInputTokens

				newAI, err := rt.NewAgentInstanceFromSpec(ctx, ai.AgentSpecID)
				if err != nil {
					return ez.Wrap(op, err)
				}

				newAI.CompactCount = ai.CompactCount + 1

				rt.RunAgentInstance(newAI, compactingResponse.Text)

				ai.RunPostContextCompactionHook(ctx, newAI.ID)

				return ez.New(op, ez.EINVALID, "Context window exceeded, compacted in new conversation", nil)
			}

			return ez.Wrap(op, err)
		}

		log.Info().Int("input_tokens", inputTokens).Msg("Estimated input tokens")

		// Step 2: Make the LLM call
		chatRequest := types.ChatRequest{
			Messages:               ai.Messages,
			Tools:                  ai.Tools,
			PreviousResponseID:     prevResponseID,
			ThinkingEffort:         string(ai.ReasoningEffort),
			WebSearch:              ai.WebSearch,
			StructuredOutputs:      ai.StructuredOutput,
			StructuredOutputSchema: ai.StructuredOutputSchema,
		}

		response, err := ai.provider.Chat(ctx, ai.Model, &chatRequest)
		if err != nil {
			return ez.Wrap(op, err)
		}

		prevResponseID = response.ID // NOTE: This only applies to OpenAI

		newInputTokens := response.TokenUsage.InputTokens - response.TokenUsage.CacheReadInputTokens
		if newInputTokens < 0 {
			newInputTokens = 0
		}
		ai.InputTokens += newInputTokens
		ai.OutputTokens += response.TokenUsage.OutputTokens
		ai.CachedTokens += response.TokenUsage.CacheReadInputTokens

		// Step 3: If we do have tool calls, execute them
		for _, toolCall := range response.ToolCalls {

			log.Info().
				Str("Name", ai.AgentName).
				Str("ID", ai.ID.String()).
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
				ai.AddToolMessage(toolCall.Name, toolCall.CallID, syntheticError)

				continue
			}

			// Persist the assistant-issued tool call so resumes have the full transcript.
			ai.AddAssistantToolCall(toolCall)

			// 3.3 Run any pre-tool-use hooks
			err = ai.RunPreToolUseHook(ctx, &toolCall, "")
			if err != nil {
				// Record the step to help the anti-loop policy.
				toolCalls[callKey] = step

				continue
			}

			// 3.4 Call the tool
			toolCallResponse, err := ai.mcpMux.CallTool(ctx, &toolCall)
			if err != nil {
				return ez.Wrap("agent.ExecuteTool", err)
			}

			log.Info().
				Str("Name", ai.AgentName).
				Str("ID", ai.ID.String()).
				Str("tool", toolCall.Name).
				Str("args", toolCall.Arguments).
				Str("tool_response", toolCallResponse).
				Int("step", step).
				Msg("Tool Call Response")

			// 3.5 Run any post-tool-use hooks
			err = ai.RunPostToolUseHook(ctx, &toolCall, toolCallResponse)
			if err != nil {
				// Record the step to help the anti-loop policy.
				toolCalls[callKey] = step

				continue
			}

			// 3.6 Record the tool call step
			toolCalls[callKey] = step

			ai.AddToolMessage(toolCall.Name, toolCall.CallID, toolCallResponse)
		}

		// Step 4: If we don't have any tool calls
		if len(response.ToolCalls) == 0 {

			log.Info().
				Str("Name", ai.AgentName).
				Str("ID", ai.ID.String()).
				Str("Response", response.Text).
				Int("step", step).
				Msg("Agent response")

			ai.AddMessage(types.MessageRoleAssistant, response.Text)

			// 4.1 Update the conversation status to succeeded so that hook don't see it as running
			ai.Status = agent.ConversationStatusSucceeded
			err = ai.Update(ctx, rt.db)
			if err != nil {
				return ez.Wrap(op, err)
			}

			// 4.2 Check if any hooks want to block the stop
			blockStop := false

			err = ai.RunConversationEndedHook(ctx)
			if err != nil {

				blockStop = true

				// Since a hook has requested more work, we set the conversation back to running
				ai.Status = agent.ConversationStatusRunning
				err = ai.Update(ctx, rt.db)
				if err != nil {
					return ez.Wrap(op, err)
				}
			}

			if !blockStop {

				ai.Cost = ai.provider.CalculateCost(ai.Model, ai.InputTokens, ai.OutputTokens, ai.CachedTokens)

				log.Info().
					Str("Name", ai.AgentName).
					Str("ID", ai.ID.String()).
					Int("step", step).
					Msg("Agent finished")
				return nil
			}
		}

		err = ai.Update(ctx, rt.db)
		if err != nil {
			return ez.Wrap(op, err)
		}
	}

	return ez.Root(op, ez.ERESOURCEEXHAUSTED, "exceeded maximum inference steps")
}
