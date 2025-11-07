package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN auto_compact BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN compact_at_percent INTEGER NOT NULL DEFAULT 90,
			ADD COLUMN compaction_prompt TEXT NOT NULL DEFAULT '',
			ADD COLUMN compact_count INTEGER NOT NULL DEFAULT 0,
			ADD COLUMN shell_access BOOLEAN NOT NULL DEFAULT TRUE,
			ADD COLUMN web_search BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN structured_output BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN structured_output_schema JSONB;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			UPDATE conversations c
			SET auto_compact = s.auto_compact,
				compact_at_percent = s.compact_at_percent,
				compaction_prompt = s.compaction_prompt,
				shell_access = s.shell_access,
				web_search = s.web_search,
				structured_output = s.structured_output,
				structured_output_schema = s.structured_output_schema
			FROM agent_specs s
			WHERE c.agent_spec_id = s.id;
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN structured_output_schema,
			DROP COLUMN structured_output,
			DROP COLUMN web_search,
			DROP COLUMN shell_access,
			DROP COLUMN compact_count,
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
