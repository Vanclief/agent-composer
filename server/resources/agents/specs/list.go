package specs

import (
	"context"

	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/provider"
)

type ListRequest struct {
	pagination.CursorRequest

	// Optional filters
	Provider *provider.LLMProvider `json:"provider,omitempty"`
	Search   string                `json:"search"`
}

func (r *ListRequest) Validate() error {
	const op = "agents.ListRequest.Validate"

	err := r.CursorRequest.Validate()
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	// if err := validation.ValidateStruct(r); err != nil {
	// 	return ez.Wrap(op, err)
	// }
	return nil
}

type ListResponse struct {
	pagination.CursorResponse
	AgentSpecs []agent.Spec `json:"agent_specs"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "specs.API.List"

	// Query
	items := []agent.Spec{}
	model := agent.Spec{}

	selectQuery := api.db.NewSelect().
		Model(&items)

	// Default newest-first by cursor (UUIDv7)
	selectQuery, err := pagination.ApplyCursorToQuery(selectQuery, &request.CursorRequest, model, pagination.DESC)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = selectQuery.Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Cursor response
	resp, err := pagination.BuildCursorResponse(items, request.Limit)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ListResponse{
		AgentSpecs:     resp.GetItems().([]agent.Spec),
		CursorResponse: *resp,
	}, nil
}
