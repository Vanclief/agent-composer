package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/accounts/oauth"
	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/secrets"
)

// errNothingPending is the answer when no sign-in waits for a code.
const errNothingPending = "no sign-in pending for this account; start again"

// SigninCompleteRequest finishes a sign-in with what the page showed:
// `code#state`, or the bare code. Code is never returned, logged or
// stored.
type SigninCompleteRequest struct {
	Name string `json:"-"`
	Code string `json:"code"`
}

// String hides the code, should the request ever be printed.
func (r SigninCompleteRequest) String() string {
	return "{name:" + r.Name + "}"
}

// MarshalJSON leaves the code out: the REST error path logs a failed
// request's body as JSON.
func (r SigninCompleteRequest) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: r.Name})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return data, nil
}

func (r SigninCompleteRequest) Validate() error {
	err := validateName(r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if strings.TrimSpace(r.Code) == "" {
		return ez.New(ez.EINVALID, "code is required: paste the code shown after signing in", nil)
	}

	return nil
}

// SigninComplete exchanges the pasted code for tokens, writes the refresh
// token to secrets.env as ACCOUNT_<NAME>_LOGIN, keeps the access token in
// memory, sets signed_in_at, reads the account's usage at once and sets
// its email from the profile when it had none.
func (api *API) SigninComplete(ctx context.Context, requester interface{}, request *SigninCompleteRequest) (*AccountView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := account.GetByName(ctx, api.db, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = signsIn(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	token, err := api.exchange(ctx, record.Name, request.Code)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.saveLogin(ctx, record, token.Refresh)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.loginMu.Lock()
	api.access[record.Name] = token
	api.loginMu.Unlock()

	email := api.pollNow(ctx, record.Name)
	if record.Email == "" && email != "" {
		err = record.SetEmail(ctx, api.db, email)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return api.view(ctx, record), nil
}

// exchange checks the pasted code against the account's pending sign-in
// and trades it for tokens. The pending sign-in is dropped once the
// exchange succeeds; a failed one can be retried until it lapses.
func (api *API) exchange(ctx context.Context, name, pasted string) (oauth.Token, error) {
	code, state, hasState := strings.Cut(strings.TrimSpace(pasted), "#")

	api.loginMu.Lock()
	pending, ok := api.pending[name]
	cfg := api.oauth
	client := api.client
	api.loginMu.Unlock()

	if !ok || time.Now().After(pending.ExpiresAt) {
		return oauth.Token{}, ez.New(ez.EINVALID, errNothingPending, nil)
	}

	if hasState && state != pending.State {
		return oauth.Token{}, ez.New(ez.EINVALID, "the code is from another sign-in than the one pending for this account; start again", nil)
	}

	if code == "" {
		return oauth.Token{}, ez.New(ez.EINVALID, "code is required: paste the code shown after signing in", nil)
	}

	token, err := oauth.Exchange(ctx, client, cfg, code, pending.State, pending.Verifier)
	if err != nil {
		status := oauth.Status(err)
		if status != 0 {
			msg := fmt.Sprintf("sign-in failed: the token endpoint answered HTTP %d; start again", status)
			return oauth.Token{}, ez.New(ez.EINVALID, msg, nil)
		}
		return oauth.Token{}, ez.Wrap(err)
	}

	api.loginMu.Lock()
	current, ok := api.pending[name]
	if ok && current.State == pending.State {
		delete(api.pending, name)
	}
	api.loginMu.Unlock()

	return token, nil
}

// saveLogin writes the refresh token as the account's login line and sets
// signed_in_at, under one transaction. The line's name must not be
// another account's secret.
func (api *API) saveLogin(ctx context.Context, record *account.Account, refresh string) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	loginName := account.LoginSecretName(record.Name)

	records, err := account.List(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	for _, other := range records {
		if other.Secret == loginName {
			return ez.New(ez.ECONFLICT, "account "+other.Name+" keeps its value in "+loginName+", where account "+record.Name+" keeps its sign-in: rename one of them", nil)
		}
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return ez.Wrap(err)
	}

	at := time.Now().UTC().Truncate(time.Second)
	previous := record.SignedInAt

	written := false
	err = api.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := record.SetSignedIn(ctx, tx, &at, "")
		if err != nil {
			return ez.Wrap(err)
		}

		err = store.Set(loginName, refresh)
		if err != nil {
			return ez.Wrap(err)
		}

		written = true

		return nil
	})
	if err != nil {
		if written {
			removeErr := store.Remove(loginName)
			if removeErr != nil {
				log.Error().Err(removeErr).Str("secret", loginName).Msg("cannot remove the sign-in line of an account whose row was not saved")
			}
		}
		record.SignedInAt = previous
		return ez.Wrap(err)
	}

	return nil
}
