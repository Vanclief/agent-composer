package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/task"
)

func TestSandboxServerAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The sandboxes table as ADR 0016 left it. The new column lands last;
	// nothing reads columns by position.
	_, err := db.ExecContext(ctx, `CREATE TABLE sandboxes (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, agent TEXT NOT NULL DEFAULT '',
name TEXT NOT NULL UNIQUE, spec TEXT, command TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '',
volume TEXT NOT NULL DEFAULT '', boot_error TEXT, booted_at TIMESTAMP, stopped_at TIMESTAMP,
created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 3, 14, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO sandboxes (id, task_id, name, spec, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000002", "0190a7c2-0000-7000-8000-000000000001", "old-box", "{}", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = sandboxServer(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := task.GetSandboxByName(ctx, db, "old-box")
	if err != nil {
		t.Fatal(err)
	}
	if record.Server != "" {
		t.Fatalf("server = %q, want empty (local)", record.Server)
	}
}
