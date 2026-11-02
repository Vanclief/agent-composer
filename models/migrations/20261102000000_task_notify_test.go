package migrations

import (
	"context"
	"testing"
	"time"
)

func TestTaskNotifyDefaultsTrue(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	_, err := db.ExecContext(ctx, `CREATE TABLE tasks (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE,
created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO tasks (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)", "0190a7c2-0000-7000-8000-000000000001", "stop-icon", now, now)
	if err != nil {
		t.Fatal(err)
	}

	err = taskNotify(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// A second run, as on a fresh database where the model made the column.
	err = taskNotify(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	var notify bool
	err = db.QueryRowContext(ctx, "SELECT notify FROM tasks WHERE name = 'stop-icon'").Scan(&notify)
	if err != nil {
		t.Fatal(err)
	}

	if !notify {
		t.Fatal("existing task does not notify")
	}
}
