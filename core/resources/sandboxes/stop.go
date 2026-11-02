package sandboxes

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type StopRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r StopRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Stop shuts the VM down and keeps its disk, then stops the listener.
func (api *API) Stop(ctx context.Context, requester interface{}, request *StopRequest) (*SandboxView, error) {
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

	err = api.rt.Stop(ctx, record.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.stopListener(record.Name)

	now := time.Now().UTC()
	record.StoppedAt = &now

	err = record.Update(ctx, api.db, "stopped_at")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record, owner.Name)
}

// requireBooted refuses runtime operations on a sandbox that is still
// starting or failed to boot.
func requireBooted(record *taskmodels.Sandbox) error {
	if record.BootError != nil {
		return ez.New(ez.ECONFLICT, "sandbox "+record.Name+" failed to boot: "+*record.BootError, nil)
	}

	if record.BootedAt == nil {
		return ez.New(ez.ECONFLICT, "sandbox "+record.Name+" is still starting", nil)
	}

	return nil
}
