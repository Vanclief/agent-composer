package schedules

import (
	"context"

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
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Delete removes the schedule and its items. The tasks it created stay.
// A tick running now finishes.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{ID: record.ID, Name: record.Name}, nil
}
