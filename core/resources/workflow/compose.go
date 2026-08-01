package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/core/resources/settings"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type ComposeRequest struct {
	// WorkflowID is empty when the request should create a workflow.
	WorkflowID string `json:"workflow_id"`
	Request    string `json:"request"`
}

func (r *ComposeRequest) Validate() error {
	const op = "workflow.ComposeRequest.Validate"

	if strings.TrimSpace(r.Request) == "" {
		return ez.New(op, ez.EINVALID, "request is required", nil)
	}

	return nil
}

type ComposeResponse struct {
	WorkflowID string `json:"workflow_id"`
	Action     string `json:"action"`
	Summary    string `json:"summary"`
	Harness    string `json:"harness"`
	Model      string `json:"model"`
	// Draft is the proposed blueprint now waiting for Save — empty
	// when the composer proposed nothing.
	Draft string `json:"draft,omitempty"`
}

// composerAgent resolves which harness/model the composer runs on:
// the settings choice, or the first installed harness in the catalog.
func composerAgent(ctx context.Context) (agent.Harness, string, error) {
	const op = "workflow.composerAgent"

	data, err := settings.Load()
	if err != nil {
		return "", "", ez.Wrap(op, err)
	}

	harness := strings.TrimSpace(data.Composer.Harness)
	model := strings.TrimSpace(data.Composer.Model)
	if harness != "" && model != "" {
		return agent.Harness(harness), model, nil
	}

	for _, info := range harnesses.ListHarnessInfo(ctx) {
		if !info.Available || len(info.Models) == 0 {
			continue
		}
		if harness != "" && string(info.ID) != harness {
			continue
		}

		return info.ID, info.Models[0], nil
	}

	return "", "", ez.New(op, ez.EINVALID, "No harness is available for the composer — pick one in Settings", nil)
}

func (api *API) Compose(ctx context.Context, requester interface{}, request *ComposeRequest) (*ComposeResponse, error) {
	const op = "workflow.API.Compose"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	harness, model, err := composerAgent(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	workflowID := strings.TrimSpace(request.WorkflowID)

	// The edit base: an unsaved draft when one exists, else the saved
	// blueprint. Empty for a create.
	baseSpec := ""
	if workflowID != "" {
		baseSpec, err = workflowruntime.ReadDraft(workflowID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		if baseSpec == "" {
			raw, err := workflowruntime.ReadBlueprintBytesByWorkflowID(workflowID)
			if err != nil {
				return nil, ez.Wrap(op, err)
			}
			baseSpec = string(raw)
		}
	}

	result, err := workflowruntime.Compose(ctx, workflowruntime.ComposeOptions{
		WorkflowID: workflowID,
		BaseSpec:   baseSpec,
		Request:    request.Request,
		Harness:    harness,
		Model:      model,
	})
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	response := &ComposeResponse{
		WorkflowID: result.WorkflowID,
		Action:     result.Action,
		Summary:    result.Summary,
		Harness:    string(harness),
		Model:      model,
	}

	if result.Action == "unchanged" || result.YAML == "" {
		return response, nil
	}

	// Trust but verify: the proposal must compile on the server's own
	// compiler and keep its id before it becomes the draft.
	proposedID, err := workflowruntime.VerifyProposedBlueprint(
		[]byte(result.YAML),
		workflowID,
	)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = workflowruntime.WriteDraft(proposedID, []byte(result.YAML))
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	response.WorkflowID = proposedID
	response.Draft = result.YAML

	return response, nil
}

type SaveDraftRequest struct {
	WorkflowID string `json:"workflow_id"`
}

func (r *SaveDraftRequest) Validate() error {
	const op = "workflow.SaveDraftRequest.Validate"

	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(op, ez.EINVALID, "workflow_id is required", nil)
	}

	return nil
}

type SaveDraftResponse struct {
	WorkflowID string `json:"workflow_id"`
	Version    string `json:"version"`
	Spec       string `json:"spec"`
}

func (api *API) SaveDraft(ctx context.Context, requester interface{}, request *SaveDraftRequest) (*SaveDraftResponse, error) {
	const op = "workflow.API.SaveDraft"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	saved, err := workflowruntime.SaveDraft(request.WorkflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &SaveDraftResponse{
		WorkflowID: saved.WorkflowID,
		Version:    saved.Version,
		Spec:       saved.Spec,
	}, nil
}

type DeleteDraftRequest struct {
	WorkflowID string `json:"workflow_id"`
}

func (r *DeleteDraftRequest) Validate() error {
	const op = "workflow.DeleteDraftRequest.Validate"

	if strings.TrimSpace(r.WorkflowID) == "" {
		return ez.New(op, ez.EINVALID, "workflow_id is required", nil)
	}

	return nil
}

type DeleteDraftResponse struct {
	WorkflowID string `json:"workflow_id"`
	Deleted    bool   `json:"deleted"`
}

func (api *API) DeleteDraft(ctx context.Context, requester interface{}, request *DeleteDraftRequest) (*DeleteDraftResponse, error) {
	const op = "workflow.API.DeleteDraft"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = workflowruntime.DeleteDraft(request.WorkflowID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &DeleteDraftResponse{
		WorkflowID: strings.TrimSpace(request.WorkflowID),
		Deleted:    true,
	}, nil
}
