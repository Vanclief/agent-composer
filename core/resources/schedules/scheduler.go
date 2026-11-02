package schedules

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

// Start runs the scheduler until ctx is canceled: at once and then every
// CheckInterval it ticks each enabled schedule whose next run is due, one
// goroutine per schedule. agc rest starts it; nothing else does.
func (api *API) Start(ctx context.Context) {
	log.Info().Dur("every", CheckInterval).Msg("schedules: scheduler started")

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		api.RunDue(ctx)

		select {
		case <-ctx.Done():
			api.Wait()
			log.Info().Msg("schedules: scheduler stopped")
			return
		case <-ticker.C:
		}
	}
}

// RunDue starts a tick for every enabled schedule that is due and returns
// without waiting for them. A due schedule whose previous tick still runs
// is skipped.
func (api *API) RunDue(ctx context.Context) {
	records, err := schedulemodels.List(ctx, api.db)
	if err != nil {
		log.Warn().Err(err).Msg("schedules: cannot list schedules")
		return
	}

	now := api.now()
	for i := range records {
		record := records[i]
		if !record.Enabled || record.NextRunAt().After(now) {
			continue
		}

		if api.isRunning(record.ID) {
			log.Info().Str("schedule", record.Name).Msg("schedules: due but its last tick still runs, skipped")
			continue
		}

		api.ticks.Add(1)
		go func() {
			defer api.ticks.Done()

			_, err := api.tick(ctx, record.ID, record.Name, false)
			if err != nil {
				log.Warn().Err(err).Str("schedule", record.Name).Msg("schedules: tick did not run")
			}
		}()
	}
}
