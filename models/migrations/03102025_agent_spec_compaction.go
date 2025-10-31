package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ADD COLUMN auto_compact BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN compact_at_percent INTEGER NOT NULL DEFAULT 90,
			ADD COLUMN compaction_prompt TEXT NOT NULL DEFAULT '';
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			DROP COLUMN compaction_prompt,
			DROP COLUMN compact_at_percent,
			DROP COLUMN auto_compact;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
