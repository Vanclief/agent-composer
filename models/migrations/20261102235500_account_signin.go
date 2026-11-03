package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/vanclief/ez"
)

// Accounts sign in (ADR 0018 "Sign-in"): accounts gain signed_in_at, null
// for an account with no login, which every existing account is. A fresh
// database already has the column from the model.
func init() {
	Migrations.MustRegister(accountSignin, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func accountSignin(ctx context.Context, db *bun.DB) error {
	definition := "TIMESTAMP"
	if db.Dialect().Name() == dialect.PG {
		definition = "TIMESTAMPTZ"
	}

	err := addColumn(ctx, db, "accounts", "signed_in_at", definition)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
