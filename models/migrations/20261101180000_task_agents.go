package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// DefaultOwner is the configured task owner (defaults.owner or
// AGC_DEFAULT_OWNER) when migrations run. The controller sets it first;
// empty means none is configured.
var DefaultOwner string

// Every task has an agent and a manager (ADR 0009). Tasks gain model,
// manager and owner; sandboxes gain instructions; messages gain human.
// Existing tasks get the configured owner as manager and owner, or stay
// empty when none is configured. Existing messages copy human from their
// participant.
func init() {
	Migrations.MustRegister(taskAgents, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func taskAgents(ctx context.Context, db *bun.DB) error {
	hadManager, err := columnExists(ctx, db, "tasks", "manager")
	if err != nil {
		return ez.Wrap(err)
	}

	hadHuman, err := columnExists(ctx, db, "messaging_messages", "human")
	if err != nil {
		return ez.Wrap(err)
	}

	columns := []struct {
		table      string
		name       string
		definition string
	}{
		{"tasks", "model", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "manager", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "owner", "TEXT NOT NULL DEFAULT ''"},
		{"sandboxes", "instructions", "TEXT NOT NULL DEFAULT ''"},
		{"messaging_messages", "human", "BOOLEAN NOT NULL DEFAULT false"},
	}

	for _, column := range columns {
		err = addColumn(ctx, db, column.table, column.name, column.definition)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if !hadManager && DefaultOwner != "" {
		_, err = db.ExecContext(ctx, "UPDATE tasks SET manager = ?, owner = ? WHERE manager = '' AND owner = ''", DefaultOwner, DefaultOwner)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if !hadHuman {
		_, err = db.ExecContext(ctx, `UPDATE messaging_messages SET human = (
SELECT p.human FROM messaging_participants p WHERE p.id = messaging_messages.participant_id)
WHERE EXISTS (SELECT 1 FROM messaging_participants p WHERE p.id = messaging_messages.participant_id)`)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
