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
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	workflows, err := api.Registry.List(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// Composed-but-never-saved workflows exist only as drafts; they
	// still belong in the list so a reload cannot orphan them.
	draftOnly, err := api.Registry.ListDraftOnly(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	workflows = append(workflows, draftOnly...)

	return &ListResponse{
		Workflows: workflows,
	}, nil
}
