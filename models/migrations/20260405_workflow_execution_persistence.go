package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS workflow_executions (
				id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
				workflow_id TEXT NOT NULL,
				workflow_version TEXT NOT NULL,
				workflow_snapshot JSONB NOT NULL,
				input_snapshot JSONB,
				output_snapshot JSONB,
				status TEXT NOT NULL,
				shell_root TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				metadata JSONB,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS node_executions (
				id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
				workflow_execution_id UUID NOT NULL,
				parent_node_execution_id UUID,
				node_id TEXT NOT NULL,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				node_snapshot JSONB NOT NULL,
				input_snapshot JSONB,
				output_snapshot JSONB,
				trace JSONB,
				iteration_index INTEGER,
				branch_name TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ,
				finished_at TIMESTAMPTZ,
				metadata JSONB,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_node_executions_workflow_execution_id ON node_executions (workflow_execution_id);
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_node_executions_parent_node_execution_id ON node_executions (parent_node_execution_id);
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			ADD COLUMN IF NOT EXISTS node_execution_id UUID,
			ADD COLUMN IF NOT EXISTS input_snapshot JSONB,
			ADD COLUMN IF NOT EXISTS output_snapshot JSONB;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_conversations_node_execution_id ON conversations (node_execution_id);
		`)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			DROP INDEX IF EXISTS idx_conversations_node_execution_id;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			ALTER TABLE conversations
			DROP COLUMN IF EXISTS output_snapshot,
			DROP COLUMN IF EXISTS input_snapshot,
			DROP COLUMN IF EXISTS node_execution_id;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			DROP INDEX IF EXISTS idx_node_executions_parent_node_execution_id;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			DROP INDEX IF EXISTS idx_node_executions_workflow_execution_id;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			DROP TABLE IF EXISTS node_executions;
		`)
		if err != nil {
			return err
		}

		_, err = db.ExecContext(ctx, `
			DROP TABLE IF EXISTS workflow_executions;
		`)
		if err != nil {
			return err
		}

		return nil
	})
}
