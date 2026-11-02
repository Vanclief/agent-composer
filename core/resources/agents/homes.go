package agents

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/agent"
)

// homes serves the agent routes on homes' sockets (sandboxes.Homes).
type homes struct {
	api *API
}

var _ sandboxes.Homes = homes{}

// HomeAgent answers GET /agent on a home's socket: the agent, the channels
// it reads now, the channels outside those where it started a thread
// (Asked: only the replies in its threads there are turns), and how long
// it may be idle before it sleeps.
type HomeAgent struct {
	Agent    *AgentView `json:"agent"`
	Channels []string   `json:"channels"`
	Asked    []string   `json:"asked"`
	Idle     string     `json:"idle"`
}

// agentOf returns the agent living in home and its template, nil when the
// template cannot be read.
func (h homes) agentOf(ctx context.Context, home string) (*agent.Agent, *Template, error) {
	record, err := agent.GetAgentByHome(ctx, h.api.db, home)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	tmpl, err := LoadTemplate(h.api.dir, record.Role)
	if err != nil {
		return record, nil, nil
	}

	return record, tmpl, nil
}

// Caller is who a home speaks as in messaging: its agent, by the home's
// name, granted the channels it reads now (Channels) and, like a task
// sandbox (ADR 0009 item 5), its owner's inbox #<owner>, so it can ask the
// owner there, and the channels where it started a thread (Asked), so it
// reads the replies and answers in the thread. The inbox is not in Reads:
// only a reply in a thread the agent started there wakes it. The main
// role needs no grants: messaging lets it in everywhere.
func (h homes) Caller(ctx context.Context, home string) (messaging.Caller, error) {
	record, tmpl, err := h.agentOf(ctx, home)
	if err != nil {
		return messaging.Caller{}, ez.New(ez.ENOTAUTHORIZED, "sandbox "+home+" is no agent's home", err)
	}

	channels, err := h.api.Channels(ctx, record, tmpl)
	if err != nil {
		return messaging.Caller{}, ez.Wrap(err)
	}

	asked, err := h.api.Asked(ctx, record, channels)
	if err != nil {
		return messaging.Caller{}, ez.Wrap(err)
	}
	channels = append(channels, asked...)

	grants := []messaging.Grant{}
	for _, channel := range channels {
		grants = append(grants, messaging.Grant{Channel: channel})
	}

	if record.Owner != "" && !slices.Contains(channels, "#"+record.Owner) {
		grants = append(grants, messaging.Grant{Channel: "#" + record.Owner})
	}

	return messaging.Caller{Agent: home, Grants: grants}, nil
}

func (h homes) Agent(ctx context.Context, home string) (any, error) {
	record, tmpl, err := h.agentOf(ctx, home)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := h.api.view(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	channels, err := h.api.Channels(ctx, record, tmpl)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	asked, err := h.api.Asked(ctx, record, channels)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	idle := record.Idle
	if idle == "" {
		idle = "15m"
	}

	return &HomeAgent{Agent: view, Channels: channels, Asked: asked, Idle: idle}, nil
}

func (h homes) Sleep(ctx context.Context, home string) error {
	record, err := agent.GetAgentByHome(ctx, h.api.db, home)
	if err != nil {
		return ez.Wrap(err)
	}

	return h.api.sleep(ctx, record)
}

// CreateTask creates a task from a home: the agent is its manager whatever
// the body says, and its owner, when the body names none (ADR 0016 item
// 8).
func (h homes) CreateTask(ctx context.Context, home string, body []byte) (any, error) {
	record, err := agent.GetAgentByHome(ctx, h.api.db, home)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	request := &tasks.CreateRequest{}

	err = json.Unmarshal(body, request)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "request body must be a task create request", err)
	}

	request.Manager = record.Name
	if request.Owner == "" {
		request.Owner = record.Owner
	}

	return h.api.tasks.Create(ctx, nil, request)
}

// managedTask returns the agent living in home and its task called
// taskName, refused when the agent does not manage it.
func (h homes) managedTask(ctx context.Context, home string, taskName string) (*agent.Agent, *tasks.TaskView, error) {
	record, err := agent.GetAgentByHome(ctx, h.api.db, home)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	view, err := h.api.tasks.Get(ctx, nil, &tasks.GetRequest{Name: taskName})
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	if view.Manager != record.Name {
		return nil, nil, ez.New(ez.ENOTAUTHORIZED, "task "+taskName+" is managed by "+view.Manager+", not by "+record.Name, nil)
	}

	return record, view, nil
}

// GetTask reads a task the agent manages.
func (h homes) GetTask(ctx context.Context, home string, taskName string) (any, error) {
	_, view, err := h.managedTask(ctx, home, taskName)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return view, nil
}

// AcceptTask accepts a task the agent manages, as the agent.
func (h homes) AcceptTask(ctx context.Context, home string, taskName string) (any, error) {
	record, _, err := h.managedTask(ctx, home, taskName)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return h.api.tasks.Accept(ctx, nil, &tasks.AcceptRequest{Name: taskName, By: record.Name})
}

// RejectTask sends back a task the agent manages, as the agent.
func (h homes) RejectTask(ctx context.Context, home string, taskName string, why string) (any, error) {
	record, _, err := h.managedTask(ctx, home, taskName)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return h.api.tasks.Reject(ctx, nil, &tasks.RejectRequest{Name: taskName, By: record.Name, Why: why})
}
