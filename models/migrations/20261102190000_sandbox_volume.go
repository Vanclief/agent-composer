package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Sandboxes gain volume: the named volume holding their harness homes,
// kept after the sandbox is removed (ADR 0008, "Disk and transcripts").
// Existing sandboxes booted without one and keep an empty volume.
func init() {
	Migrations.MustRegister(sandboxVolume, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func sandboxVolume(ctx context.Context, db *bun.DB) error {
	err := addColumn(ctx, db, "sandboxes", "volume", "TEXT NOT NULL DEFAULT ''")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
