package tasks

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
	Tasks []TaskView `json:"tasks"`
}

// List returns every task, newest first, each with its live status.
func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	records, err := taskmodels.ListTasks(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []TaskView{}
	for i := range records {
		view, err := api.view(ctx, &records[i])
		if err != nil {
			return nil, ez.Wrap(err)
		}
		views = append(views, *view)
	}

	return &ListResponse{Tasks: views}, nil
}
