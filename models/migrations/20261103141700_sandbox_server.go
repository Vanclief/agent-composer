package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Sandboxes gain server: the agc server whose msb runs the VM (ADR 0018).
// Empty is the local server, so existing rows need no backfill.
func init() {
	Migrations.MustRegister(sandboxServer, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func sandboxServer(ctx context.Context, db *bun.DB) error {
	err := addColumn(ctx, db, "sandboxes", "server", "TEXT NOT NULL DEFAULT ''")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
