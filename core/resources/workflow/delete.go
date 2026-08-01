package workflow

import (
	"context"
	"strings"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	WorkflowID string `json:"workflow_id"`
}

func (r *DeleteRequest) Validate() error {
	const op = "workflow.DeleteRequest.Validate"

	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(op, ez.EINVALID, "workflow_id is required", nil)
	}

	return nil
}

type DeleteResponse struct {
	WorkflowID string `json:"workflow_id"`
	Deleted    bool   `json:"deleted"`
}

// Delete removes a workflow from the library only — its run history
// and version archive stay untouched.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	const op = "workflow.API.Delete"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = workflowruntime.DeleteWorkflow(request.WorkflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &DeleteResponse{
		WorkflowID: strings.TrimSpace(request.WorkflowID),
		Deleted:    true,
	}, nil
}
