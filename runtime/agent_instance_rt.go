package runtime

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/mcp"
	shellmcp "github.com/vanclief/agent-composer/mcp/shell"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/providers/chatgpt"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

// TODO: Try to take out the Runtime

func (rt *Runtime) NewAgentInstanceFromSpec(ctx context.Context, agentSpecID uuid.UUID) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstanceFromSpec"

	// Step 1) Fetch the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, rt.db, agentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	msgs := []types.Message{*types.NewSystemMessage(spec.Instructions)}

	// Step 2) Create the a new conversation
	conversation, err := agent.NewConversation(spec, msgs)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, spec, conversation, true)
}

func (rt *Runtime) NewAgentInstanceFromConversation(ctx context.Context, conversationID uuid.UUID) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstanceFromConversation"

	// Step 1) Load the existing conversation
	conversation, err := agent.GetConversationByID(ctx, rt.db, conversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2) Check if there is an agent spec
	spec, _ := agent.GetAgentSpecByID(ctx, rt.db, conversation.AgentSpecID)

	return rt.newAgentInstance(ctx, spec, conversation, false)
}

func (rt *Runtime) newAgentInstance(ctx context.Context, spec *agent.Spec, conversation *agent.Conversation, new bool) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstance"

	// Step 2) Create the ChatGPT instance
	chatGPT, err := chatgpt.New(rt.openai)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	var tools []types.ToolDefinition
	var mux *mcp.Mux

	// Step 4) Create the MCP servers and mux them
	// TODO: Need to refactor this horrible flow as spec is checked later
	// probably the conversation should hold the settings instead of the
	// spec so if a spec is deleted we just keep the conversation settings
	if spec != nil && spec.ShellAccess {
		shellMCP, err := shellmcp.NewClient(ctx, "", nil, ".", 0)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		// TODO: Limit what commands the shell can use

		mux, err = mcp.NewMux(ctx, shellMCP)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		// Step 5) Add the tools
		tools, err = mux.ListTools(ctx)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	conversation.Tools = tools

	if new {
		err = conversation.Insert(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	} else {
		err = conversation.Update(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	// Step 6) Load the hooks
	hooks, err := loadInstanceHooks(ctx, rt.db, conversation.AgentName)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 7) Create the instance

	ai := &AgentInstance{
		ID:              conversation.ID,
		conversation:    conversation,
		provider:        chatGPT,
		name:            conversation.AgentName,
		model:           conversation.Model,
		instructions:    conversation.Instructions,
		reasoningEffort: conversation.ReasoningEffort,
		mcpMux:          mux,
		tools:           tools,
		messages:        conversation.Messages,
		hooks:           hooks,
	}

	// Step 8) Create the instance
	if spec != nil {
		ai.autoCompact = spec.AutoCompact
		ai.compactAtPercent = spec.CompactAtPercent
		ai.compactionPrompt = spec.CompactionPrompt
		ai.shellAccess = spec.ShellAccess
		ai.webSearch = spec.WebSearch
		ai.structuredOutput = spec.StructuredOutput
		ai.structuredOutputSchema = spec.StructuredOutputSchema
	}

	return ai, nil
}
