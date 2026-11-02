package agents

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
)

type ListRequest struct{}

func (r ListRequest) Validate() error {
	return nil
}

type ListResponse struct {
	Agents []AgentView `json:"agents"`
}

// List returns every agent, oldest first, each with its state read from
// its home now.
func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	records, err := agent.ListAgents(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []AgentView{}
	for i := range records {
		view, err := api.view(ctx, &records[i])
		if err != nil {
			return nil, ez.Wrap(err)
		}

		views = append(views, *view)
	}

	return &ListResponse{Agents: views}, nil
}
