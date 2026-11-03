package agents

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/models/agent"
)

// SleepRequest names the agent to put to sleep; the name comes from the
// path.
type SleepRequest struct {
	Name string `json:"-"`
}

func (r SleepRequest) Validate() error {
	if r.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

// Sleep stops an awake agent's home and posts "Asleep." in its inbox #<name>
// so the thread shows where the VM stopped (ADR 0016 item 4). An agent
// that is not awake answers 409.
func (api *API) Sleep(ctx context.Context, requester interface{}, request *SleepRequest) (*AgentView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.sleep(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}

func (api *API) sleep(ctx context.Context, record *agent.Agent) error {
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

	if state != StateAwake {
		return ez.New(ez.ECONFLICT, "agent "+record.Name+" is "+state+"; only an awake agent can sleep", nil)
	}

	_, err = api.sbx.Stop(ctx, nil, &sandboxes.StopRequest{Name: home.Name})
	if err != nil {
		return ez.Wrap(err)
	}

	dm := DMChannel(record)
	if dm == "" {
		return nil
	}

	// The VM is already stopped, so a failed post is logged, not returned.
	_, err = api.msg.PostAsSystem(ctx, dm, "Asleep.")
	if err != nil {
		log.Warn().Err(err).Str("agent", record.Name).Str("channel", dm).Msg("cannot post Asleep. in the agent's inbox")
	}

	return nil
}
