package schedules

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

// UpdateRequest replaces a schedule's every, enabled, task and poll; its
// name stays. Enabled defaults to true, as on create. Its last run and
// items are kept, so keys already seen still create nothing.
type UpdateRequest struct {
	ID      uuid.UUID               `json:"id"`
	Name    string                  `json:"name"`
	Every   string                  `json:"every"`
	Enabled *bool                   `json:"enabled"`
	Task    schedulemodels.Template `json:"task"`
	Poll    *schedulemodels.Poll    `json:"poll"`
}

func (r UpdateRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateBody(r.Every, r.Poll)
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*ScheduleView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record.Every = strings.TrimSpace(request.Every)
	record.Enabled = request.Enabled == nil || *request.Enabled
	record.Task = request.Task
	record.Poll = cleanPoll(request.Poll)

	err = api.checkTemplate(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(record), nil
}
