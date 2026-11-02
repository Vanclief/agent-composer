package migrations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/models/task"
)

func newSQLite(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	return bun.NewDB(sqldb, sqlitedialect.New())
}

func TestAllocateSandboxesMovesSpecs(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The tasks table as the model created it before ADR 0008.
	_, err := db.ExecContext(ctx, `CREATE TABLE tasks (
id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL UNIQUE,
spec BLOB, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	created := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO tasks VALUES (?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000001", "Stop icon", "stop-icon", `{"repo":"github.com/o/n"}`, created, created)
	if err != nil {
		t.Fatal(err)
	}

	err = allocateSandboxes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	hasSpec, err := columnExists(ctx, db, "tasks", "spec")
	if err != nil || hasSpec {
		t.Fatalf("tasks.spec still there: %v, err %v", hasSpec, err)
	}

	// The later migrations run before the models read the rows.
	_, err = db.NewCreateTable().Model((*msgmodels.Message)(nil)).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = taskAgents(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	err = taskAcceptance(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	record, err := task.GetTaskByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Brief != "" || record.DoneAt != nil {
		t.Fatalf("task = %+v", record)
	}

	sandbox, err := task.GetSandboxByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.TaskID != record.ID || string(sandbox.Spec) != `{"repo":"github.com/o/n"}` {
		t.Fatalf("sandbox = %+v", sandbox)
	}
	if sandbox.BootedAt == nil || !sandbox.BootedAt.Equal(created) || sandbox.BootError != nil {
		t.Fatalf("booted_at = %v, boot_error = %v", sandbox.BootedAt, sandbox.BootError)
	}

	// Running it again, as on a fresh database, changes nothing.
	err = allocateSandboxes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	sandboxes, err := task.ListSandboxes(ctx, db)
	if err != nil || len(sandboxes) != 1 {
		t.Fatalf("sandboxes = %+v, err %v", sandboxes, err)
	}
}
