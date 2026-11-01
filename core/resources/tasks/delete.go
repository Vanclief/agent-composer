package tasks

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
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

// Delete removes the sandbox and its disk, the listener and the row.
// Volumes and the Task's channel history are kept (ADR 0003, 0005). A
// sandbox the runtime no longer knows is treated as already removed.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	name := record.Name

	status, err := api.rt.Status(ctx, name)
	if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	if err == nil {
		if status != statusStopped {
			err = api.rt.Stop(ctx, name)
			if err != nil {
				return nil, ez.Wrap(err)
			}
		}

		err = api.rt.Remove(ctx, name)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	api.stopListener(name)

	err = os.Remove(api.socketPath(name))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, ez.New(ez.EINTERNAL, "cannot remove socket "+api.socketPath(name), err)
	}

	err = record.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{Deleted: true, Name: name}, nil
}
