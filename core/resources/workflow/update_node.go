package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type UpdateNodeRequest struct {
	WorkflowID string `json:"workflow_id"`
	Node       string `json:"node"`
	// nil fields are left unchanged.
	Model       *string `json:"model,omitempty"`
	Harness     *string `json:"harness,omitempty"`
	Instruction *string `json:"instruction,omitempty"`
	// "" resets to the harness default.
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
}

func (r *UpdateNodeRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowID) == "" || strings.TrimSpace(r.Node) == "" {
		return ez.New(ez.EINVALID, "workflow_id and node are required", nil)
	}
	if r.Model == nil && r.Harness == nil && r.Instruction == nil &&
		r.ReasoningEffort == nil {
		return ez.New(ez.EINVALID, "Nothing to update", nil)
	}
	if r.Model != nil && strings.TrimSpace(*r.Model) == "" {
		return ez.New(ez.EINVALID, "model cannot be blank", nil)
	}
	if r.Harness != nil {
		err := agent.Harness(strings.TrimSpace(*r.Harness)).Validate()
		if err != nil {
			return ez.New(ez.EINVALID, "Unknown harness "+*r.Harness, err)
		}
	}
	if r.ReasoningEffort != nil && strings.TrimSpace(*r.ReasoningEffort) != "" {
		err := runtimetypes.ReasoningEffort(
			strings.TrimSpace(*r.ReasoningEffort),
		).Validate()
		if err != nil {
			return ez.New(ez.EINVALID, "Unknown reasoning effort "+*r.ReasoningEffort, err)
		}
	}

	return nil
}

type UpdateNodeResponse struct {
	WorkflowID string `json:"workflow_id"`
	Node       string `json:"node"`
	Spec       string `json:"spec"`
}

func (api *API) UpdateNode(ctx context.Context, requester interface{}, request *UpdateNodeRequest) (*UpdateNodeResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	trim := func(value *string) *string {
		if value == nil {
			return nil
		}
		trimmed := strings.TrimSpace(*value)
		return &trimmed
	}

	err = workflowruntime.UpdateNodeConfig(
		strings.TrimSpace(request.WorkflowID),
		strings.TrimSpace(request.Node),
		workflowruntime.NodeConfigUpdate{
			Model:           trim(request.Model),
			Harness:         trim(request.Harness),
			ReasoningEffort: trim(request.ReasoningEffort),
			// Instruction keeps its exact text — only the caller knows
			// whether whitespace matters.
			Instruction: request.Instruction,
		},
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	raw, err := workflowruntime.ReadBlueprintBytesByWorkflowID(
		strings.TrimSpace(request.WorkflowID),
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &UpdateNodeResponse{
		WorkflowID: strings.TrimSpace(request.WorkflowID),
		Node:       strings.TrimSpace(request.Node),
		Spec:       string(raw),
	}, nil
}
