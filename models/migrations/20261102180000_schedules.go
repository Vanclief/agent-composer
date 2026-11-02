package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/schedule"
)

// Scheduled tasks (ADR 0011) add two tables: schedules, and
// schedule_items with one row per key a schedule has seen, unique on
// (schedule_id, key). A fresh database already has both from the models.
func init() {
	Migrations.MustRegister(createSchedules, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func createSchedules(ctx context.Context, db *bun.DB) error {
	_, err := db.NewCreateTable().Model((*schedule.Schedule)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewCreateTable().Model((*schedule.Item)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
