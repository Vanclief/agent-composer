package sandboxes

import (
	"context"

	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

type ListResponse struct {
	Sandboxes []SandboxView `json:"sandboxes"`
}

// List returns every sandbox, newest first, each with its status.
func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	records, err := taskmodels.ListSandboxes(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	owners, err := api.tasksByID(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []SandboxView{}
	for i := range records {
		view, err := api.view(ctx, &records[i], owners.of(&records[i]))
		if err != nil {
			return nil, ez.Wrap(err)
		}
		views = append(views, *view)
	}

	return &ListResponse{Sandboxes: views}, nil
}
