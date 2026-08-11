package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type GetRequest struct {
	WorkflowSlug string `json:"workflow_slug"`
}

func (r *GetRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowSlug) == "" {
		return ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}

	return nil
}

type GetResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Spec         string `json:"spec"`
	// Draft holds unsaved composer changes; empty when none exist.
	Draft string `json:"draft,omitempty"`
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*GetResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	workflowID := strings.TrimSpace(request.WorkflowSlug)

	draft, err := api.Registry.ReadDraft(ctx, workflowID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	raw, err := api.Registry.SpecBytes(ctx, workflowID)
	if err != nil {
		// A never-saved workflow still exists as its draft.
		if draft != "" && ez.ErrorCode(err) == ez.ENOTFOUND {
			return &GetResponse{
				WorkflowSlug: workflowID,
				Draft:        draft,
			}, nil
		}

		return nil, ez.Wrap(err)
	}

	return &GetResponse{
		WorkflowSlug: workflowID,
		Spec:         string(raw),
		Draft:        draft,
	}, nil
}
