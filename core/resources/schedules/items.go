package schedules

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

type ListItemsRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r ListItemsRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// ItemView is one key the schedule has seen and the task made for it.
type ItemView struct {
	Key    string    `json:"key"`
	Task   string    `json:"task"`
	TaskID uuid.UUID `json:"task_id"`
	SeenAt time.Time `json:"seen_at"`
}

type ListItemsResponse struct {
	Items []ItemView `json:"items"`
}

// ListItems returns the schedule's items, newest first.
func (api *API) ListItems(ctx context.Context, requester interface{}, request *ListItemsRequest) (*ListItemsResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	items, err := schedulemodels.ListItems(ctx, api.db, record.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := make([]ItemView, 0, len(items))
	for _, item := range items {
		views = append(views, ItemView{Key: item.Key, Task: item.Task, TaskID: item.TaskID, SeenAt: item.SeenAt})
	}

	return &ListItemsResponse{Items: views}, nil
}
