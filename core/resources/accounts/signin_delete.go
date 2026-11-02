package accounts

import (
	"context"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/secrets"
)

// SignoutRequest signs an account out.
type SignoutRequest struct {
	Name string `json:"-"`
}

func (r SignoutRequest) Validate() error {
	return validateName(r.Name)
}

// Signout removes the account's login line from secrets.env, forgets its
// access token and pending sign-in, and clears signed_in_at; its usage is
// read again with the account's own token. Signing out an account with no
// login changes nothing.
func (api *API) Signout(ctx context.Context, requester interface{}, request *SignoutRequest) (*AccountView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := account.GetByName(ctx, api.db, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.removeLogin(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.forget(record.Name)
	api.pollNow(ctx, record.Name)

	return api.view(ctx, record), nil
}

// removeLogin clears signed_in_at and removes the login line under one
// transaction.
func (api *API) removeLogin(ctx context.Context, record *account.Account) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return ez.Wrap(err)
	}

	loginName := account.LoginSecretName(record.Name)
	previous := record.SignedInAt

	err = api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := record.SetSignedIn(ctx, tx, nil, "")
		if err != nil {
			return ez.Wrap(err)
		}

		// A line by that name that is another account's secret is not a
		// login and stays.
		if !ownsLine(ctx, tx, record, loginName) {
			return nil
		}

		err = store.Remove(loginName)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	})
	if err != nil {
		record.SignedInAt = previous
		return ez.Wrap(err)
	}

	return nil
}

// ownsLine reports whether the line called loginName is record's login:
// no other account keeps its value there.
func ownsLine(ctx context.Context, db bun.IDB, record *account.Account, loginName string) bool {
	records, err := account.List(ctx, db)
	if err != nil {
		return false
	}

	for _, other := range records {
		if other.ID != record.ID && other.Secret == loginName {
			return false
		}
	}

	return true
}
