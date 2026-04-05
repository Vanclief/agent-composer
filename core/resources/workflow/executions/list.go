package executions

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	pagination.CursorRequest

	WorkflowID string `json:"workflow_id,omitempty"`
}

func (r *ListRequest) Validate() error {
	const op = "workflow.executions.ListRequest.Validate"

	err := r.CursorRequest.Validate()
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type ListResponse struct {
	pagination.CursorResponse
	WorkflowExecutions []execution.WorkflowExecution `json:"workflow_executions"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "workflow.executions.API.List"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	items := []execution.WorkflowExecution{}
	model := execution.WorkflowExecution{}

	q := api.db.NewSelect().Model(&items)

	workflowID := strings.TrimSpace(request.WorkflowID)
	if workflowID != "" {
		q = q.Where("workflow_id = ?", workflowID)
	}

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
		WorkflowExecutions: resp.GetItems().([]execution.WorkflowExecution),
		CursorResponse:     *resp,
	}, nil
}
