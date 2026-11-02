package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
)

// Accounts (ADR 0018) add the accounts table, unique on name, and a
// partial unique index on provider where is_default holds, so a provider
// has at most one default account. A fresh database already has the table
// from the model, but not the partial index, which only this adds.
func init() {
	Migrations.MustRegister(createAccounts, func(ctx context.Context, db *bun.DB) error {
		// Never rolled back: we do not roll back schema.
		return nil
	})
}

func createAccounts(ctx context.Context, db *bun.DB) error {
	_, err := db.NewCreateTable().Model((*account.Account)(nil)).IfNotExists().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewCreateIndex().
		Model((*account.Account)(nil)).
		Unique().
		Index(account.OneDefaultIndex).
		Column("provider").
		Where("is_default = ?", true).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
