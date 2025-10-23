package runtime

import (
	"context"

	"github.com/google/uuid"
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
	session         *agent.Session
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

	// Step 2) Create the a new agent session
	session, err := agent.NewAgentSession(spec, msgs)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, session, true)
}

func (rt *Runtime) NewAgentInstanceFromSession(ctx context.Context, agentSessionID uuid.UUID) (*AgentInstance, error) {
	const op = "runtime.NewAgentInstanceFromSession"

	// Step 1) Load the existing session
	session, err := agent.GetAgentSessionByID(ctx, rt.db, agentSessionID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, session, false)
}

func (rt *Runtime) newAgentInstance(ctx context.Context, session *agent.Session, new bool) (*AgentInstance, error) {
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

	session.Tools = tools

	if new {
		err = session.Insert(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	} else {
		err = session.Update(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	// Step 6) Load the hooks
	hooks, err := loadInstanceHooks(ctx, rt.db, session.Name)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &AgentInstance{
		ID:              session.ID,
		session:         session,
		provider:        chatGPT,
		name:            session.Name,
		model:           session.Model,
		instructions:    session.Instructions,
		reasoningEffort: session.ReasoningEffort,
		mcpMux:          mux,
		tools:           tools,
		messages:        session.Messages,
		hooks:           hooks,
	}, nil
}
