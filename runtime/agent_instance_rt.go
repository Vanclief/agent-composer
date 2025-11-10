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

func (rt *Runtime) NewConversationInstanceFromSpec(ctx context.Context, agentSpecID uuid.UUID, sessionID string) (*ConversationInstance, error) {
	const op = "runtime.NewConversationInstanceFromSpec"

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

	conversation.SessionID = sessionID

	return rt.newAgentInstance(ctx, conversation, true)
}

func (rt *Runtime) NewConversationInstance(ctx context.Context, conversationID uuid.UUID) (*ConversationInstance, error) {
	const op = "runtime.NewConversationInstance"

	// Step 1) Load the existing conversation
	conversation, err := agent.GetConversationByID(ctx, rt.db, conversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, conversation, false)
}

func (rt *Runtime) newAgentInstance(ctx context.Context, conversation *agent.Conversation, new bool) (*ConversationInstance, error) {
	const op = "runtime.NewAgentInstance"

	// Step 1) Create the ChatGPT instance
	chatGPT, err := chatgpt.New(rt.openai)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	var tools []types.ToolDefinition
	var mux *mcp.Mux

	// Step 4) Create the MCP servers and mux them
	if conversation.ShellAccess {
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

	ci := &ConversationInstance{
		Conversation: conversation,
		provider:     chatGPT,
		mcpMux:       mux,
		hooks:        hooks,
	}

	return ci, nil
}
