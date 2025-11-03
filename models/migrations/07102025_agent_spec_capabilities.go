package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ADD COLUMN shell_access BOOLEAN NOT NULL DEFAULT TRUE,
			ADD COLUMN web_search BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN structured_output BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN structured_output_schema JSONB;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			DROP COLUMN structured_output_schema,
			DROP COLUMN structured_output,
			DROP COLUMN web_search,
			DROP COLUMN shell_access;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
