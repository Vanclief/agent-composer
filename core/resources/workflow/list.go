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

	for index := range workflows {
		draft, err := workflowruntime.ReadDraft(workflows[index].ID)
		if err == nil && draft != "" {
			workflows[index].HasDraft = true
		}
	}

	// Composed-but-never-saved workflows exist only as drafts; they
	// still belong in the list so a reload cannot orphan them.
	draftOnly, err := workflowruntime.ListDraftOnlyBlueprints()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	for _, summary := range draftOnly {
		summary.HasDraft = true
		summary.DraftOnly = true
		workflows = append(workflows, summary)
	}

	return &ListResponse{
		Workflows: workflows,
	}, nil
}
