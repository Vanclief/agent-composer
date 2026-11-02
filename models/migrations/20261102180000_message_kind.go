package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Messages gain kind, message or question; existing rows are messages.
// The reply_to index finds the replies that answer a question.
func init() {
	Migrations.MustRegister(messageKind, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func messageKind(ctx context.Context, db *bun.DB) error {
	err := addColumn(ctx, db, "messaging_messages", "kind", "TEXT NOT NULL DEFAULT 'message'")
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewCreateIndex().
		Model((*msgmodels.Message)(nil)).
		Index(msgmodels.ReplyToIndex).
		Column("reply_to").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
