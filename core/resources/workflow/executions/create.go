package executions

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	WorkflowID string         `json:"workflow_id,omitempty"`
	File       string         `json:"file,omitempty"`
	Input      map[string]any `json:"input"`
	ShellRoot  string         `json:"shell_root,omitempty"`
}

func (r CreateRequest) Validate() error {
	const op = "workflow.executions.CreateRequest.Validate"

	workflowID := strings.TrimSpace(r.WorkflowID)
	file := strings.TrimSpace(r.File)
	if workflowID == "" && file == "" {
		return ez.New(op, ez.EINVALID, "one of workflow_id or file is required", nil)
	}

	err := validation.ValidateStruct(&r,
		validation.Field(&r.Input, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type CreateResponse struct {
	ExecutionID     string         `json:"execution_id,omitempty"`
	WorkflowID      string         `json:"workflow_id"`
	WorkflowVersion string         `json:"workflow_version"`
	Output          map[string]any `json:"output"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	const op = "workflow.executions.API.Create"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	blueprint, err := api.loadBlueprint(request)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	snapshot, err := workflowruntime.Compile(blueprint)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	shellRoot := strings.TrimSpace(request.ShellRoot)
	if shellRoot == "" && api.rt != nil {
		shellRoot = api.rt.ShellRoot()
	}

	executor := workflowruntime.NewExecutor(shellRoot)
	if api.newRecorder != nil {
		executor.Recorder = api.newRecorder()
	}

	output, handle, err := executor.RunWithHandle(ctx, snapshot, request.Input)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	executionID := ""
	if handle != nil {
		executionID = handle.ID.String()
	}

	return &CreateResponse{
		ExecutionID:     executionID,
		WorkflowID:      snapshot.WorkflowID,
		WorkflowVersion: snapshot.WorkflowVersion,
		Output:          output,
	}, nil
}

func (api *API) loadBlueprint(request *CreateRequest) (*workflowruntime.Blueprint, error) {
	const op = "workflow.executions.API.loadBlueprint"

	workflowID := strings.TrimSpace(request.WorkflowID)
	if workflowID != "" {
		blueprint, err := workflowruntime.LoadBlueprintByWorkflowID(workflowID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		return blueprint, nil
	}

	blueprint, err := workflowruntime.LoadBlueprintFile(strings.TrimSpace(request.File))
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return blueprint, nil
}
