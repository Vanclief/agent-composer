package tasks

import (
	"context"

	"github.com/google/uuid"
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

// Delete removes the task's sandboxes, VMs and rows, then the task.
// Volumes and the task's channel history are kept (ADR 0003, 0005).
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	sandboxRecords, err := taskmodels.ListTaskSandboxes(ctx, api.db, record.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for i := range sandboxRecords {
		err = api.Sandboxes.Remove(ctx, &sandboxRecords[i])
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	err = record.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{Deleted: true, Name: record.Name}, nil
}
