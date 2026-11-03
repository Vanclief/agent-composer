package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// Messages carry turn_for (ADR 0016 item 6): messaging_messages gains
// turn_for, a JSON list of the persistent agents a message is a turn for,
// null for every existing message. A fresh database already has it.
func init() {
	Migrations.MustRegister(messageTurnFor, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func messageTurnFor(ctx context.Context, db *bun.DB) error {
	turnForType := "TEXT"
	if db.Dialect().Name() == dialect.PG {
		turnForType = "JSONB"
	}

	err := addColumn(ctx, db, "messaging_messages", "turn_for", turnForType)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
