package usage

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

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

		token, ok := tokens.Lookup(a.Secret)
		if !ok || token == "" {
			p.store(a.Name, Result{Usage: UsageView{Error: "secret not found in secrets.env"}})
			failed++
			continue
		}

		result, err := p.Fetch(ctx, token)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			result = Result{Usage: UsageView{Error: "fetch failed"}}
		}

		if result.Usage.Error != "" {
			failed++
		}

		p.store(a.Name, result)
	}
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
