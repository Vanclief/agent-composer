package tasks

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
)

type StopRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r StopRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Stop shuts the sandbox down and keeps its disk, then stops the listener.
func (api *API) Stop(ctx context.Context, requester interface{}, request *StopRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.rt.Stop(ctx, record.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.stopListener(record.Name)

	return api.view(ctx, record)
}
