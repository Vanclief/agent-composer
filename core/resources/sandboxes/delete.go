package sandboxes

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type DeleteRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r DeleteRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

type DeleteResponse struct {
	Deleted bool   `json:"deleted"`
	Name    string `json:"name"`
}

// Delete removes the VM and its disk, the listener and the row. Volumes and
// the task's channel history are kept (ADR 0003, 0005).
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.Remove(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{Deleted: true, Name: record.Name}, nil
}

// Remove cancels a boot in progress, removes the VM, the listener, the
// socket and the row. A VM the runtime no longer knows is treated as
// already removed.
func (api *API) Remove(ctx context.Context, record *taskmodels.Sandbox) error {
	name := record.Name

	api.cancelBoot(name)

	status, err := api.rt.Status(ctx, name)
	if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
		return ez.Wrap(err)
	}

	if err == nil {
		if status != StatusStopped {
			err = api.rt.Stop(ctx, name)
			if err != nil {
				return ez.Wrap(err)
			}
		}

		err = api.rt.Remove(ctx, name)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	api.stopListener(name)

	err = os.Remove(api.socketPath(name))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ez.New(ez.EINTERNAL, "cannot remove socket "+api.socketPath(name), err)
	}

	err = record.Delete(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	api.publishDeleted(ctx, record)

	return nil
}

// publishDeleted sends the sandbox's last view with status deleted.
func (api *API) publishDeleted(ctx context.Context, record *taskmodels.Sandbox) {
	if api.events == nil {
		return
	}

	owner, err := api.ownerOf(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot read the task for sandbox.status deleted")
		return
	}

	view, err := baseView(record, owner)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot build the view for sandbox.status deleted")
		return
	}

	view.Status = StatusDeleted
	api.publish(view)
}
