package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// WorkflowSlug overrides the id derived from the name.
	WorkflowSlug string `json:"workflow_slug"`
}

func (r *CreateRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ez.New(ez.EINVALID, "name is required", nil)
	}

	return nil
}

type CreateResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Name         string `json:"name"`
	// Draft is the scaffolded spec, waiting for nodes and Save.
	Draft string `json:"draft"`
}

// Create scaffolds a new named workflow as a draft — nothing lands in
// the registry until it is saved.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	created, err := api.Registry.CreateDraft(
		ctx,
		request.Name,
		request.Description,
		request.WorkflowSlug,
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &CreateResponse{
		WorkflowSlug: created.WorkflowSlug,
		Name:         strings.TrimSpace(request.Name),
		Draft:        created.Spec,
	}, nil
}
