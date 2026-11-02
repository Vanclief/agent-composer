package sandboxes

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r GetRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, owner, err := api.findWithTask(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record, owner.Name)
}
