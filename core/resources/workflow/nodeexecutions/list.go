package nodeexecutions

import (
	"context"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	pagination.CursorRequest

	WorkflowExecutionID uuid.UUID `json:"workflow_execution_id,omitempty"`
}

func (r *ListRequest) Validate() error {
	err := r.CursorRequest.Validate()
	if err != nil {
		return ez.New(ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type ListResponse struct {
	pagination.CursorResponse
	NodeExecutions []execution.NodeExecution `json:"node_executions"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	items := []execution.NodeExecution{}
	model := execution.NodeExecution{}

	q := api.db.NewSelect().Model(&items)

	if request.WorkflowExecutionID != uuid.Nil {
		q = q.Where("workflow_execution_id = ?", request.WorkflowExecutionID)
	}

	q, err = pagination.ApplyCursorToQuery(q, &request.CursorRequest, model, pagination.DESC)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = q.Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	resp, err := pagination.BuildCursorResponse(items, request.Limit)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &ListResponse{
		NodeExecutions: resp.GetItems().([]execution.NodeExecution),
		CursorResponse: *resp,
	}, nil
}
