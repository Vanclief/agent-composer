package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// A task is done when someone accepts it (ADR 0010). Tasks gain accept
// (a JSON array of criteria), delivered_at, summary and accepted_by.
// Existing tasks get no criteria, no delivery and an empty accepted_by.
func init() {
	Migrations.MustRegister(taskAcceptance, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func taskAcceptance(ctx context.Context, db *bun.DB) error {
	acceptType := "TEXT NOT NULL DEFAULT '[]'"
	deliveredAtType := "TIMESTAMP"
	if db.Dialect().Name() == dialect.PG {
		acceptType = "JSONB NOT NULL DEFAULT '[]'"
		deliveredAtType = "TIMESTAMPTZ"
	}

	columns := []struct {
		name       string
		definition string
	}{
		{"accept", acceptType},
		{"delivered_at", deliveredAtType},
		{"summary", "TEXT NOT NULL DEFAULT ''"},
		{"accepted_by", "TEXT NOT NULL DEFAULT ''"},
	}

	for _, column := range columns {
		err := addColumn(ctx, db, "tasks", column.name, column.definition)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
