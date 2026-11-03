// Package account holds accounts: a credential for one provider, kept as a
// line of secrets.env and described by a row here (ADR 0018). The value
// never reaches the database.
package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
)

// Provider is who issued the credential.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// Providers lists every provider, in the order the API shows them.
var Providers = []Provider{ProviderAnthropic, ProviderOpenAI}

// Kind is what the credential is: a subscription token or an API key.
type Kind string

const (
	KindOAuth  Kind = "oauth"
	KindAPIKey Kind = "api_key"
)

// OneDefaultIndex is the partial unique index on provider where is_default
// holds: at most one default account per provider.
const OneDefaultIndex = "accounts_one_default_per_provider"

// MaxNameBytes bounds an account's name.
const MaxNameBytes = 32

var nameRegex = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// Account is a credential for one provider. Secret is the NAME of the
// secrets.env line holding its value, ACCOUNT_<NAME>.
type Account struct {
	bun.BaseModel `bun:"table:accounts"`

	ID       uuid.UUID `bun:",pk,type:uuid" json:"id"`
	Name     string    `bun:"name,notnull,unique" json:"name"`
	Provider Provider  `bun:"provider,notnull" json:"provider"`
	Kind     Kind      `bun:"kind,notnull" json:"kind"`
	Secret   string    `bun:"secret,notnull" json:"secret"`
	Email    string    `bun:"email,notnull,default:''" json:"email"`
	// Default marks the provider's default account, bound to a sandbox
	// whose spec names no credential of its harness.
	Default bool `bun:"is_default,notnull,default:false" json:"default"`
	// SignedInAt is when the account last signed in through agc (ADR 0018
	// "Sign-in"), nil when it has no login: its refresh token is then not
	// in secrets.env.
	SignedInAt *time.Time `bun:"signed_in_at" json:"signed_in_at"`
	CreatedAt  time.Time  `bun:"created_at,notnull" json:"created_at"`
	UpdatedAt  time.Time  `bun:"updated_at,notnull" json:"updated_at"`
}

// LoginSecretName is the secrets.env name holding the account's sign-in
// refresh token: its secret name and _LOGIN.
func LoginSecretName(name string) string {
	return SecretName(name) + "_LOGIN"
}

// ValidateName checks an account's name: lowercase letters, digits and
// dashes, 1 to 32 of them.
func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		msg := fmt.Sprintf("account name %q must be 1 to %d lowercase letters, digits and dashes", name, MaxNameBytes)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

// SecretName is the secrets.env name holding the account's value:
// ACCOUNT_ and the name upper-cased, dashes as underscores.
func SecretName(name string) string {
	return "ACCOUNT_" + strings.ReplaceAll(strings.ToUpper(name), "-", "_")
}

// Validate checks the provider.
func (p Provider) Validate() error {
	for _, known := range Providers {
		if p == known {
			return nil
		}
	}

	return ez.New(ez.EINVALID, fmt.Sprintf("unknown provider %q: use anthropic or openai", string(p)), nil)
}

// Harnesses are the harnesses an account of the provider serves.
func (p Provider) Harnesses() []agent.Harness {
	switch p {
	case ProviderAnthropic:
		return []agent.Harness{agent.HarnessClaudeCode}
	case ProviderOpenAI:
		return []agent.Harness{agent.HarnessCodexCLI}
	}

	return []agent.Harness{}
}

// Variable is the guest variable an account of kind k of the provider is
// handed to, empty when the provider has no such kind.
func (p Provider) Variable(k Kind) string {
	switch {
	case p == ProviderAnthropic && k == KindOAuth:
		return "CLAUDE_CODE_OAUTH_TOKEN"
	case p == ProviderAnthropic && k == KindAPIKey:
		return "ANTHROPIC_API_KEY"
	case p == ProviderOpenAI && k == KindAPIKey:
		return "CODEX_API_KEY"
	}

	return ""
}

// ProviderOf is the provider whose accounts serve harness h; found is
// false for a harness no provider serves (pi).
func ProviderOf(h agent.Harness) (Provider, bool) {
	for _, p := range Providers {
		for _, served := range p.Harnesses() {
			if served == h {
				return p, true
			}
		}
	}

	return "", false
}

// ValidateKind checks k is a kind the provider has.
func ValidateKind(p Provider, k Kind) error {
	if k != KindOAuth && k != KindAPIKey {
		return ez.New(ez.EINVALID, fmt.Sprintf("unknown kind %q: use oauth or api_key", string(k)), nil)
	}

	if p.Variable(k) == "" {
		return ez.New(ez.EINVALID, fmt.Sprintf("provider %s has no %s accounts yet: use api_key", p, k), nil)
	}

	return nil
}

func (a *Account) Validate() error {
	err := ValidateName(a.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	err = a.Provider.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	err = ValidateKind(a.Provider, a.Kind)
	if err != nil {
		return ez.Wrap(err)
	}

	if a.Secret != SecretName(a.Name) {
		return ez.New(ez.EINVALID, "account "+a.Name+" must keep its value in "+SecretName(a.Name), nil)
	}

	return nil
}

func (a *Account) Insert(ctx context.Context, db bun.IDB) error {
	if a.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		a.ID = id
	}

	at := now()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = at
	}
	a.UpdatedAt = at

	err := a.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(a).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// SetDefault makes a the only default account of its provider, or, with
// on false, leaves the provider with no default when a was it. Run it in
// a transaction.
func (a *Account) SetDefault(ctx context.Context, db bun.IDB, on bool) error {
	at := now()

	if on {
		_, err := db.NewUpdate().Model((*Account)(nil)).
			Set("is_default = ?", false).Set("updated_at = ?", at).
			Where("provider = ?", a.Provider).Where("is_default = ?", true).Where("id <> ?", a.ID).
			Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	a.Default = on
	a.UpdatedAt = at

	_, err := db.NewUpdate().Model(a).Column("is_default", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// SetSignedIn records a sign-in at at, or none with nil, and sets the
// email when email is not empty.
func (a *Account) SetSignedIn(ctx context.Context, db bun.IDB, at *time.Time, email string) error {
	a.SignedInAt = at
	if email != "" {
		a.Email = email
	}
	a.UpdatedAt = now()

	_, err := db.NewUpdate().Model(a).Column("signed_in_at", "email", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// SetEmail sets the account's email.
func (a *Account) SetEmail(ctx context.Context, db bun.IDB, email string) error {
	a.Email = email
	a.UpdatedAt = now()

	_, err := db.NewUpdate().Model(a).Column("email", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (a *Account) Delete(ctx context.Context, db bun.IDB) error {
	if a.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(a).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// GetByName returns the account called name.
func GetByName(ctx context.Context, db bun.IDB, name string) (*Account, error) {
	record := new(Account)

	err := db.NewSelect().Model(record).Where("name = ?", name).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, "account "+name+" not found", err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// GetDefault returns the provider's default account, nil when it has
// none.
func GetDefault(ctx context.Context, db bun.IDB, provider Provider) (*Account, error) {
	record := new(Account)

	err := db.NewSelect().Model(record).Where("provider = ?", provider).Where("is_default = ?", true).
		Order("updated_at DESC").Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// List returns every account by provider, then name.
func List(ctx context.Context, db bun.IDB) ([]Account, error) {
	records := []Account{}

	err := db.NewSelect().Model(&records).Order("provider ASC", "name ASC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return records, nil
}

// now is the current time as the database keeps it, to the microsecond.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
