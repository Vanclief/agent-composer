package sandboxes

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// Resume starts the messaging listener of every running sandbox, for agc
// starting while sandboxes kept running. Each listener speaks for the
// sandbox's task. A sandbox that fails is logged and skipped.
//
// With restart, which only the server asks for, it also starts again every
// sandbox that went down without agc stopping it while its task is not
// done (Recoverable: after a reboot msb reports every VM that was running
// as crashed), the way Restart does, and posts one line as agc in #<task>
// and #<owner> per task: that it came back, or why a sandbox did not. A
// failure is logged, posted that once and skipped; the watchdog's
// heartbeat tries again later (ADR 0017).
func (api *API) Resume(ctx context.Context, restart bool) error {
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
		owner := owners.of(record)

		if record.BootedAt == nil || record.BootError != nil {
			continue
		}

		status, err := api.rt.Status(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot read sandbox status")
			continue
		}

		if restart && Recoverable(record, owner, status) {
			api.recoverAtStart(ctx, record, owner, status)
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

		err = api.startSandboxListener(name, owner, spec)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot resume sandbox messaging")
		}
	}

	return nil
}

// recoverAtStart restarts one sandbox found down at startup and says so.
func (api *API) recoverAtStart(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task, status string) {
	log.Info().Str("sandbox", record.Name).Str("task", owner.Name).Str("status", status).Msg("restarting a sandbox that went down without being stopped")

	err := api.Restart(ctx, record, owner)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot restart the sandbox after agc started")
		api.tell(ctx, owner, RecoveryFailed(owner.Name, record.Name, status, err))
		return
	}

	api.tell(ctx, owner, Recovered(owner.Name, record))
}

// Recovered is the line agc posts when a task's sandbox came back after a
// restart of agc or of the host.
func Recovered(taskName string, record *taskmodels.Sandbox) string {
	if record.Command == "" {
		return taskName + " came back after a restart; its sandbox " + record.Name + " is running again."
	}

	return taskName + " came back after a restart; its agent continues."
}

// RecoveryFailed is the line agc posts when a sandbox found down at
// startup cannot be started again.
func RecoveryFailed(taskName string, name string, status string, err error) string {
	return taskName + " did not come back after a restart: sandbox " + name + " is " + status + " and could not start: " + ez.ErrorMessage(err) + ". Start it with `agc sandbox start " + name + "` once fixed."
}

// tell posts body as agc in #<task> and, when the task has an owner, in
// #<owner>. Failures are logged.
func (api *API) tell(ctx context.Context, owner *taskmodels.Task, body string) {
	if api.msg == nil {
		return
	}

	channels := []string{messaging.TaskChannel(owner.Name)}
	if owner.Owner != "" {
		channels = append(channels, "#"+owner.Owner)
	}

	for _, channel := range channels {
		_, err := api.msg.PostAsSystem(ctx, channel, body)
		if err != nil {
			log.Warn().Err(err).Str("task", owner.Name).Str("channel", channel).Msg("cannot post the recovery")
		}
	}
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
