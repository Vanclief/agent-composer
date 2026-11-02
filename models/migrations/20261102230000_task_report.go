package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// Tasks gain report: the last delivery's structured report, a JSON object
// next to summary (ADR 0010, "Delivery report"). Existing tasks have none.
func init() {
	Migrations.MustRegister(taskReport, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func taskReport(ctx context.Context, db *bun.DB) error {
	reportType := "TEXT"
	if db.Dialect().Name() == dialect.PG {
		reportType = "JSONB"
	}

	err := addColumn(ctx, db, "tasks", "report", reportType)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
