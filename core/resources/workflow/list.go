package workflow

import (
	"context"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type ListRequest struct{}

func (r *ListRequest) Validate() error {
	return nil
}

type ListResponse struct {
	Workflows []workflowruntime.WorkflowSummary `json:"workflows"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	const op = "workflow.API.List"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	workflows, err := workflowruntime.ListBlueprints()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &ListResponse{
		Workflows: workflows,
	}, nil
}
