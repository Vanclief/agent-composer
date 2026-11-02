package sandboxes

import (
	"context"

	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

type StartRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r StartRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Start boots a stopped sandbox from its disk and relaunches its job.
// Secrets are read again from the store, since their values may have
// changed.
func (api *API) Start(ctx context.Context, requester interface{}, request *StartRequest) (*SandboxView, error) {
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

	spec, err := decodeSpec(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	bindings, err := api.resolveSecrets(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.startSandboxListener(record, owner, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.rt.Start(ctx, record.Name, task.SandboxSecrets(bindings))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// A start boots the disk but not the job, so the job runs again here
	// (ADR 0008 item 4, ADR 0016 item 4). Allocate discards a sandbox whose
	// job cannot start; a start must not remove a kept disk, so it fails
	// with the same error and stops the VM again, leaving it stopped.
	err = api.relaunchJob(ctx, record, spec)
	if err != nil {
		api.stopListener(record.Name)
		stopErr := api.rt.Stop(context.WithoutCancel(ctx), record.Name)
		if stopErr != nil {
			log.Warn().Err(stopErr).Str("sandbox", record.Name).Msg("cannot stop sandbox after its job failed to start")
		}
		return nil, ez.Wrap(err)
	}

	record.StoppedAt = nil

	err = record.Update(ctx, api.db, "stopped_at")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view, err := api.view(ctx, record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	return view, nil
}

// relaunchJob starts the sandbox's command as allocate does on the first
// boot, unless a running terminal already runs it.
func (api *API) relaunchJob(ctx context.Context, record *taskmodels.Sandbox, spec task.Spec) error {
	if record.Command == "" {
		return nil
	}

	sb, err := api.rt.Connect(ctx, record.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	defer func() {
		detachErr := sb.Detach(context.WithoutCancel(ctx))
		if detachErr != nil {
			log.Warn().Err(detachErr).Str("sandbox", record.Name).Msg("cannot release sandbox handle")
		}
	}()

	terminals, err := sb.ListTerminals(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	fields := strings.Fields(record.Command)
	for _, terminal := range terminals {
		if terminal.Running && slices.Equal(terminal.Command, fields) {
			return nil
		}
	}

	_, err = sb.StartTerminal(ctx, jobCommand(record, spec))
	if err != nil {
		return jobError(record, err)
	}

	return nil
}
