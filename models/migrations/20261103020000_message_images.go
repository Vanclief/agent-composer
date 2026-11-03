package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// Messages carry images (ADR 0005 "Images"): messaging_messages gains
// images, a JSON list of {name, type, size}, null for the text-only
// messages every existing one is. A fresh database already has it.
func init() {
	Migrations.MustRegister(messageImages, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func messageImages(ctx context.Context, db *bun.DB) error {
	imagesType := "TEXT"
	if db.Dialect().Name() == dialect.PG {
		imagesType = "JSONB"
	}

	err := addColumn(ctx, db, "messaging_messages", "images", imagesType)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
