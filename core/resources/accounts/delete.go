package accounts

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/secrets"
)

type DeleteRequest struct {
	Name string `json:"-"`
}

func (r DeleteRequest) Validate() error {
	return validateName(r.Name)
}

// Delete removes the account's row and its secrets.env line. It is
// refused with ECONFLICT while a running or booting sandbox's spec names
// its secret.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*struct{}, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.mu.Lock()
	defer api.mu.Unlock()

	record, err := account.GetByName(ctx, api.db, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if api.users != nil {
		names, err := api.users.SandboxesUsingSecret(ctx, record.Secret)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if len(names) > 0 {
			msg := "account " + record.Name + " is in use by running sandbox " + strings.Join(names, ", ") + ": stop it first"
			return nil, ez.New(ez.ECONFLICT, msg, nil)
		}
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// The line goes inside the transaction, so a failed removal keeps the
	// row.
	err = api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := record.Delete(ctx, tx)
		if err != nil {
			return ez.Wrap(err)
		}

		err = store.Remove(record.Secret)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return nil, nil
}
