package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
)

// Persistent agents (ADR 0016) add the agents table; a sandbox can be an
// agent's home instead of a task's (sandboxes.agent); and participants and
// messages record the home of the agent that joined or posted
// (messaging_participants.agent, messaging_messages.agent). A fresh
// database already has all of it from the models.
func init() {
	Migrations.MustRegister(createAgents, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func createAgents(ctx context.Context, db *bun.DB) error {
	_, err := db.NewCreateTable().Model((*agent.Agent)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	columns := []string{"sandboxes", "messaging_participants", "messaging_messages"}
	for _, table := range columns {
		err = addColumn(ctx, db, table, "agent", "TEXT NOT NULL DEFAULT ''")
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
