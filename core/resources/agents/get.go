package agents

import (
	"context"

	"github.com/vanclief/ez"
)

// GetRequest names an agent; the name comes from the path.
type GetRequest struct {
	Name string `json:"-"`
}

func (r GetRequest) Validate() error {
	if r.Name == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*AgentView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}
