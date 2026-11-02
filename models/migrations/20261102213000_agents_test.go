package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	"github.com/vanclief/agent-composer/models/task"
)

func TestCreateAgentsAddsTableAndColumns(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t)

	// The tables as ADR 0012 left them.
	statements := []string{
		`CREATE TABLE sandboxes (id TEXT PRIMARY KEY, task_id TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
spec TEXT, command TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '', volume TEXT NOT NULL DEFAULT '', boot_error TEXT,
booted_at TIMESTAMP, stopped_at TIMESTAMP, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL)`,
		`CREATE TABLE messaging_participants (id TEXT PRIMARY KEY, name TEXT NOT NULL, task TEXT NOT NULL, human BOOLEAN NOT NULL,
joined_at TIMESTAMP NOT NULL, left_at TIMESTAMP, active_name TEXT UNIQUE)`,
		`CREATE TABLE messaging_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, channel TEXT NOT NULL, "from" TEXT NOT NULL,
participant_id TEXT NOT NULL, task TEXT NOT NULL, human BOOLEAN NOT NULL DEFAULT false, reply_to INTEGER, body TEXT NOT NULL,
kind TEXT NOT NULL DEFAULT 'message', created_at TIMESTAMP NOT NULL)`,
	}
	for _, statement := range statements {
		_, err := db.ExecContext(ctx, statement)
		if err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 11, 2, 21, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(ctx, "INSERT INTO sandboxes (id, task_id, name, spec, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"0190a7c2-0000-7000-8000-000000000002", "0190a7c2-0000-7000-8000-000000000001", "stop-icon", "{}", now, now)
	if err != nil {
		t.Fatal(err)
	}

	// Twice: the second run, as on a fresh database, changes nothing.
	for range 2 {
		err = createAgents(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := task.GetSandboxByName(ctx, db, "stop-icon")
	if err != nil {
		t.Fatal(err)
	}
	if record.Agent != "" {
		t.Fatalf("agent = %q, want empty", record.Agent)
	}

	p := &msgmodels.Participant{Name: "reviewer-1", Agent: "reviewer-1"}
	err = p.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	m := &msgmodels.Message{Channel: "#franco", From: p.Name, ParticipantID: p.ID, Agent: p.Agent, Body: "hi"}
	err = m.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	a := &agent.Agent{Name: "reviewer-1", Role: "reviewer", Number: 1, Series: "reviewer", Harness: agent.HarnessClaudeCode}
	err = a.Insert(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	got, err := agent.GetAgentByName(ctx, db, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 1 || got.Kept || got.Home != "" {
		t.Fatalf("agent = %+v, want reviewer-1 numbered 1, not kept, no home", got)
	}
}
