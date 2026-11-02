package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/secrets"
)

// maxEmailBytes bounds an account's email.
const maxEmailBytes = 254

// CreateRequest adds an account. Value is written to secrets.env and
// never returned, logged or stored in the database.
type CreateRequest struct {
	Name     string           `json:"name"`
	Provider account.Provider `json:"provider"`
	Kind     account.Kind     `json:"kind"`
	Value    string           `json:"value"`
	Email    string           `json:"email"`
}

// String hides the value, should the request ever be printed.
func (r CreateRequest) String() string {
	return fmt.Sprintf("{name:%s provider:%s kind:%s email:%s}", r.Name, r.Provider, r.Kind, r.Email)
}

// loggedRequest is CreateRequest without its value.
type loggedRequest struct {
	Name     string           `json:"name"`
	Provider account.Provider `json:"provider"`
	Kind     account.Kind     `json:"kind"`
	Email    string           `json:"email"`
}

// MarshalJSON leaves the value out: the REST error path logs a failed
// request's body as JSON. Binding a request reads JSON, so value still
// comes in.
func (r CreateRequest) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(loggedRequest{Name: r.Name, Provider: r.Provider, Kind: r.Kind, Email: r.Email})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return data, nil
}

func (r CreateRequest) Validate() error {
	err := account.ValidateName(strings.TrimSpace(r.Name))
	if err != nil {
		return ez.Wrap(err)
	}

	err = r.Provider.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	err = account.ValidateKind(r.Provider, r.Kind)
	if err != nil {
		return ez.Wrap(err)
	}

	if strings.TrimSpace(r.Value) == "" {
		return ez.New(ez.EINVALID, "value is required: the token or key the account signs in with", nil)
	}

	email := strings.TrimSpace(r.Email)
	if len(email) > maxEmailBytes || strings.IndexFunc(email, unicode.IsSpace) >= 0 || strings.IndexFunc(email, unicode.IsControl) >= 0 {
		return ez.New(ez.EINVALID, fmt.Sprintf("email must be at most %d characters, without spaces", maxEmailBytes), nil)
	}

	return nil
}

// Create writes ACCOUNT_<NAME>=<value> to secrets.env (0600, created when
// missing) and inserts the row, not as the default. The secret name must
// be free in secrets.env and the account name unused.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*AccountView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.mu.Lock()
	defer api.mu.Unlock()

	name := strings.TrimSpace(request.Name)
	record := &account.Account{
		Name:     name,
		Provider: request.Provider,
		Kind:     request.Kind,
		Secret:   account.SecretName(name),
		Email:    strings.TrimSpace(request.Email),
	}

	_, err = account.GetByName(ctx, api.db, name)
	if err == nil {
		return nil, ez.New(ez.ECONFLICT, "account "+name+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if store.Has(record.Secret) {
		return nil, ez.New(ez.ECONFLICT, "secret "+record.Secret+" is already set in "+api.secretsPath+": remove it there or pick another account name", nil)
	}

	// The row goes in first, inside a transaction that only commits once
	// the line is written, so a failed write leaves no row, and a failed
	// commit takes the line out again.
	written := false
	err = api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := record.Insert(ctx, tx)
		if err != nil {
			return ez.Wrap(err)
		}

		err = store.Set(record.Secret, strings.TrimSpace(request.Value))
		if err != nil {
			return ez.Wrap(err)
		}

		written = true

		return nil
	})
	if err != nil {
		if written {
			removeErr := store.Remove(record.Secret)
			if removeErr != nil {
				log.Error().Err(removeErr).Str("secret", record.Secret).Msg("cannot remove the secrets.env line of an account whose row was not saved")
			}
		}
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record), nil
}
