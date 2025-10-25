package conversations

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	pagination.CursorRequest

	// Optional filters
	Provider    *agent.LLMProvider        `json:"provider,omitempty"`
	Search      string                    `json:"search"`
	AgentSpecID uuid.UUID                 `json:"agent_spec_id,omitempty"`
	Status      *agent.ConversationStatus `json:"status,omitempty"`
}

func (r *ListRequest) Validate() error {
	const op = "conversations.ListRequest.Validate"

	err := r.CursorRequest.Validate()
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type ListResponse struct {
	pagination.CursorResponse
	Conversations []agent.Conversation `json:"conversations"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "conversations.API.List"

	items := []agent.Conversation{}
	model := agent.Conversation{}

	selectQuery := api.db.NewSelect().
		Model(&items)

	if request.Provider != nil {
		selectQuery = selectQuery.Where("conversation.provider = ?", *request.Provider)
	}

	if request.AgentSpecID != uuid.Nil {
		selectQuery = selectQuery.Where("conversation.agent_spec_id = ?", request.AgentSpecID)
	}

	if request.Status != nil {
		selectQuery = selectQuery.Where("conversation.status = ?", *request.Status)
	}

	if request.Search != "" {
		selectQuery = selectQuery.Where("conversation.name ILIKE ?", "%"+request.Search+"%")
	}

	selectQuery, err := pagination.ApplyCursorToQuery(selectQuery, &request.CursorRequest, model, pagination.DESC)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = selectQuery.Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	resp, err := pagination.BuildCursorResponse(items, request.Limit)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ListResponse{
		Conversations:  resp.GetItems().([]agent.Conversation),
		CursorResponse: *resp,
	}, nil
}
