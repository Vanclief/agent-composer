package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/messaging"
)

// Archived channels (ADR 0005 "Archived channels") add channel_archives,
// one row per archived channel, unique on channel. A fresh database
// already has it from the model.
func init() {
	Migrations.MustRegister(createChannelArchives, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func createChannelArchives(ctx context.Context, db *bun.DB) error {
	_, err := db.NewCreateTable().Model((*messaging.ChannelArchive)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
