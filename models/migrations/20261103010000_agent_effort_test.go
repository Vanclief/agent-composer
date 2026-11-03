package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/agent"
)

func TestAgentEffortAddsColumn(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The agents table as the agents migration first made it.
	_, err := db.ExecContext(ctx, `CREATE TABLE agents (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, role TEXT NOT NULL,
number INTEGER NOT NULL, series TEXT NOT NULL, kept BOOLEAN NOT NULL DEFAULT false, harness TEXT NOT NULL,
model TEXT NOT NULL DEFAULT '', owner TEXT NOT NULL DEFAULT '', home TEXT NOT NULL DEFAULT '', idle TEXT NOT NULL DEFAULT '',
created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL, last_active_at TIMESTAMP)`)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, "INSERT INTO agents (id, name, role, number, series, harness, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000001", "reviewer-1", "reviewer", 1, "reviewer", "claude_code", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = agentEffort(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := agent.GetAgentByName(ctx, db, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Effort != "" {
		t.Fatalf("effort = %q, want empty", record.Effort)
	}

	record.Effort = "high"
	err = record.Update(ctx, db, "effort")
	if err != nil {
		t.Fatal(err)
	}

	record, err = agent.GetAgentByName(ctx, db, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Effort != "high" {
		t.Fatalf("effort = %q, want high", record.Effort)
	}
}
