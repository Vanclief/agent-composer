package accounts

import (
	"context"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/accounts/usage"
	"github.com/vanclief/agent-composer/models/account"
)

var _ usage.Lister = (*API)(nil)

// ListAccounts lists every account for the usage poller, which reads the
// anthropic oauth ones (usage.Lister).
func (api *API) ListAccounts(ctx context.Context) ([]usage.Account, error) {
	records, err := account.List(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	accounts := []usage.Account{}
	for _, record := range records {
		accounts = append(accounts, usage.Account{
			Name:     record.Name,
			Provider: string(record.Provider),
			Kind:     string(record.Kind),
			Secret:   record.Secret,
			Login:    record.SignedInAt != nil,
		})
	}

	return accounts, nil
}

// PollerUsage is the usage poller's latest readings as a UsageSource.
type PollerUsage struct {
	Poller *usage.Poller
}

var _ UsageSource = PollerUsage{}

// Usage returns account's latest reading, nil until the poller has read it.
func (p PollerUsage) Usage(ctx context.Context, name string) (*UsageView, error) {
	reading := p.Poller.Usage(name)
	if reading == nil {
		return nil, nil
	}

	view := &UsageView{UpdatedAt: reading.UpdatedAt, Windows: []UsageWindow{}, Error: reading.Error}
	for _, w := range reading.Windows {
		view.Windows = append(view.Windows, UsageWindow{ID: w.ID, Label: w.Label, Used: w.Used, ResetsAt: resetsAt(w.ResetsAt)})
	}

	return view, nil
}

var _ AccountPoller = PollerUsage{}

// PollAccount reads account name's usage now and returns its email.
func (p PollerUsage) PollAccount(ctx context.Context, name string) string {
	return p.Poller.PollAccount(ctx, name)
}

// resetsAt is the poller's RFC 3339 reset time, nil when it gave none or
// one that does not parse.
func resetsAt(value string) *time.Time {
	if value == "" {
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}

	return &parsed
}
