package sandboxes

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// Resume starts the messaging listener of every running sandbox, for agc
// starting while sandboxes kept running. Each listener speaks for the
// sandbox's task. A sandbox that fails is logged and skipped.
func (api *API) Resume(ctx context.Context) error {
	records, err := taskmodels.ListSandboxes(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	owners, err := api.tasksByID(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	for i := range records {
		record := &records[i]
		name := record.Name

		if record.BootedAt == nil || record.BootError != nil {
			continue
		}

		status, err := api.rt.Status(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot read sandbox status")
			continue
		}

		if status != StatusRunning {
			continue
		}

		spec, err := decodeSpec(record)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot resume sandbox messaging")
			continue
		}

		err = api.startSandboxListener(name, owners.of(record), spec)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot resume sandbox messaging")
		}
	}

	return nil
}

// taskIndex maps task ids to tasks.
type taskIndex map[uuid.UUID]*taskmodels.Task

// of returns the task a sandbox is allocated to, or an empty task when it
// is gone.
func (index taskIndex) of(record *taskmodels.Sandbox) *taskmodels.Task {
	owner, ok := index[record.TaskID]
	if !ok {
		return &taskmodels.Task{}
	}

	return owner
}

// tasksByID indexes every task by id.
func (api *API) tasksByID(ctx context.Context) (taskIndex, error) {
	tasks, err := taskmodels.ListTasks(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	index := taskIndex{}
	for i := range tasks {
		index[tasks[i].ID] = &tasks[i]
	}

	return index, nil
}
