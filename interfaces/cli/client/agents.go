package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/vanclief/agent-composer/core/resources/agents"
)

// Persistent agents (ADR 0016).

// ListAgents lists every agent with its state.
func (c *Client) ListAgents(ctx context.Context) (*agents.ListResponse, error) {
	out := &agents.ListResponse{}
	err := c.do(ctx, http.MethodGet, "/api/agents", nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GetAgent reads the agent called name.
func (c *Client) GetAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	return c.agentCall(ctx, http.MethodGet, name, "", nil)
}

// CreateAgent makes a numbered agent from a role.
func (c *Client) CreateAgent(ctx context.Context, request *agents.CreateRequest) (*agents.AgentView, error) {
	out := &agents.AgentView{}
	err := c.do(ctx, http.MethodPost, "/api/agents", request, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// KeepAgent names (or renames) the agent and may change its role.
func (c *Client) KeepAgent(ctx context.Context, name string, request *agents.KeepRequest) (*agents.AgentView, error) {
	return c.agentCall(ctx, http.MethodPost, name, "/keep", request)
}

// WakeAgent wakes the agent.
func (c *Client) WakeAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	return c.agentCall(ctx, http.MethodPost, name, "/wake", nil)
}

// SleepAgent puts the agent to sleep.
func (c *Client) SleepAgent(ctx context.Context, name string) (*agents.AgentView, error) {
	return c.agentCall(ctx, http.MethodPost, name, "/sleep", nil)
}

// DeleteAgent deletes the agent and its home; the volume stays.
func (c *Client) DeleteAgent(ctx context.Context, name string) (*agents.DeleteResponse, error) {
	out := &agents.DeleteResponse{}
	err := c.do(ctx, http.MethodDelete, "/api/agents/"+url.PathEscape(name), nil, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *Client) agentCall(ctx context.Context, method string, name string, suffix string, in any) (*agents.AgentView, error) {
	out := &agents.AgentView{}
	err := c.do(ctx, method, "/api/agents/"+url.PathEscape(name)+suffix, in, out)
	if err != nil {
		return nil, err
	}

	return out, nil
}
