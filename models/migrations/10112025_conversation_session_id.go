package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_conversations_session_id ON conversations (session_id);
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			DROP INDEX IF EXISTS idx_conversations_session_id;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN session_id;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
