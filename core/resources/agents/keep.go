package agents

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// KeepRequest names the agent (or renames a kept one) and may change its
// role. Agent comes from the path.
type KeepRequest struct {
	Agent string `json:"-"`
	// Name is lowercase letters, digits and dashes.
	Name string `json:"name"`
	// Role, when set, is the template the agent runs from now on.
	Role string `json:"role"`
}

func (r KeepRequest) Validate() error {
	if r.Agent == "" {
		return ez.New(ez.EINVALID, "the agent to keep is required", nil)
	}

	err := agent.ValidateName(strings.TrimSpace(r.Name))
	if err != nil {
		return ez.Wrap(err)
	}

	role := strings.TrimSpace(r.Role)
	if role != "" {
		err = ValidateRole(role)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}

// Keep sets kept and renames the agent; 409 when another agent has the
// name. The home sandbox and its volume keep their names: a rename is a
// label, not a move (ADR 0016 item 3). The tasks it manages follow it to
// the new name, and so does its home's link.
func (api *API) Keep(ctx context.Context, requester interface{}, request *KeepRequest) (*AgentView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	name := strings.TrimSpace(request.Name)
	role := strings.TrimSpace(request.Role)

	if role != "" {
		_, err = LoadTemplate(api.dir, role)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	api.naming.Lock()
	defer api.naming.Unlock()

	record, err := api.find(ctx, request.Agent)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if name != record.Name {
		_, err = agent.GetAgentByName(ctx, api.db, name)
		if err == nil {
			return nil, ez.New(ez.ECONFLICT, "agent "+name+" already exists", nil)
		}
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return nil, ez.Wrap(err)
		}
	}

	old := record.Name
	record.Name = name
	record.Kept = true
	if role != "" {
		record.Role = role
	}

	err = api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := record.Update(ctx, tx, "name", "kept", "role")
		if err != nil {
			return ez.Wrap(err)
		}

		if old == name {
			return nil
		}

		_, err = tx.NewUpdate().Model((*taskmodels.Task)(nil)).Set("manager = ?", name).Where("manager = ?", old).Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}

		_, err = tx.NewUpdate().Model((*taskmodels.Sandbox)(nil)).Set("agent = ?", name).Where("agent = ?", old).Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := api.view(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	return view, nil
}
