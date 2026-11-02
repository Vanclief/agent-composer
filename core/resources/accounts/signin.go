package accounts

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/accounts/oauth"
	"github.com/vanclief/agent-composer/core/resources/accounts/usage"
	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/secrets"
)

// refreshBefore is how long before its expiry an access token is
// refreshed.
const refreshBefore = 5 * time.Minute

var _ usage.LoginSource = (*API)(nil)

// SigninRequest begins an account's sign-in (ADR 0018 "Sign-in").
type SigninRequest struct {
	Name string `json:"-"`
}

func (r SigninRequest) Validate() error {
	return validateName(r.Name)
}

// SigninResponse is the page to open, and when the sign-in lapses.
type SigninResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Signin begins a sign-in of an anthropic oauth account: a random state
// and a PKCE verifier, kept in memory for ten minutes, one per account (a
// new one replaces the last), and the authorize URL to open.
func (api *API) Signin(ctx context.Context, requester interface{}, request *SigninRequest) (*SigninResponse, error) {
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

	api.loginMu.Lock()
	defer api.loginMu.Unlock()

	now := time.Now()
	for name, pending := range api.pending {
		if now.After(pending.ExpiresAt) {
			delete(api.pending, name)
		}
	}

	pending, link := api.oauth.Begin()
	api.pending[record.Name] = pending

	return &SigninResponse{URL: link, ExpiresAt: pending.ExpiresAt.UTC().Truncate(time.Second)}, nil
}

// signsIn refuses an account that cannot sign in: only anthropic oauth
// accounts have a login.
func signsIn(record *account.Account) error {
	if record.Provider != account.ProviderAnthropic || record.Kind != account.KindOAuth {
		return ez.New(ez.EINVALID, "only anthropic oauth accounts sign in: account "+record.Name+" is "+string(record.Provider)+" "+string(record.Kind), nil)
	}

	return nil
}

// AccessToken returns the signed-in account's access token, refreshing
// it from the refresh token in secrets.env when it is missing or expires
// within five minutes, and writing back a rotated refresh token
// (usage.LoginSource). ENOTFOUND when the account has no login line.
func (api *API) AccessToken(ctx context.Context, name string) (string, error) {
	api.loginMu.Lock()
	defer api.loginMu.Unlock()

	token, ok := api.access[name]
	if ok && time.Until(token.ExpiresAt) > refreshBefore {
		return token.Access, nil
	}

	loginName := account.LoginSecretName(name)

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return "", ez.Wrap(err)
	}

	refresh, ok := store.Lookup(loginName)
	if !ok || refresh == "" {
		delete(api.access, name)
		return "", ez.New(ez.ENOTFOUND, "account "+name+" is not signed in", nil)
	}

	fresh, err := oauth.Refresh(ctx, api.client, api.oauth, refresh)
	if err != nil {
		delete(api.access, name)
		return "", ez.Wrap(err)
	}

	api.access[name] = fresh

	if fresh.Refresh != refresh {
		api.rotate(loginName, fresh.Refresh)
	}

	return fresh.Access, nil
}

// rotate writes a rotated refresh token over the login line, unless the
// account signed out meanwhile. A failed write is logged, by name only.
func (api *API) rotate(loginName, refresh string) {
	api.mu.Lock()
	defer api.mu.Unlock()

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		log.Error().Err(err).Str("secret", loginName).Msg("accounts: cannot write a rotated sign-in refresh token")
		return
	}

	if !store.Has(loginName) {
		return
	}

	err = store.Set(loginName, refresh)
	if err != nil {
		log.Error().Err(err).Str("secret", loginName).Msg("accounts: cannot write a rotated sign-in refresh token")
	}
}

// forget drops the account's pending sign-in and access token.
func (api *API) forget(name string) {
	api.loginMu.Lock()
	delete(api.pending, name)
	delete(api.access, name)
	api.loginMu.Unlock()
}

// pollNow reads the account's usage at once when a poller is set, and
// returns the email its profile gave.
func (api *API) pollNow(ctx context.Context, name string) string {
	api.usageMu.Lock()
	poller := api.poller
	api.usageMu.Unlock()

	if poller == nil {
		return ""
	}

	return poller.PollAccount(ctx, name)
}
