package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/task"
)

func TestTaskAcceptanceAddsColumns(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The tasks table as ADR 0009 left it.
	_, err := db.ExecContext(ctx, `CREATE TABLE tasks (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL UNIQUE,
brief TEXT NOT NULL DEFAULT '', assignee TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
manager TEXT NOT NULL DEFAULT '', owner TEXT NOT NULL DEFAULT '', for_id TEXT NOT NULL DEFAULT '',
done_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO tasks (id, title, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000001", "Stop icon", "stop-icon", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// The earlier task notify migration, so the model can read the row.
	err = taskNotify(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = taskAcceptance(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	// The later report column, so the current model can read the row.
	err = taskReport(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	record, err := task.GetTaskByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Accept == nil || len(record.Accept) != 0 || record.DeliveredAt != nil || record.Summary != "" || record.AcceptedBy != "" {
		t.Fatalf("task = %+v", record)
	}
}
