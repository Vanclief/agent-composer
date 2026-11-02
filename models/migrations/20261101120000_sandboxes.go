package migrations

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/task"
)

// Tasks allocate compute (ADR 0008). Tasks gain brief, assignee, for and
// done_at; sandboxes get their own table; every task row gives its name and
// spec to one sandbox, booted when the task was created; then tasks.spec is
// dropped. A fresh database already has the new shape from the models, so
// each step looks before it changes anything.
func init() {
	Migrations.MustRegister(allocateSandboxes, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func allocateSandboxes(ctx context.Context, db *bun.DB) error {
	doneAtType := "TIMESTAMP"
	if db.Dialect().Name() == dialect.PG {
		doneAtType = "TIMESTAMPTZ"
	}

	columns := []struct {
		name       string
		definition string
	}{
		{"brief", "TEXT NOT NULL DEFAULT ''"},
		{"assignee", "TEXT NOT NULL DEFAULT ''"},
		{"for_id", "TEXT NOT NULL DEFAULT ''"},
		{"done_at", doneAtType},
	}

	for _, column := range columns {
		err := addColumn(ctx, db, "tasks", column.name, column.definition)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	_, err := db.NewCreateTable().Model((*task.Sandbox)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS sandboxes_task_id_idx ON sandboxes (task_id)")
	if err != nil {
		return ez.Wrap(err)
	}

	hasSpec, err := columnExists(ctx, db, "tasks", "spec")
	if err != nil {
		return ez.Wrap(err)
	}

	if !hasSpec {
		return nil
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return moveSpecsToSandboxes(ctx, tx)
	})
}

// moveSpecsToSandboxes gives every task one sandbox named after it, with
// its spec, booted_at = the task's created_at, then drops tasks.spec.
// Values are copied by the database, so no type round-trips through Go.
func moveSpecsToSandboxes(ctx context.Context, tx bun.Tx) error {
	var ids []string

	err := tx.NewRaw("SELECT id FROM tasks").Scan(ctx, &ids)
	if err != nil {
		return ez.Wrap(err)
	}

	for _, taskID := range ids {
		sandboxID, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		_, err = tx.ExecContext(ctx, `INSERT INTO sandboxes (id, task_id, name, spec, command, booted_at, created_at, updated_at)
SELECT ?, id, name, spec, '', created_at, created_at, updated_at FROM tasks
WHERE id = ? AND NOT EXISTS (SELECT 1 FROM sandboxes WHERE sandboxes.name = tasks.name)`, sandboxID.String(), taskID)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE tasks DROP COLUMN spec")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// addColumn adds column to table unless it is already there.
func addColumn(ctx context.Context, db *bun.DB, table, column, definition string) error {
	exists, err := columnExists(ctx, db, table, column)
	if err != nil {
		return ez.Wrap(err)
	}

	if exists {
		return nil
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
