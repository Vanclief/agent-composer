package schedules

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/tasks"
)

type RunRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r RunRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// RunResponse is one tick: the schedule after it, the tasks it created and
// its error, empty when it succeeded (also the schedule's last_error).
type RunResponse struct {
	Schedule ScheduleView     `json:"schedule"`
	Created  []tasks.TaskView `json:"created"`
	Error    string           `json:"error"`
}

// Run ticks the schedule now, enabled or not and due or not. It is a
// conflict while another tick of the schedule runs.
func (api *API) Run(ctx context.Context, requester interface{}, request *RunRequest) (*RunResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.tick(ctx, record.ID, record.Name, true)
}
