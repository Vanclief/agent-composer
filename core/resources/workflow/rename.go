package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/models/execution"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type RenameRequest struct {
	WorkflowID string `json:"workflow_id"`
	// NewID moves the workflow to a new id; empty keeps the current.
	NewID string `json:"new_id"`
	// Name sets the display name; empty keeps the current.
	Name string `json:"name"`
	// Description sets the description; nil keeps the current, ""
	// clears it.
	Description *string `json:"description,omitempty"`
}

func (r *RenameRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(ez.EINVALID, "workflow_id is required", nil)
	}
	if strings.TrimSpace(r.NewID) == "" && strings.TrimSpace(r.Name) == "" &&
		r.Description == nil {
		return ez.New(ez.EINVALID, "Nothing to update", nil)
	}

	return nil
}

type RenameResponse struct {
	WorkflowID string `json:"workflow_id"`
	// UpdatedRefs lists workflows whose blueprints were rewritten
	// because they embed the renamed workflow.
	UpdatedRefs []string `json:"updated_refs,omitempty"`
}

// Rename edits a workflow's identity: id, display name, and/or
// description. An id change
// cascades: registry file, draft, versions archive, embedding
// blueprints, and the run history rows that key Monitor's views.
func (api *API) Rename(ctx context.Context, requester interface{}, request *RenameRequest) (*RenameResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	oldID := strings.TrimSpace(request.WorkflowID)
	newID := strings.TrimSpace(request.NewID)
	effectiveID := oldID
	updatedRefs := []string{}

	if newID != "" && newID != oldID {
		result, err := workflowruntime.RenameWorkflowID(oldID, newID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		effectiveID = result.WorkflowID
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
		err = workflowruntime.SetWorkflowHeader(
			effectiveID,
			request.Name,
			request.Description,
		)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return &RenameResponse{
		WorkflowID:  effectiveID,
		UpdatedRefs: updatedRefs,
	}, nil
}
