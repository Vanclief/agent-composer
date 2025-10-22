package runs

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/models/parrot"
	"github.com/vanclief/agent-composer/models/provider"
)

type ListRequest struct {
    pagination.CursorRequest

    // Optional filters
    Provider   *provider.LLMProvider `json:"provider,omitempty"`
    Search     string                `json:"search"`
    TemplateID uuid.UUID             `json:"template_id,omitempty"`
    Status     *parrot.RunStatus     `json:"status,omitempty"`
}

func (r *ListRequest) Validate() error {
	const op = "parrots.ListRequest.Validate"

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
	Parrots []parrot.Run `json:"parrot_runs"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "parrots.API.List"

	// Query
	items := []parrot.Run{}
	model := parrot.Run{}

    selectQuery := api.db.NewSelect().
        Model(&items).
        Column("run.id", "run.template_id", "run.name", "run.provider", "run.status")

    // Filters
    if request.TemplateID != uuid.Nil {
        selectQuery = selectQuery.Where("run.template_id = ?", request.TemplateID)
    }

    if request.Status != nil {
        selectQuery = selectQuery.Where("run.status = ?", *request.Status)
    }

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
		Parrots:        resp.GetItems().([]parrot.Run),
		CursorResponse: *resp,
	}, nil
}
