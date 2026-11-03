package accounts

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/account"
)

type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

// ListResponse is every account and every provider.
type ListResponse struct {
	Accounts  []AccountView  `json:"accounts"`
	Providers []ProviderView `json:"providers"`
}

// List returns the accounts by provider, then name, and the providers.
func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	records, err := account.List(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []AccountView{}
	for i := range records {
		views = append(views, *api.view(ctx, &records[i]))
	}

	return &ListResponse{Accounts: views, Providers: providerViews()}, nil
}
