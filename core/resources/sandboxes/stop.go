package sandboxes

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type StopRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r StopRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Stop shuts the VM down and keeps its disk, then stops the listener.
func (api *API) Stop(ctx context.Context, requester interface{}, request *StopRequest) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, owner, err := api.findWithTask(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = requireBooted(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.stop(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record, owner)
}

// stop shuts the VM down, stops the listener and sets stopped_at: what
// Stop does once the sandbox is known to be booted.
func (api *API) stop(ctx context.Context, record *taskmodels.Sandbox) error {
	err := api.rt.Stop(ctx, record.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	api.stopListener(record.Name)

	now := time.Now().UTC()
	record.StoppedAt = &now

	err = record.Update(ctx, api.db, "stopped_at")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// stopOnce is stop for the task's acceptance, which may reach the same
// sandbox twice (StopTask and the end of its boot): the second finds
// stopped_at set and does nothing.
func (api *API) stopOnce(ctx context.Context, id uuid.UUID) error {
	api.stopping.Lock()
	defer api.stopping.Unlock()

	record, err := taskmodels.GetSandboxByID(ctx, api.db, id)
	if err != nil {
		return ez.Wrap(err)
	}

	if record.StoppedAt != nil {
		return nil
	}

	return api.stop(ctx, record)
}

// StopTask stops every running sandbox of an accepted task the way Stop
// does (ADR 0010 item 4). A sandbox still booting is left to its boot,
// which stops it when it finishes because the task is done
// (stopIfTaskDone); stopping it mid-boot would fail the boot and discard
// its disk. Failures are logged, not returned.
func (api *API) StopTask(ctx context.Context, owner *taskmodels.Task) {
	records, err := taskmodels.ListTaskSandboxes(ctx, api.db, owner.ID)
	if err != nil {
		log.Warn().Err(err).Str("task", owner.Name).Msg("cannot list the accepted task's sandboxes to stop them")
		return
	}

	for i := range records {
		record := &records[i]

		if api.isBooting(record.Name) {
			log.Info().Str("task", owner.Name).Str("sandbox", record.Name).Msg("the accepted task's sandbox is booting, it stops once booted")
			continue
		}

		view, err := api.view(ctx, record, owner)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot read the accepted task's sandbox")
			continue
		}

		if view.Status != StatusRunning {
			continue
		}

		err = api.stopOnce(ctx, record.ID)
		if err != nil {
			log.Warn().Err(err).Str("task", owner.Name).Str("sandbox", record.Name).Msg("cannot stop the accepted task's sandbox")
		}
	}
}

// stopIfTaskDone stops a sandbox that has just booted when its task was
// accepted while it booted, so an accepted task never keeps a VM running.
func (api *API) stopIfTaskDone(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) {
	current, err := taskmodels.GetTaskByID(ctx, api.db, owner.ID)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot read the task after the boot")
		return
	}

	if current.DoneAt == nil {
		return
	}

	err = api.stopOnce(ctx, record.ID)
	if err != nil {
		log.Warn().Err(err).Str("task", owner.Name).Str("sandbox", record.Name).Msg("cannot stop the accepted task's sandbox after its boot")
		return
	}

	log.Info().Str("task", owner.Name).Str("sandbox", record.Name).Msg("stopped the sandbox after its boot: the task was accepted")
}

// requireBooted refuses runtime operations on a sandbox that is still
// starting or failed to boot.
func requireBooted(record *taskmodels.Sandbox) error {
	if record.BootError != nil {
		return ez.New(ez.ECONFLICT, "sandbox "+record.Name+" failed to boot: "+*record.BootError, nil)
	}

	if record.BootedAt == nil {
		return ez.New(ez.ECONFLICT, "sandbox "+record.Name+" is still starting", nil)
	}

	return nil
}
