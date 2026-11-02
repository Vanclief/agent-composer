package sandboxes

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// StatusCrashed is msb's word for a VM that died without being stopped,
// what every running VM reports after the host reboots (ADR 0017).
const StatusCrashed = "crashed"

// Recoverable reports whether a sandbox the runtime reports as status
// should be started again on its own (ADR 0017): it booted, agc never
// stopped it (no stopped_at), its task is not done, and the runtime says
// it is anything but running. A sandbox agc stopped, or whose task was
// accepted, stays as it is.
func Recoverable(record *taskmodels.Sandbox, owner *taskmodels.Task, status string) bool {
	if record.BootedAt == nil || record.BootError != nil || record.StoppedAt != nil {
		return false
	}

	if owner == nil || owner.Name == "" || owner.DoneAt != nil {
		return false
	}

	return status != "" && status != StatusRunning
}

// Restart starts a sandbox that went down without agc stopping it, the way
// Start does (secrets resolved again, listener attached, runtime started,
// stopped_at cleared), then starts its agent job again when no job with
// its command runs: msb does not restart jobs when it boots a VM. It is
// what recovery after a reboot and the watchdog's restarts call (ADR 0017).
func (api *API) Restart(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) error {
	err := requireBooted(record)
	if err != nil {
		return ez.Wrap(err)
	}

	ok := api.beginRestart(record.Name)
	if !ok {
		return ez.New(ez.ECONFLICT, "sandbox "+record.Name+" is already restarting", nil)
	}
	defer api.endRestart(record.Name)

	spec, err := decodeSpec(record)
	if err != nil {
		return ez.Wrap(err)
	}

	bindings, err := api.resolveSecrets(spec)
	if err != nil {
		return ez.Wrap(err)
	}

	err = api.startSandboxListener(record.Name, owner, spec)
	if err != nil {
		return ez.Wrap(err)
	}

	err = api.rt.Start(ctx, record.Name, task.SandboxSecrets(bindings))
	if err != nil {
		return ez.Wrap(err)
	}

	if record.StoppedAt != nil {
		record.StoppedAt = nil

		err = record.Update(ctx, api.db, "stopped_at")
		if err != nil {
			return ez.Wrap(err)
		}
	}

	err = api.startAgentJob(ctx, record, spec)
	if err != nil {
		return ez.Wrap(err)
	}

	api.publishCurrent(ctx, record, owner)

	return nil
}

// startAgentJob starts the sandbox's command in a terminal, as its boot
// did, unless the runtime already runs a job with that command.
func (api *API) startAgentJob(ctx context.Context, record *taskmodels.Sandbox, spec task.Spec) error {
	fields := strings.Fields(record.Command)
	if len(fields) == 0 {
		return nil
	}

	sb, err := api.rt.Connect(ctx, record.Name)
	if err != nil {
		return ez.Wrap(err)
	}
	defer func() {
		err := sb.Detach(context.Background())
		if err != nil {
			log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot release sandbox handle")
		}
	}()

	watcher, ok := sb.(executor.JobWatcher)
	if ok {
		jobs, err := watcher.ListJobs(ctx)
		if err != nil {
			return ez.Wrap(err)
		}

		job := newestWithCommand(jobs, fields)
		if job != nil && job.Running {
			return nil
		}
	}

	_, err = sb.StartTerminal(ctx, executor.Command{Name: fields[0], Args: fields[1:], Dir: workDir(spec)})
	if err != nil {
		return ez.New(ez.ErrorCode(err), "cannot start "+record.Command+": "+ez.ErrorMessage(err), err)
	}

	return nil
}

// beginRestart marks name as restarting; false when it already is.
func (api *API) beginRestart(name string) bool {
	api.mu.Lock()
	defer api.mu.Unlock()

	if api.restarting == nil {
		api.restarting = map[string]bool{}
	}

	if api.restarting[name] {
		return false
	}

	api.restarting[name] = true

	return true
}

func (api *API) endRestart(name string) {
	api.mu.Lock()
	defer api.mu.Unlock()

	delete(api.restarting, name)
}
