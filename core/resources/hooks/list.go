package hooks

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/runtime/events"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	pagination.CursorRequest

	// Optional filters
	EventType    *events.EventType `json:"event_type,omitempty"`
	TemplateName *string           `json:"template_name,omitempty"`
	Search       string            `json:"search"` // ILIKE on template_name/command
}

func (r *ListRequest) Validate() error {
	const op = "hooks.ListRequest.Validate"

	err := r.CursorRequest.Validate()
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}
	return nil
}

type ListResponse struct {
	pagination.CursorResponse
	Hooks []hook.Hook `json:"hooks"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "hooks.API.List"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: permissions

	items := []hook.Hook{}
	model := hook.Hook{}

	q := api.db.NewSelect().Model(&items)

	// Filters
	if request.EventType != nil {
		q = q.Where("event_type = ?", *request.EventType)
	}
	if request.TemplateName != nil {
		q = q.Where("template_name = ?", strings.TrimSpace(*request.TemplateName))
	}
	if strings.TrimSpace(request.Search) != "" {
		search := "%" + strings.TrimSpace(request.Search) + "%"
		q = q.Where("(template_name ILIKE ? OR command ILIKE ?)", search, search)
	}

	// Newest-first by cursor (UUIDv7 or your model's rules)
	q, err = pagination.ApplyCursorToQuery(q, &request.CursorRequest, model, pagination.DESC)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = q.Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	resp, err := pagination.BuildCursorResponse(items, request.Limit)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ListResponse{
		Hooks:          resp.GetItems().([]hook.Hook),
		CursorResponse: *resp,
	}, nil
}
