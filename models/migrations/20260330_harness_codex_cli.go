package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ADD COLUMN IF NOT EXISTS harness TEXT NOT NULL DEFAULT 'codex_cli',
			ADD COLUMN IF NOT EXISTS harness_config JSONB NOT NULL DEFAULT '{}'::jsonb;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN IF NOT EXISTS harness TEXT NOT NULL DEFAULT 'codex_cli',
			ADD COLUMN IF NOT EXISTS harness_config JSONB NOT NULL DEFAULT '{}'::jsonb,
			ADD COLUMN IF NOT EXISTS harness_session_ref TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS harness_state JSONB,
			ADD COLUMN IF NOT EXISTS raw_harness_output TEXT NOT NULL DEFAULT '',
			ADD COLUMN IF NOT EXISTS harness_exit_code INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS harness_error TEXT NOT NULL DEFAULT '';
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			DROP COLUMN IF EXISTS provider;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN IF EXISTS provider;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'open_ai';
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'open_ai';
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE agent_specs
			DROP COLUMN IF EXISTS harness,
			DROP COLUMN IF EXISTS harness_config;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN IF EXISTS harness,
			DROP COLUMN IF EXISTS harness_config,
			DROP COLUMN IF EXISTS harness_session_ref,
			DROP COLUMN IF EXISTS harness_state,
			DROP COLUMN IF EXISTS raw_harness_output,
			DROP COLUMN IF EXISTS harness_exit_code,
			DROP COLUMN IF EXISTS harness_error;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
