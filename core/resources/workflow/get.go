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
	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(ez.EINVALID, "workflow_id is required", nil)
	}

	return nil
}

type GetResponse struct {
	WorkflowID string `json:"workflow_id"`
	Spec       string `json:"spec"`
	// Draft holds unsaved composer changes; empty when none exist.
	Draft string `json:"draft,omitempty"`
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*GetResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	workflowID := strings.TrimSpace(request.WorkflowID)

	draft, err := workflowruntime.ReadDraft(workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	raw, err := workflowruntime.ReadBlueprintBytesByWorkflowID(workflowID)
	if err != nil {
		// A never-saved workflow still exists as its draft.
		if draft != "" && ez.ErrorCode(err) == ez.ENOTFOUND {
			return &GetResponse{
				WorkflowID: workflowID,
				Draft:      draft,
			}, nil
		}

		return nil, ez.Wrap(err)
	}

	return &GetResponse{
		WorkflowID: workflowID,
		Spec:       string(raw),
		Draft:      draft,
	}, nil
}
