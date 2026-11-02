package sandboxes

import (
	"context"

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

	taskNames, err := api.taskNames(ctx)
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

		err = api.startSandboxListener(name, taskNames[record.TaskID.String()], spec)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", name).Msg("cannot resume sandbox messaging")
		}
	}

	return nil
}

// taskNames maps every task id to its name.
func (api *API) taskNames(ctx context.Context) (map[string]string, error) {
	tasks, err := taskmodels.ListTasks(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	names := map[string]string{}
	for _, record := range tasks {
		names[record.ID.String()] = record.Name
	}

	return names, nil
}
