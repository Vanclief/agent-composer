package tasks

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

// Start boots a stopped Task from its disk. Secrets are read again from
// the store, since their values may have changed.
func (api *API) Start(ctx context.Context, requester interface{}, request *StartRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec, err := decodeSpec(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	bindings, err := api.resolveSecrets(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.startTaskListener(record.Name, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.rt.Start(ctx, record.Name, task.SandboxSecrets(bindings))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}
