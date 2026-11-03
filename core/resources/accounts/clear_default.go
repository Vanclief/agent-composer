package accounts

import (
	"context"

	"github.com/vanclief/ez"
)

type ClearDefaultRequest struct {
	Name string `json:"-"`
}

func (r ClearDefaultRequest) Validate() error {
	return validateName(r.Name)
}

// ClearDefault takes the default flag off the account, leaving its
// provider with no default: sandboxes get no credential from it.
func (api *API) ClearDefault(ctx context.Context, requester interface{}, request *ClearDefaultRequest) (*AccountView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.setDefault(ctx, request.Name, false)
}
