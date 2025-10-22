package orchestra

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/mcp"
	shellmcp "github.com/vanclief/agent-composer/mcp/shell"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/models/parrot"
	"github.com/vanclief/agent-composer/runtime/llm"
)

type ParrotRunInstance struct {
	ID              uuid.UUID
	run             *parrot.Run
	provider        llm.Provider
	name            string
	model           string
	instructions    string
	reasoningEffort llm.ReasoningEffort
	mcpMux          *mcp.Mux
	tools           []llm.ToolDefinition
	messages        []llm.Message
	hooks           map[hook.EventType][]hook.Hook
}

type toolCallKey struct{ name, args string }

const defaultParrotPolicy = `
Policy:
- Use other tools only when strictly necessary. Do not re-run a tool just to "confirm".
- NEVER call the same tool with identical arguments twice in a row. If you must retry, briefly explain why and change the arguments.`

func NewParrotRunInstance(ctx context.Context, db bun.IDB, parrotTemplateID uuid.UUID) (*ParrotRunInstance, error) {
	const op = "runtime.NewParrotRunInstance"

	// Step 1) Fetch the parrot template
	pt, err := parrot.GetParrotTemplateByID(ctx, db, parrotTemplateID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2) Create the provider
	llmProvider, err := llm.NewOpenAI()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	pt.Instructions += "\n" + defaultParrotPolicy

	msgs := []llm.Message{*llm.NewSystemMessage(pt.Instructions)}

	// Step 3) Create the parrot run
	pr, err := parrot.NewParrotRun(pt, msgs)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 4) Create the MCP servers and mux them
	// TODO: This is currently hardcoded
	shellMCP, err := shellmcp.NewClient(ctx, "", nil, ".", 0)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	mux, err := mcp.NewMux(ctx, shellMCP)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 5) Add the tools
	tools, err := mux.ListTools(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	filteredTools := []llm.ToolDefinition{}
	if len(pt.AllowedTools) > 0 {
		if len(pt.AllowedTools) == 1 && strings.EqualFold(pt.AllowedTools[0], "all") {
			filteredTools = tools
		} else {
			allowedSet := make(map[string]struct{}, len(pt.AllowedTools))

			for _, name := range pt.AllowedTools {
				if name == "" {
					continue
				}
				allowedSet[name] = struct{}{}
			}

			filtered := make([]llm.ToolDefinition, 0, len(tools))
			for _, tool := range tools {
				_, ok := allowedSet[tool.Name]
				if ok {
					filtered = append(filtered, tool)
				}
			}
			filteredTools = filtered
		}
	}

	pr.Tools = filteredTools

	err = pr.Insert(ctx, db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 6) Load the hooks
	hooks, err := loadInstanceHooks(ctx, db, pt.Name)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ParrotRunInstance{
		ID:              pr.ID,
		run:             pr,
		provider:        llmProvider,
		name:            pt.Name,
		model:           pt.Model,
		instructions:    pr.Instructions,
		reasoningEffort: pt.ReasoningEffort,
		mcpMux:          mux,
		tools:           filteredTools,
		messages:        msgs,
		hooks:           hooks,
	}, nil
}

func (p *ParrotRunInstance) Run(ctx context.Context, o *Orchestrator, prompt string) (string, error) {
	const op = "parrot.run"

	// Persist a context that survives cancellation but is bounded by timeout.
	pCtx := context.WithoutCancel(ctx)

	// Step 1: Append the user prompt to the messages and update the status
	msg := llm.NewMessage(llm.MessageRoleUser, prompt) // Always make it user so we can chain parrots
	p.messages = append(p.messages, *msg)

	p.run.Status = parrot.RunStatusRunning
	err := p.run.Update(ctx, o.db)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	for _, h := range p.hooks[hook.EventTypeRunStarted] {
		RunHook(ctx, h, p.ID, p.name, "", "", "")
	}

	// Step 2: Run the inference
	inferenceErr := p.runInference(ctx, o)
	if inferenceErr != nil {
		if strings.Contains(inferenceErr.Error(), "context canceled") {
			p.run.Status = parrot.RunStatusCanceled
		} else {
			p.run.Status = parrot.RunStatusFailed
		}
	} else {
		p.run.Status = parrot.RunStatusSucceeded
	}

	p.run.Messages = p.messages

	err = p.run.Update(pCtx, o.db)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	if inferenceErr != nil {
		return "", ez.Wrap(op, inferenceErr)
	}

	// Step 3: If there was no error, return the last message content
	lastMsg := p.messages[len(p.messages)-1]

	log.Info().Str("parrot_run_id", p.ID.String()).Msg("Finished running inference")

	return lastMsg.Content, nil
}

func (p *ParrotRunInstance) runInference(ctx context.Context, o *Orchestrator) error {
	const op = "parrot.runInference"

	const maxSteps = 300

	toolCalls := map[toolCallKey]int{}
	var prevResponseID string

	for step := 0; step < maxSteps; step++ {

		log.Info().Int("step", step).Str("Name", p.run.Name).Msg("Parrot running inference")

		chatRequest := llm.ChatRequest{
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

			lastResponse := *llm.NewAssistantMessage(res.Text)

			p.messages = append(p.messages, lastResponse)

			// Check if any hooks want to block the stop
			blockStop := false
			for _, h := range p.hooks[hook.EventTypeRunEnded] {
				out, _ := RunHook(ctx, h, p.ID, p.name, lastResponse.Content, "", "")
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *llm.NewUserMessage(string(out.Stderr)))
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

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Msg("Parrot calling tool")

			callKey := toolCallKey{name: call.Name, args: call.Arguments}

			lastStepCall, found := toolCalls[callKey]

			if found && step-lastStepCall <= 1 {
				toolCalls[callKey] = step

				log.Warn().Str("tool", call.Name).Str("args", call.Arguments).Msg("Skipping tool call due to anti-loop policy")

				// IMPORTANT: Always satisfy the protocol with a ToolMessage for this call_id.
				// We send a synthetic error payload instead of executing the tool again.
				synthetic := `{"error":"duplicate_tool_call","policy":"anti-loop","message":"Duplicate tool call with identical arguments within one step; tool execution skipped."}`
				p.messages = append(p.messages, *llm.NewToolMessage(call.Name, call.CallID, synthetic))

				continue
			}

			for _, h := range p.hooks[hook.EventTypePreToolUse] {
				out, _ := RunHook(ctx, h, p.ID, p.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *llm.NewUserMessage(string(out.Stderr)))
				}
			}

			// TOOL Called
			toolCallResponse, err := p.mcpMux.CallTool(ctx, &call)
			if err != nil {
				return ez.Wrap("parrot.ExecuteTool", err)
			}

			log.Info().Str("tool", call.Name).Str("args", call.Arguments).Str("tool_response", toolCallResponse).Msg("Tool call response")

			for _, h := range p.hooks[hook.EventTypePostToolUse] {
				out, _ := RunHook(ctx, h, p.ID, p.name, "", call.Name, call.Arguments)
				if out.ExitCode == 2 {
					p.messages = append(p.messages, *llm.NewUserMessage(string(out.Stderr)))
				}
			}

			toolCalls[callKey] = step

			p.messages = append(p.messages, *llm.NewToolMessage(call.Name, call.CallID, toolCallResponse))
		}
	}

	return ez.Root(op, ez.ERESOURCEEXHAUSTED, "exceeded maximum inference steps")
}
