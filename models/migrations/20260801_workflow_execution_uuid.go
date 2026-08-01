package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// Workflows now carry a permanent uuid alongside their renameable
// slug. Executions record it so history survives slug renames.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE workflow_executions
			ADD COLUMN IF NOT EXISTS workflow_uuid UUID;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_uuid
			ON workflow_executions (workflow_uuid);
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE workflow_executions
			DROP COLUMN IF EXISTS workflow_uuid;
		`)
		return err
	})
}
