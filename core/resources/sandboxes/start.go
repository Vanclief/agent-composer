package sandboxes

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/task"
)

type StartRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r StartRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Start boots a stopped sandbox from its disk. Secrets are read again from
// the store of the sandbox's server, since their values may have changed.
func (api *API) Start(ctx context.Context, requester interface{}, request *StartRequest) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, owner, err := api.findWithTask(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = requireBooted(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec, err := decodeSpec(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	bindings, err := api.resolveSecretsOn(ctx, record.Server, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.startSandboxListener(record, owner, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.rt.Start(ctx, record.Name, task.SandboxSecrets(bindings))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record.StoppedAt = nil

	err = record.Update(ctx, api.db, "stopped_at")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := api.view(ctx, record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	return view, nil
}
