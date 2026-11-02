package schedules

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

type GetRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r GetRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*ScheduleView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(record), nil
}

type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

type ListResponse struct {
	Schedules []ScheduleView `json:"schedules"`
}

// List returns every schedule by name.
func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	records, err := schedulemodels.List(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []ScheduleView{}
	for i := range records {
		views = append(views, *api.view(&records[i]))
	}

	return &ListResponse{Schedules: views}, nil
}
