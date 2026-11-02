package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/task"
)

func TestSandboxVolumeAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The sandboxes table as ADR 0010 left it.
	_, err := db.ExecContext(ctx, `CREATE TABLE sandboxes (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
spec TEXT, command TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '', boot_error TEXT,
booted_at TIMESTAMP, stopped_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO sandboxes (id, task_id, name, spec, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000002", "0190a7c2-0000-7000-8000-000000000001", "stop-icon", "{}", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = sandboxVolume(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := task.GetSandboxByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Volume != "" {
		t.Fatalf("volume = %q, want empty", record.Volume)
	}
}
