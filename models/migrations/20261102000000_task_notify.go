package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Tasks gain notify: the panel shows a toast and plays a chime when the
// task starts and when it is done. Existing tasks notify.
func init() {
	Migrations.MustRegister(taskNotify, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func taskNotify(ctx context.Context, db *bun.DB) error {
	err := addColumn(ctx, db, "tasks", "notify", "BOOLEAN NOT NULL DEFAULT true")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
