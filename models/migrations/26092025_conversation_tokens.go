package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN cached_tokens INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN cost INTEGER NOT NULL DEFAULT 0;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN cost,
			DROP COLUMN cached_tokens,
			DROP COLUMN output_tokens,
			DROP COLUMN input_tokens;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
