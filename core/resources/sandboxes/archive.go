package sandboxes

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// archiveChannels archives #<task> of every task done longer than keep ago
// whose channel is not archived yet (ADR 0005 "Archived channels"), and
// answers the channels it archived. It rides the collector's tick; a
// channel that cannot be archived is logged and the rest go on.
func (api *API) archiveChannels(ctx context.Context, keep time.Duration) ([]string, error) {
	tasks, err := taskmodels.ListTasks(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	archived, err := msgmodels.ArchivedChannels(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	now := api.now()
	done := []string{}

	for i := range tasks {
		record := &tasks[i]
		if record.DoneAt == nil || now.Sub(*record.DoneAt) < keep {
			continue
		}

		channel := messaging.TaskChannel(record.Name)
		_, ok := archived[channel]
		if ok {
			continue
		}

		added, err := api.msg.Archive(ctx, channel, now)
		if err != nil {
			log.Warn().Err(err).Str("channel", channel).Str("task", record.Name).Msg("messaging: cannot archive channel")
			continue
		}

		if !added {
			continue
		}

		doneFor := now.Sub(*record.DoneAt).Round(time.Minute).String()
		log.Info().Str("channel", channel).Str("task", record.Name).Str("done_for", doneFor).Msg("messaging: archived channel")
		done = append(done, channel)
	}

	return done, nil
}
