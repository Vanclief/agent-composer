// Package accounts manages accounts: a credential for one provider, kept
// as a line of secrets.env and described by a row, and the default
// account per provider a sandbox gets when its spec names no credential
// (docs/decisions/0018-accounts.md). No route returns a value, and no
// error or log line holds one.
package accounts

import (
	"context"
	"sync"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/models/account"
	"github.com/vanclief/agent-composer/models/agent"
)

// UsageSource reads an account's usage by account name. PollerUsage
// serves the usage poller's readings; until a source is set every
// account's usage is null.
type UsageSource interface {
	Usage(ctx context.Context, name string) (*UsageView, error)
}

// SecretUsers reports the sandboxes still running (or booting) whose spec
// names a secret, so an account is not deleted from under them. The
// sandboxes API is one.
type SecretUsers interface {
	SandboxesUsingSecret(ctx context.Context, secretName string) ([]string, error)
}

type API struct {
	db          bun.IDB
	secretsPath string
	users       SecretUsers

	// mu serializes the writes to secrets.env and the rows behind them.
	mu sync.Mutex

	// usageMu guards usage, apart from mu: views are built while mu is
	// held.
	usageMu sync.Mutex
	usage   UsageSource
}

// NewAPI returns the accounts API over the secrets file at secretsPath.
func NewAPI(ctrl *controller.Controller, secretsPath string, users SecretUsers) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.DB, secretsPath, users)
}

// New is NewAPI over a bare database, for tests.
func New(db bun.IDB, secretsPath string, users SecretUsers) *API {
	return &API{db: db, secretsPath: secretsPath, users: users}
}

// SetUsage fills anthropic oauth accounts' usage from source from now on.
func (api *API) SetUsage(source UsageSource) {
	api.usageMu.Lock()
	api.usage = source
	api.usageMu.Unlock()
}

// AccountView is an account as the API shows it: never its value.
type AccountView struct {
	Name      string           `json:"name"`
	Provider  account.Provider `json:"provider"`
	Kind      account.Kind     `json:"kind"`
	Secret    string           `json:"secret"`
	Email     string           `json:"email"`
	Default   bool             `json:"default"`
	CreatedAt time.Time        `json:"created_at"`
	// Usage is filled for anthropic oauth accounts once a UsageSource is
	// set, else null.
	Usage *UsageView `json:"usage"`
}

// UsageView is an account's usage windows as its provider reports them.
type UsageView struct {
	UpdatedAt time.Time     `json:"updated_at"`
	Windows   []UsageWindow `json:"windows"`
	// Error says why usage could not be read, empty when it was.
	Error string `json:"error"`
}

// UsageWindow is one limit: session, weekly or a model's slug.
type UsageWindow struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Used is the share used, 0 to 1.
	Used     float64    `json:"used"`
	ResetsAt *time.Time `json:"resets_at"`
}

// ProviderView is a provider, the harnesses its accounts serve and the
// variable each kind of account is handed to.
type ProviderView struct {
	ID        account.Provider `json:"id"`
	Harnesses []agent.Harness  `json:"harnesses"`
	Variables ProviderVars     `json:"variables"`
}

// ProviderVars is the guest variable per kind; OAuth is omitted for a
// provider with no oauth kind.
type ProviderVars struct {
	OAuth  string `json:"oauth,omitempty"`
	APIKey string `json:"api_key"`
}

// providerViews lists every provider.
func providerViews() []ProviderView {
	views := []ProviderView{}
	for _, p := range account.Providers {
		views = append(views, ProviderView{
			ID:        p,
			Harnesses: p.Harnesses(),
			Variables: ProviderVars{OAuth: p.Variable(account.KindOAuth), APIKey: p.Variable(account.KindAPIKey)},
		})
	}

	return views
}

// view is the account's view, with its usage when a source is set and
// the account is an anthropic oauth one. A usage error is shown on the
// view, not returned.
func (api *API) view(ctx context.Context, record *account.Account) *AccountView {
	view := &AccountView{
		Name:      record.Name,
		Provider:  record.Provider,
		Kind:      record.Kind,
		Secret:    record.Secret,
		Email:     record.Email,
		Default:   record.Default,
		CreatedAt: record.CreatedAt,
	}

	api.usageMu.Lock()
	source := api.usage
	api.usageMu.Unlock()

	if source == nil || record.Provider != account.ProviderAnthropic || record.Kind != account.KindOAuth {
		return view
	}

	usage, err := source.Usage(ctx, record.Name)
	if err != nil {
		usage = &UsageView{UpdatedAt: time.Now().UTC(), Windows: []UsageWindow{}, Error: ez.ErrorMessage(err)}
	}

	view.Usage = usage

	return view
}

// validateName checks a name from the path.
func validateName(name string) error {
	if name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}
