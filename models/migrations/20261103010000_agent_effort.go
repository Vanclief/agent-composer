package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Agents carry a reasoning effort (ADR 0016 item 2): agents gain effort,
// empty for the harness's default, which every existing agent keeps. A
// fresh database already has the column from the model.
func init() {
	Migrations.MustRegister(agentEffort, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func agentEffort(ctx context.Context, db *bun.DB) error {
	err := addColumn(ctx, db, "agents", "effort", "TEXT NOT NULL DEFAULT ''")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
