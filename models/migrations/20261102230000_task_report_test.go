package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/task"
)

func TestTaskReportAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The tasks table as ADR 0010 left it.
	_, err := db.ExecContext(ctx, `CREATE TABLE tasks (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL UNIQUE,
brief TEXT NOT NULL DEFAULT '', assignee TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
manager TEXT NOT NULL DEFAULT '', owner TEXT NOT NULL DEFAULT '', for_id TEXT NOT NULL DEFAULT '',
notify BOOLEAN NOT NULL DEFAULT TRUE, accept TEXT NOT NULL DEFAULT '[]', delivered_at TIMESTAMP,
summary TEXT NOT NULL DEFAULT '', accepted_by TEXT NOT NULL DEFAULT '',
done_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO tasks (id, title, name, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000001", "Stop icon", "stop-icon", "Filled the icon.", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = taskReport(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := task.GetTaskByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Report != nil || record.Summary != "Filled the icon." {
		t.Fatalf("task = %+v", record)
	}
}
