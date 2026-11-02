package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/task"
)

func TestTaskAgentsFillsManagerOwnerAndHuman(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The tables as ADR 0008 left them.
	statements := []string{
		`CREATE TABLE tasks (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', name TEXT NOT NULL UNIQUE,
brief TEXT NOT NULL DEFAULT '', assignee TEXT NOT NULL DEFAULT '', for_id TEXT NOT NULL DEFAULT '',
done_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE sandboxes (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, name TEXT NOT NULL UNIQUE, spec BLOB,
command TEXT NOT NULL DEFAULT '', boot_error TEXT, booted_at TIMESTAMP, stopped_at TIMESTAMP,
created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE messaging_participants (id TEXT PRIMARY KEY, name TEXT NOT NULL, task TEXT NOT NULL,
human BOOLEAN NOT NULL, joined_at TIMESTAMP NOT NULL, left_at TIMESTAMP, active_name TEXT UNIQUE)`,
		`CREATE TABLE messaging_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL, "from" TEXT NOT NULL,
participant_id TEXT NOT NULL, task TEXT NOT NULL, reply_to INTEGER, body TEXT NOT NULL, created_at TIMESTAMP NOT NULL)`,
	}
	for _, statement := range statements {
		_, err := db.ExecContext(ctx, statement)
		if err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	inserts := []struct {
		query string
		args  []interface{}
	}{
		{"INSERT INTO tasks (id, title, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", []interface{}{"0190a7c2-0000-7000-8000-000000000001", "Stop icon", "stop-icon", now, now}},
		{"INSERT INTO messaging_participants VALUES (?, ?, ?, ?, ?, NULL, ?)", []interface{}{"p1", "franco", "", true, now, "franco"}},
		{"INSERT INTO messaging_participants VALUES (?, ?, ?, ?, ?, NULL, ?)", []interface{}{"p2", "reviewer", "stop-icon", false, now, "reviewer"}},
		{`INSERT INTO messaging_messages (channel, "from", participant_id, task, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`, []interface{}{"#stop-icon", "franco", "p1", "", "hi", now}},
		{`INSERT INTO messaging_messages (channel, "from", participant_id, task, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`, []interface{}{"#stop-icon", "reviewer", "p2", "stop-icon", "hello", now}},
	}
	for _, insert := range inserts {
		_, err := db.ExecContext(ctx, insert.query, insert.args...)
		if err != nil {
			t.Fatal(err)
		}
	}

	DefaultOwner = "franco"
	t.Cleanup(func() {
		DefaultOwner = ""
	})

	err := taskAgents(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	// The later migrations run before the model reads the rows.
	err = taskNotify(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	err = taskAcceptance(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	err = taskReport(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	record, err := task.GetTaskByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Manager != "franco" || record.Owner != "franco" || record.Model != "" {
		t.Fatalf("task = %+v", record)
	}

	var humans []bool
	err = db.NewRaw("SELECT human FROM messaging_messages ORDER BY id").Scan(ctx, &humans)
	if err != nil {
		t.Fatal(err)
	}
	if len(humans) != 2 || !humans[0] || humans[1] {
		t.Fatalf("human = %v", humans)
	}

	hasInstructions, err := columnExists(ctx, db, "sandboxes", "instructions")
	if err != nil || !hasInstructions {
		t.Fatalf("sandboxes.instructions: %v, err %v", hasInstructions, err)
	}

	// Running it again, as on a fresh database, changes nothing.
	DefaultOwner = "ana"

	err = taskAgents(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	record, err = task.GetTaskByName(ctx, db, "stop-icon")
	if err != nil || record.Manager != "franco" {
		t.Fatalf("task = %+v, err %v", record, err)
	}
}
