package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
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
	ai.conversation.Status = agent.ConversationStatusRunning

	err := ai.conversation.Update(ctx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	// Step 2: Run any session started hooks
	for _, h := range ai.hooks[hook.EventTypeConversationStarted] {
		RunHook(ctx, h, ai.ID, ai.name, "", "", "")
	}

	// Step 3: Run the inference
	inferenceErr := rt.runInference(ctx, ai)
	if inferenceErr != nil {
		if strings.Contains(inferenceErr.Error(), "context canceled") {
			ai.conversation.Status = agent.ConversationStatusCanceled
		} else {
			ai.conversation.Status = agent.ConversationStatusFailed
		}
	}

	ai.conversation.Messages = ai.messages

	pCtx := context.WithoutCancel(ctx)
	err = ai.conversation.Update(pCtx, rt.db)
	if err != nil {
		return ez.Wrap(op, err)
	}

	if inferenceErr != nil {
		return ez.Wrap(op, inferenceErr)
	}

	// Step 4: If there was no error, return the last message content
	// lastMsg := ai.messages[len(ai.messages)-1]
	log.Info().Str("conversation_id", ai.ID.String()).Msg("Finished running inference")

	return nil
}

func (rt *Runtime) runInference(ctx context.Context, ai *AgentInstance) error {
	const op = "runtime.AgentInstance.runInference"

	const maxSteps = 300

	toolCalls := map[toolCallKey]int{}
	var prevResponseID string // This is for OpenAI

	for step := 0; step < maxSteps; step++ {

		log.Info().Int("step", step).Str("Name", ai.conversation.Name).Msg("Conversation inference")

		// Step 1: Create the chat request
		chatRequest := types.ChatRequest{
			Messages:           ai.messages,
			Tools:              ai.tools,
			PreviousResponseID: prevResponseID,
			ThinkingEffort:     string(ai.reasoningEffort),
		}

		// Step 2: Call the chat
		res, err := ai.provider.Chat(ctx, ai.model, &chatRequest)
		if err != nil {
			log.Debug().Err(err).Msg("Model chat request failed")
			return ez.Wrap(op, err)
		}

		prevResponseID = res.ID // NOTE: This only applies to OpenAI

		// Step 3: If we do have tool calls, execute them
		for _, call := range res.ToolCalls {

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Msg("Agent calling tool")

			// 3.1 Create a tool call key
			callKey := toolCallKey{name: call.Name, args: call.Arguments}

			// 3.2 Check that we are not in an infinite loop of tool calls with identical arguments
			lastStepCall, found := toolCalls[callKey]

			if found && step-lastStepCall <= 1 {
				toolCalls[callKey] = step

				log.Warn().Str("tool", call.Name).Str("args", call.Arguments).Msg("Skipping tool call due to anti-loop policy")

				// IMPORTANT: Always satisfy the protocol with a ToolMessage for this call_id.
				// We send a synthetic error payload instead of executing the tool again.
				synthetic := `{"error":"duplicate_tool_call","policy":"anti-loop","message":"Duplicate tool call with identical arguments within one step; tool execution skipped."}`
				ai.AddToolMessage(call.Name, call.CallID, synthetic)

				continue
			}

			// 3.3 Run any pre-tool-use hooks
			for _, h := range ai.hooks[hook.EventTypePreToolUse] {
				out, _ := RunHook(ctx, h, ai.ID, ai.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					ai.AddMessage(types.MessageRoleUser, string(out.Stderr))
				}
			}

			// 3.4 Call the tool
			toolCallResponse, err := ai.mcpMux.CallTool(ctx, &call)
			if err != nil {
				return ez.Wrap("agent.ExecuteTool", err)
			}

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Str("tool_response", toolCallResponse).Msg("Tool call response")

			// 3.5 Run any post-tool-use hooks
			for _, h := range ai.hooks[hook.EventTypePostToolUse] {
				out, _ := RunHook(ctx, h, ai.ID, ai.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					ai.AddMessage(types.MessageRoleUser, string(out.Stderr))
				}
			}

			// 3.6 Record the tool call step
			toolCalls[callKey] = step

			ai.AddToolMessage(call.Name, call.CallID, toolCallResponse)
		}

		// Step 4: If we don't have any tool calls
		if len(res.ToolCalls) == 0 {

			ai.AddMessage(types.MessageRoleAssistant, res.Text)

			// 4.1 Update the conversation status to succeeded so that hook don't see it as running
			ai.conversation.Status = agent.ConversationStatusSucceeded
			err = ai.conversation.Update(ctx, rt.db)
			if err != nil {
				return ez.Wrap(op, err)
			}

			// 4.2 Check if any hooks want to block the stop
			blockStop := false
			for _, h := range ai.hooks[hook.EventTypeConversationEnded] {

				out, _ := RunHook(ctx, h, ai.ID, ai.name, res.Text, "", "")
				if out.ExitCode == 2 {

					blockStop = true
					ai.AddMessage(types.MessageRoleUser, (string(out.Stderr)))

					// Since a hook has requested more work, we set the conversation back to running
					ai.conversation.Status = agent.ConversationStatusRunning
					err = ai.conversation.Update(ctx, rt.db)
					if err != nil {
						return ez.Wrap(op, err)
					}
				}
			}

			if !blockStop {
				log.Info().Str("text", res.Text).Msg("Final assistant response received")
				return nil
			}
		}

	}

	return ez.Root(op, ez.ERESOURCEEXHAUSTED, "exceeded maximum inference steps")
}
