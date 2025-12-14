package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ALTER COLUMN structured_output_schema TYPE JSON USING structured_output_schema::json;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			ALTER COLUMN structured_output_schema TYPE JSON USING structured_output_schema::json;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ALTER COLUMN structured_output_schema TYPE JSONB USING structured_output_schema::jsonb;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			ALTER COLUMN structured_output_schema TYPE JSONB USING structured_output_schema::jsonb;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
