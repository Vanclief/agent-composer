package agents

import (
	"context"
	"strings"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/models/agent"
)

// WakeRequest names the agent to wake; the name comes from the path.
type WakeRequest struct {
	Name string `json:"-"`
}

func (r WakeRequest) Validate() error {
	if r.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

// Wake wakes the agent (ADR 0016 item 4): with no home it allocates one
// from the template, booting in the background (waking); with a stopped
// home it starts it; with a home running or starting it does nothing. A
// home that failed to boot is deleted and allocated again.
func (api *API) Wake(ctx context.Context, requester interface{}, request *WakeRequest) (*AgentView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.wake(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}

// wake does Wake's work on record, refreshed under the wake lock.
func (api *API) wake(ctx context.Context, record *agent.Agent) error {
	api.waking.Lock()
	defer api.waking.Unlock()

	current, err := agent.GetAgentByName(ctx, api.db, record.Name)
	if err != nil {
		return ez.Wrap(err)
	}
	*record = *current

	state, home, err := api.state(ctx, record)
	if err != nil {
		return ez.Wrap(err)
	}

	switch {
	case home == nil:
		return api.allocate(ctx, record)
	case state == StateAwake || state == StateWaking:
		return nil
	case state == StateAsleep:
		_, err = api.sbx.Start(ctx, nil, &sandboxes.StartRequest{Name: home.Name})
		if err != nil {
			return ez.Wrap(err)
		}

		return api.touch(ctx, record)
	case state == sandboxes.StatusFailed:
		_, err = api.sbx.Delete(ctx, nil, &sandboxes.DeleteRequest{Name: home.Name})
		if err != nil {
			return ez.Wrap(err)
		}

		return api.allocate(ctx, record)
	}

	return ez.New(ez.ECONFLICT, "agent "+record.Name+" is "+state+"; wake it once its home "+home.Name+" is stopped or running", nil)
}

// allocate boots the agent's first home (or a new one when the old one is
// gone) from its template, and records it.
func (api *API) allocate(ctx context.Context, record *agent.Agent) error {
	tmpl, err := api.template(record.Role)
	if err != nil {
		return ez.Wrap(err)
	}

	view, err := api.sbx.AllocateHome(ctx, sandboxes.HomeRequest{
		Agent:        record.Name,
		Harness:      record.Harness,
		Sandbox:      tmpl.Sandbox,
		Command:      Command(record),
		Instructions: tmpl.AgentMD(),
	})
	if err != nil {
		return ez.Wrap(err)
	}

	idle := strings.TrimSpace(tmpl.Idle)
	if idle == "" {
		idle = "15m"
	}

	now := time.Now().UTC()
	record.Home = view.Name
	record.Idle = idle
	record.LastActiveAt = &now

	err = record.Update(ctx, api.db, "home", "idle", "last_active_at")
	if err != nil {
		return ez.Wrap(err)
	}

	// The allocation's sandbox.status went out before the home was
	// recorded, so the agent's waking is published here.
	agentView, err := api.viewWith(ctx, record, stateOf(record, view))
	if err != nil {
		return ez.Wrap(err)
	}

	api.publish(agentView)

	return nil
}

// touch records that the agent was woken.
func (api *API) touch(ctx context.Context, record *agent.Agent) error {
	now := time.Now().UTC()
	record.LastActiveAt = &now

	err := record.Update(ctx, api.db, "last_active_at")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Command is the home's job agent: the guest agc runs the agent loop on
// AGENT.md (ADR 0016 item 6; the loop is agc agent run --agent). The name
// is the agent's at its first wake; the loop reads its current one from
// GET /agent on the socket.
func Command(record *agent.Agent) string {
	fields := []string{sandboxes.GuestAgc, "agent", "run", "--agent", record.Name, "--harness", string(record.Harness)}
	if record.Model != "" {
		fields = append(fields, "--model", record.Model)
	}
	fields = append(fields, "--brief", sandboxes.AgentBriefPath)

	return strings.Join(fields, " ")
}
