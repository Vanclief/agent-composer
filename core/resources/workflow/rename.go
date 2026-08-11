package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type RenameRequest struct {
	WorkflowSlug string `json:"workflow_slug"`
	// NewID moves the workflow to a new id; empty keeps the current.
	NewID string `json:"new_id"`
	// Name sets the display name; empty keeps the current.
	Name string `json:"name"`
	// Description sets the description; nil keeps the current, ""
	// clears it.
	Description *string `json:"description,omitempty"`
}

func (r *RenameRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowSlug) == "" {
		return ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}
	if strings.TrimSpace(r.NewID) == "" && strings.TrimSpace(r.Name) == "" &&
		r.Description == nil {
		return ez.New(ez.EINVALID, "Nothing to update", nil)
	}

	return nil
}

type RenameResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	// UpdatedRefs lists workflows whose specs were rewritten
	// because they embed the renamed workflow.
	UpdatedRefs []string `json:"updated_refs,omitempty"`
}

// Rename edits a workflow's identity: id, display name, and/or
// description. An id change
// cascades: registry file, draft, versions archive, embedding
// specs, and the run history rows that key Monitor's views.
func (api *API) Rename(ctx context.Context, requester interface{}, request *RenameRequest) (*RenameResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	oldID := strings.TrimSpace(request.WorkflowSlug)
	newID := strings.TrimSpace(request.NewID)
	effectiveID := oldID
	updatedRefs := []string{}

	if newID != "" && newID != oldID {
		result, err := api.Registry.Rename(ctx, oldID, newID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		effectiveID = result.WorkflowSlug
		updatedRefs = result.UpdatedRefs

		// Run history follows the id so Monitor keeps the workflow's
		// past runs attached.
		_, err = api.db.NewUpdate().
			Model((*execution.WorkflowExecution)(nil)).
			Set("workflow_id = ?", effectiveID).
			Where("workflow_id = ?", oldID).
			Exec(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	if strings.TrimSpace(request.Name) != "" || request.Description != nil {
		err = api.Registry.SetHeader(
			ctx,
			effectiveID,
			request.Name,
			request.Description,
		)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return &RenameResponse{
		WorkflowSlug: effectiveID,
		UpdatedRefs:  updatedRefs,
	}, nil
}
