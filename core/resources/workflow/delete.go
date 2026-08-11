package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	WorkflowSlug string `json:"workflow_slug"`
}

func (r *DeleteRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowSlug) == "" {
		return ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}

	return nil
}

type DeleteResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Deleted      bool   `json:"deleted"`
}

// Delete removes a workflow from the library only — its run history
// and version archive stay untouched.
func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*DeleteResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.Registry.Delete(ctx, request.WorkflowSlug)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteResponse{
		WorkflowSlug: strings.TrimSpace(request.WorkflowSlug),
		Deleted:      true,
	}, nil
}
