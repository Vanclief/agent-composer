package workflow

import (
	"context"
	"strings"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *CreateRequest) Validate() error {
	const op = "workflow.CreateRequest.Validate"

	if strings.TrimSpace(r.Name) == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	return nil
}

type CreateResponse struct {
	WorkflowID string `json:"workflow_id"`
	Name       string `json:"name"`
	// Draft is the scaffolded blueprint, waiting for nodes and Save.
	Draft string `json:"draft"`
}

// Create scaffolds a new named workflow as a draft — nothing lands in
// the registry until it is saved.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	const op = "workflow.API.Create"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	created, err := workflowruntime.CreateDraft(
		request.Name,
		request.Description,
	)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &CreateResponse{
		WorkflowID: created.WorkflowID,
		Name:       strings.TrimSpace(request.Name),
		Draft:      created.Spec,
	}, nil
}
