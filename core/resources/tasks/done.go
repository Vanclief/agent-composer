package tasks

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
)

type DoneRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r DoneRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Done sets done_at, the only thing that does. Its sandboxes are left as
// they are. A task already done keeps its first done_at.
func (api *API) Done(ctx context.Context, requester interface{}, request *DoneRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.MarkDone(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}
