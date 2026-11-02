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

// Get returns the sandbox with its status and, when it is running and has
// a repo, the work in its clone.
func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, owner, err := api.findWithTask(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := api.view(ctx, record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.addWork(ctx, view)

	return view, nil
}
