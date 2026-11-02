package sandboxes

import (
	"context"

	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// ListRequest lists every sandbox. Work also checks each running clone for
// unpushed work, which runs git in every running sandbox, so it is off by
// default.
type ListRequest struct {
	Work bool `json:"-"`
}

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
		if request.Work {
			api.addWork(ctx, view)
		}
		views = append(views, *view)
	}

	return &ListResponse{Sandboxes: views}, nil
}
