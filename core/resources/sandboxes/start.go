package sandboxes

import (
	"context"
	"encoding/json"

	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/skills"
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

	harness, err := api.harnessOf(ctx, record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	bound, bindings, err := api.resolveSecrets(ctx, spec, harness)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// A sandbox allocated before its provider had a default gets it now,
	// recorded on the row so the account cannot be deleted under it.
	if len(bound.Secrets) != len(spec.Secrets) {
		encoded, err := json.Marshal(bound)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		record.Spec = encoded

		err = record.Update(ctx, api.db, "spec")
		if err != nil {
			return nil, ez.Wrap(err)
		}

		spec = bound
	}

	// A skill removed from the repository since the allocate is refused
	// before the VM boots, as on allocate.
	repo := api.skillsRepo()

	err = repo.Check(spec.Skills)
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

	// A start boots the disk but not the job, so the skills are written
	// again (an edit since the last boot reaches a wake) and the job runs
	// again here (ADR 0008 item 4, ADR 0016 item 4). Allocate discards a sandbox whose
	// job cannot start; a start must not remove a kept disk, so it fails
	// with the same error and stops the VM again, leaving it stopped.
	err = api.rewriteSkills(ctx, record, spec, repo)
	if err == nil {
		err = api.relaunchJob(ctx, record, spec)
	}
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

// rewriteSkills writes the spec's skills into the started sandbox again,
// as prepare does on the first boot (ADR 0008 "Skills").
func (api *API) rewriteSkills(ctx context.Context, record *taskmodels.Sandbox, spec task.Spec, repo skills.Repo) error {
	if len(spec.Skills) == 0 {
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

	return writeSkills(ctx, sb, repo, spec.Skills)
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
