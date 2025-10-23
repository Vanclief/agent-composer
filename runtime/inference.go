package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type toolCallKey struct{ name, args string }

func (rt *Runtime) RunAgentInstance(instance *AgentInstance, prompt string) error {
	const op = "runtime.RunSessions"

	sessionID := fmt.Sprintf("agent:%s", instance.ID)

	rt.scheduler.RunOnce(rt.rootCtx, sessionID, func(jobCtx context.Context) {
		_, err := rt.runAgentInstance(jobCtx, instance, prompt)
		if err != nil {
			log.Error().Err(err).Str("agent_session_id", instance.ID.String()).Msg("agent session failed")
		}
	})

	return nil
}

func (rt *Runtime) runAgentInstance(ctx context.Context, p *AgentInstance, prompt string) (string, error) {
	const op = "runtime.AgentInstance.Run"

	// Persist a context that survives cancellation but is bounded by timeout.
	pCtx := context.WithoutCancel(ctx)

	// Step 1: Append the user prompt to the messages and update the status
	msg := runtimetypes.NewMessage(runtimetypes.MessageRoleUser, prompt) // Always make it user so we can chain agents
	p.messages = append(p.messages, *msg)

	p.session.Status = agent.SessionStatusRunning
	err := p.session.Update(ctx, rt.db)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	for _, h := range p.hooks[hook.EventTypeSessionStarted] {
		RunHook(ctx, h, p.ID, p.name, "", "", "")
	}

	// Step 2: Run the inference
	inferenceErr := p.runInference(ctx, rt)
	if inferenceErr != nil {
		if strings.Contains(inferenceErr.Error(), "context canceled") {
			p.session.Status = agent.SessionStatusCanceled
		} else {
			p.session.Status = agent.SessionStatusFailed
		}
	} else {
		p.session.Status = agent.SessionStatusSucceeded
	}

	p.session.Messages = p.messages

	err = p.session.Update(pCtx, rt.db)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	if inferenceErr != nil {
		return "", ez.Wrap(op, inferenceErr)
	}

	// Step 3: If there was no error, return the last message content
	lastMsg := p.messages[len(p.messages)-1]

	log.Info().Str("agent_session_id", p.ID.String()).Msg("Finished running inference")

	return lastMsg.Content, nil
}

func (p *AgentInstance) runInference(ctx context.Context, o *Runtime) error {
	const op = "runtime.AgentInstance.runInference"

	const maxSteps = 300

	toolCalls := map[toolCallKey]int{}
	var prevResponseID string

	for step := 0; step < maxSteps; step++ {

		log.Info().Int("step", step).Str("Name", p.session.Name).Msg("Agent session inference")

		chatRequest := runtimetypes.ChatRequest{
			Messages:           p.messages,
			Tools:              p.tools,
			PreviousResponseID: prevResponseID,
			ThinkingEffort:     string(p.reasoningEffort),
		}

		res, err := p.provider.Chat(ctx, p.model, &chatRequest)
		if err != nil {
			log.Debug().Err(err).Msg("Model chat request failed")
			return ez.Wrap(op, err)
		}

		// TODO: This only applies to OpenAI
		prevResponseID = res.ID

		if len(res.ToolCalls) == 0 {

			lastResponse := *runtimetypes.NewAssistantMessage(res.Text)

			p.messages = append(p.messages, lastResponse)

			// Check if any hooks want to block the stop
			blockStop := false
			for _, h := range p.hooks[hook.EventTypeSessionEnded] {
				out, _ := RunHook(ctx, h, p.ID, p.name, lastResponse.Content, "", "")
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *runtimetypes.NewUserMessage(string(out.Stderr)))
					blockStop = true
				}
			}

			if !blockStop {
				log.Info().Str("text", res.Text).Msg("Final assistant response received")
				return nil
			}
		}

		// Make every tool call before running inference again
		for _, call := range res.ToolCalls {

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Msg("Agent calling tool")

			callKey := toolCallKey{name: call.Name, args: call.Arguments}

			lastStepCall, found := toolCalls[callKey]

			if found && step-lastStepCall <= 1 {
				toolCalls[callKey] = step

				log.Warn().Str("tool", call.Name).Str("args", call.Arguments).Msg("Skipping tool call due to anti-loop policy")

				// IMPORTANT: Always satisfy the protocol with a ToolMessage for this call_id.
				// We send a synthetic error payload instead of executing the tool again.
				synthetic := `{"error":"duplicate_tool_call","policy":"anti-loop","message":"Duplicate tool call with identical arguments within one step; tool execution skipped."}`
				p.messages = append(p.messages, *runtimetypes.NewToolMessage(call.Name, call.CallID, synthetic))

				continue
			}

			for _, h := range p.hooks[hook.EventTypePreToolUse] {
				out, _ := RunHook(ctx, h, p.ID, p.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *runtimetypes.NewUserMessage(string(out.Stderr)))
				}
			}

			// TOOL Called
			toolCallResponse, err := p.mcpMux.CallTool(ctx, &call)
			if err != nil {
				return ez.Wrap("agent.ExecuteTool", err)
			}

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Str("tool_response", toolCallResponse).Msg("Tool call response")

			for _, h := range p.hooks[hook.EventTypePostToolUse] {
				out, _ := RunHook(ctx, h, p.ID, p.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *runtimetypes.NewUserMessage(string(out.Stderr)))
				}
			}

			toolCalls[callKey] = step

			p.messages = append(p.messages, *runtimetypes.NewToolMessage(call.Name, call.CallID, toolCallResponse))
		}
	}

	return ez.Root(op, ez.ERESOURCEEXHAUSTED, "exceeded maximum inference steps")
}
