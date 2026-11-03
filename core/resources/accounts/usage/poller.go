package usage

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/accounts/oauth"
	"github.com/vanclief/agent-composer/secrets"
)

// DefaultInterval is how often the poller reads every account.
const DefaultInterval = 10 * time.Minute

// Account is what the poller needs of an accounts row.
type Account struct {
	Name     string
	Provider string
	Kind     string
	// Secret is the name in secrets.env holding the token.
	Secret string
	// Login is true when the account signed in through agc: its usage is
	// read with the login's access token, not Secret's.
	Login bool
}

// Lister lists the accounts rows. The accounts package implements it; the
// usage package never imports accounts.
type Lister interface {
	ListAccounts(ctx context.Context) ([]Account, error)
}

// NoAccounts is a Lister with no accounts, used until the accounts API is
// wired in.
type NoAccounts struct{}

// ListAccounts returns no accounts.
func (NoAccounts) ListAccounts(ctx context.Context) ([]Account, error) {
	return nil, nil
}

// UsageSource is what GET /api/accounts reads: the latest usage and email
// of an account by name. Usage is nil for an account never read and for
// every account that is not anthropic oauth.
type UsageSource interface {
	Usage(account string) *UsageView
	Email(account string) string
}

// LoginSource hands out a signed-in account's access token, refreshing
// it when missing or about to expire (ADR 0018 "Sign-in"). The accounts
// API implements it. An error oauth.Expired accepts, or ENOTFOUND, means
// the login is gone.
type LoginSource interface {
	AccessToken(ctx context.Context, name string) (string, error)
}

// Tokens looks a secret's value up by name; *secrets.Store implements it.
type Tokens interface {
	Lookup(name string) (string, bool)
}

// FetchFunc reads one token's usage and profile.
type FetchFunc func(ctx context.Context, token string) (Result, error)

// Poller reads every anthropic oauth account's usage once at start and every
// interval after, one fetch at a time, and keeps the latest Result per
// account in memory.
type Poller struct {
	// Interval, Fetch, LoadTokens and Now are replaced in tests.
	Interval   time.Duration
	Fetch      FetchFunc
	LoadTokens func() (Tokens, error)
	Now        func() time.Time

	listerMu sync.Mutex
	lister   Lister
	logins   LoginSource

	// polling serializes rounds: a Poll waits for the one before it.
	polling sync.Mutex

	mu      sync.RWMutex
	results map[string]Result
}

var _ UsageSource = (*Poller)(nil)

// NewPoller returns a poller over lister (NoAccounts when nil) that reads
// tokens from the secrets file at secretsPath and fetches from the Anthropic
// OAuth API.
func NewPoller(lister Lister, secretsPath string) *Poller {
	client := &http.Client{Timeout: 30 * time.Second}

	return &Poller{
		Interval: DefaultInterval,
		Fetch: func(ctx context.Context, token string) (Result, error) {
			return Fetch(ctx, client, token)
		},
		LoadTokens: func() (Tokens, error) {
			return secrets.Load(secretsPath)
		},
		Now:     time.Now,
		lister:  lister,
		results: map[string]Result{},
	}
}

// SetLister replaces the account list, for the accounts API to wire itself
// in after the poller is built.
func (p *Poller) SetLister(lister Lister) {
	p.listerMu.Lock()
	defer p.listerMu.Unlock()

	p.lister = lister
}

// SetLogins makes the poller read signed-in accounts with their login's
// access token.
func (p *Poller) SetLogins(logins LoginSource) {
	p.listerMu.Lock()
	defer p.listerMu.Unlock()

	p.logins = logins
}

func (p *Poller) currentLogins() LoginSource {
	p.listerMu.Lock()
	defer p.listerMu.Unlock()

	return p.logins
}

func (p *Poller) currentLister() Lister {
	p.listerMu.Lock()
	defer p.listerMu.Unlock()

	if p.lister == nil {
		return NoAccounts{}
	}

	return p.lister
}

// Start polls once, then every Interval, until ctx is cancelled. It blocks.
func (p *Poller) Start(ctx context.Context) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		p.Poll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Poll reads every anthropic oauth account once, in turn, and forgets the
// readings of accounts no longer listed or no longer anthropic oauth.
func (p *Poller) Poll(ctx context.Context) {
	p.polling.Lock()
	defer p.polling.Unlock()

	accounts, err := p.currentLister().ListAccounts(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("accounts usage: listing accounts failed")
		return
	}

	var wanted []Account
	for _, a := range accounts {
		if Polled(a) {
			wanted = append(wanted, a)
		}
	}

	// One line per cycle, counts only: never a token or an email.
	failed := 0
	defer func() {
		log.Info().Int("accounts", len(wanted)).Int("errors", failed).Msg("accounts usage: polled")
	}()

	p.forgetExcept(wanted)

	if len(wanted) == 0 {
		return
	}

	tokens, err := p.LoadTokens()
	if err != nil {
		log.Warn().Err(err).Msg("accounts usage: reading the secrets file failed")
		for _, a := range wanted {
			p.store(a.Name, Result{Usage: UsageView{Error: "secrets file unreadable"}})
		}
		failed = len(wanted)
		return
	}

	for _, a := range wanted {
		if ctx.Err() != nil {
			return
		}

		ok := p.pollOne(ctx, a, tokens)
		if !ok {
			failed++
		}
	}
}

// PollAccount reads the account called name now, when it is listed and
// polled, and returns the email its profile last gave ("" when unknown).
func (p *Poller) PollAccount(ctx context.Context, name string) string {
	p.polling.Lock()
	defer p.polling.Unlock()

	accounts, err := p.currentLister().ListAccounts(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("accounts usage: listing accounts failed")
		return p.Email(name)
	}

	for _, a := range accounts {
		if a.Name != name || !Polled(a) {
			continue
		}

		tokens, err := p.LoadTokens()
		if err != nil {
			log.Warn().Err(err).Msg("accounts usage: reading the secrets file failed")
			p.store(a.Name, Result{Usage: UsageView{Error: "secrets file unreadable"}})
			return p.Email(name)
		}

		p.pollOne(ctx, a, tokens)
	}

	return p.Email(name)
}

// pollOne reads account a and stores the reading; false when the reading
// has an error. A signed-in account is read with its login's access
// token; any other with its secret, and its profile status tells an
// inference-only token from a rejected one.
func (p *Poller) pollOne(ctx context.Context, a Account, tokens Tokens) bool {
	logins := p.currentLogins()

	var token string
	if a.Login && logins != nil {
		access, err := logins.AccessToken(ctx, a.Name)
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			if oauth.Expired(err) || ez.ErrorCode(err) == ez.ENOTFOUND {
				p.storeError(a.Name, ErrSignInExpired)
				return false
			}
			p.store(a.Name, Result{Usage: UsageView{Error: "sign-in: refreshing the access token failed"}})
			return false
		}
		token = access
	} else {
		value, ok := tokens.Lookup(a.Secret)
		if !ok || value == "" {
			p.store(a.Name, Result{Usage: UsageView{Error: "secret not found in secrets.env"}})
			return false
		}
		token = value
	}

	result, err := p.Fetch(ctx, token)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		result = Result{Usage: UsageView{Error: "fetch failed"}}
	}

	if !a.Login {
		why := noLoginError(result.profileStatus)
		if why != "" {
			result.Usage.Error = why
		}
	}

	p.store(a.Name, result)

	return result.Usage.Error == ""
}

// noLoginError maps the profile status of an account with no login, the
// one answer that tells a setup-token from a dead one (see the fetcher's
// comment): 403 is inference-only whatever usage said, 401 is a rejected
// token, anything else keeps the fetched why ("").
func noLoginError(profileStatus int) string {
	switch profileStatus {
	case http.StatusForbidden:
		return ErrInferenceOnly
	case http.StatusUnauthorized:
		return ErrTokenRejected
	}

	return ""
}

// storeError sets why as name's usage error and changes nothing else of
// its last reading.
func (p *Poller) storeError(name, why string) {
	p.mu.Lock()
	previous, ok := p.results[name]
	p.mu.Unlock()

	if !ok {
		p.store(name, Result{Usage: UsageView{Error: why}})
		return
	}

	previous.Usage.Error = why
	p.store(name, previous)
}

// Polled reports whether the poller reads account: anthropic oauth only.
func Polled(account Account) bool {
	return account.Provider == "anthropic" && account.Kind == "oauth"
}

// store keeps result as name's latest reading, stamping updated_at, and
// keeps the last known email when this reading has none.
func (p *Poller) store(name string, result Result) {
	if result.Usage.UpdatedAt.IsZero() {
		result.Usage.UpdatedAt = p.Now().UTC()
	}

	if result.Usage.Windows == nil {
		result.Usage.Windows = []Window{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if result.Email == "" {
		result.Email = p.results[name].Email
	}

	p.results[name] = result
}

func (p *Poller) forgetExcept(accounts []Account) {
	keep := map[string]bool{}
	for _, a := range accounts {
		keep[a.Name] = true
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for name := range p.results {
		if !keep[name] {
			delete(p.results, name)
		}
	}
}

// Usage returns a copy of account's latest usage, nil when it was never
// read (and always for accounts that are not anthropic oauth).
func (p *Poller) Usage(account string) *UsageView {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result, ok := p.results[account]
	if !ok {
		return nil
	}

	view := result.Usage
	view.Windows = append([]Window{}, result.Usage.Windows...)

	return &view
}

// Email returns the email account's profile last gave, "" when unknown.
func (p *Poller) Email(account string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.results[account].Email
}
