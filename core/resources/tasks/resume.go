package tasks

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// Resume starts the messaging listener of every running Task, for agc
// starting while Tasks kept running. A Task that fails is logged and
// skipped.
func (api *API) Resume(ctx context.Context) error {
	records, err := taskmodels.ListTasks(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	for i := range records {
		name := records[i].Name

		status, err := api.rt.Status(ctx, name)
		if err != nil {
			log.Warn().Err(err).Str("task", name).Msg("cannot read task status")
			continue
		}

		if status != statusRunning {
			continue
		}

		spec, err := decodeSpec(&records[i])
		if err != nil {
			log.Warn().Err(err).Str("task", name).Msg("cannot resume task messaging")
			continue
		}

		err = api.startTaskListener(name, spec)
		if err != nil {
			log.Warn().Err(err).Str("task", name).Msg("cannot resume task messaging")
		}
	}

	return nil
}
