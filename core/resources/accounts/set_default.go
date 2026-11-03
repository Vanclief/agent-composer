package accounts

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
)

type SetDefaultRequest struct {
	Name string `json:"-"`
}

func (r SetDefaultRequest) Validate() error {
	return validateName(r.Name)
}

// SetDefault makes the account its provider's default; the provider's
// previous default loses the flag, in the same transaction.
func (api *API) SetDefault(ctx context.Context, requester interface{}, request *SetDefaultRequest) (*AccountView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.setDefault(ctx, request.Name, true)
}

// setDefault sets or clears the account's default flag under a
// transaction.
func (api *API) setDefault(ctx context.Context, name string, on bool) (*AccountView, error) {
	api.mu.Lock()
	defer api.mu.Unlock()

	var record *account.Account

	err := api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		found, err := account.GetByName(ctx, tx, name)
		if err != nil {
			return ez.Wrap(err)
		}

		err = found.SetDefault(ctx, tx, on)
		if err != nil {
			return ez.Wrap(err)
		}

		record = found

		return nil
	})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record), nil
}
