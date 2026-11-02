package sandboxes

import (
	"context"

	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

var _ messaging.Presence = (*API)(nil)

// running reads a sandbox's status without view's side effects: a row
// that has not booted, failed to, or whose runtime cannot say is not
// running.
func (api *API) running(ctx context.Context, record *taskmodels.Sandbox) bool {
	if record.BootError != nil || record.BootedAt == nil {
		return false
	}

	status, err := api.rt.Status(ctx, record.Name)
	if err != nil {
		return false
	}

	return status == StatusRunning
}

// SandboxRunning is messaging's presence for an agent that joined as the
// sandbox name. found is false when no sandbox has that name.
func (api *API) SandboxRunning(ctx context.Context, name string) (bool, bool) {
	record, err := taskmodels.GetSandboxByName(ctx, api.db, name)
	if err != nil {
		return false, false
	}

	return api.running(ctx, record), true
}

// TaskRunning reports whether any sandbox of the task called name runs.
func (api *API) TaskRunning(ctx context.Context, name string) bool {
	owner, err := taskmodels.GetTaskByName(ctx, api.db, name)
	if err != nil {
		return false
	}

	records, err := taskmodels.ListTaskSandboxes(ctx, api.db, owner.ID)
	if err != nil {
		return false
	}

	for i := range records {
		if api.running(ctx, &records[i]) {
			return true
		}
	}

	return false
}
