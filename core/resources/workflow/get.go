package workflow

import (
	"context"
	"strings"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	WorkflowID string `json:"workflow_id"`
}

func (r *GetRequest) Validate() error {
	const op = "workflow.GetRequest.Validate"

	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(op, ez.EINVALID, "workflow_id is required", nil)
	}

	return nil
}

type GetResponse struct {
	WorkflowID string `json:"workflow_id"`
	Spec       string `json:"spec"`
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*GetResponse, error) {
	const op = "workflow.API.Get"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	workflowID := strings.TrimSpace(request.WorkflowID)
	raw, err := workflowruntime.ReadBlueprintBytesByWorkflowID(workflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &GetResponse{
		WorkflowID: workflowID,
		Spec:       string(raw),
	}, nil
}
