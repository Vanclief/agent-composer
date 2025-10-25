package runtime

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/mcp"
	shellmcp "github.com/vanclief/agent-composer/mcp/shell"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/runtime/providers"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type AgentInstance struct {
	ID              uuid.UUID
	conversation    *agent.Conversation
	provider        types.LLMProvider
	name            string
	model           string
	instructions    string
	reasoningEffort types.ReasoningEffort
	mcpMux          *mcp.Mux
	tools           []types.ToolDefinition
	messages        []types.Message
	hooks           map[hook.EventType][]hook.Hook
}

const defaultAgentPolicy = `
Policy:
- Use other tools only when strictly necessary. Do not re-run a tool just to "confirm".
- NEVER call the same tool with identical arguments twice in a row. If you must retry, briefly explain why and change the arguments.`

func (rt *Runtime) NewAgentInstanceFromSpec(ctx context.Context, agentSpecID uuid.UUID) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstanceFromSpec"

	// Step 1) Fetch the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, rt.db, agentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	spec.Instructions += "\n" + defaultAgentPolicy

	msgs := []types.Message{*types.NewSystemMessage(spec.Instructions)}

	// Step 2) Create the a new conversation
	conversation, err := agent.NewConversation(spec, msgs)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, conversation, true)
}

func (rt *Runtime) NewAgentInstanceFromConversation(ctx context.Context, conversationID uuid.UUID) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstanceFromConversation"

	// Step 1) Load the existing conversation
	conversation, err := agent.GetConversationByID(ctx, rt.db, conversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, conversation, false)
}

func (rt *Runtime) newAgentInstance(ctx context.Context, conversation *agent.Conversation, new bool) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstance"

	// Step 2) Create the ChatGPT instance
	chatGPT, err := providers.NewChatGPT(rt.openai)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 4) Create the MCP servers and mux them
	// TODO: This is currently hardcoded
	shellMCP, err := shellmcp.NewClient(ctx, "", nil, ".", 0)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Limit what commands the shell can use

	mux, err := mcp.NewMux(ctx, shellMCP)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 5) Add the tools
	tools, err := mux.ListTools(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
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
	hooks, err := loadInstanceHooks(ctx, rt.db, conversation.Name)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &AgentInstance{
		ID:              conversation.ID,
		conversation:    conversation,
		provider:        chatGPT,
		name:            conversation.Name,
		model:           conversation.Model,
		instructions:    conversation.Instructions,
		reasoningEffort: conversation.ReasoningEffort,
		mcpMux:          mux,
		tools:           tools,
		messages:        conversation.Messages,
		hooks:           hooks,
	}, nil
}

func (ai *AgentInstance) AddMessage(role types.MessageRole, content string) {
	var msg types.Message

	switch role {
	case types.MessageRoleSystem:
		msg = *types.NewSystemMessage(content)
	case types.MessageRoleUser:
		msg = *types.NewUserMessage(content)
	case types.MessageRoleAssistant:
		msg = *types.NewAssistantMessage(content)
	default:
		log.Error().Msg("Invalid message role")
		return // Invalid role; do nothing
	}

	ai.messages = append(ai.messages, msg)
}

func (ai *AgentInstance) AddToolMessage(toolName, toolCallID, content string) {
	msg := *types.NewToolMessage(toolName, toolCallID, content)
	ai.messages = append(ai.messages, msg)
}
