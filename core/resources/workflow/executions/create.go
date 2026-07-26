package executions

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	executionmodels "github.com/vanclief/agent-composer/models/execution"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	WorkflowID string         `json:"workflow_id,omitempty"`
	File       string         `json:"file,omitempty"`
	Input      map[string]any `json:"input"`
	ShellRoot  string         `json:"shell_root,omitempty"`
	// Worktree is a branch name; the run executes in that branch's
	// worktree of the ShellRoot repository (created on demand).
	Worktree string `json:"worktree,omitempty"`
	// Base is the start point when Worktree creates a new branch.
	Base string `json:"base,omitempty"`
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
	ExecutionID     string                                  `json:"execution_id,omitempty"`
	WorkflowID      string                                  `json:"workflow_id"`
	WorkflowVersion string                                  `json:"workflow_version"`
	Status          executionmodels.WorkflowExecutionStatus `json:"status"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	const op = "workflow.executions.API.Create"

	prepared, err := api.prepareExecution(ctx, request)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	handle, err := prepared.Executor.Start(ctx, prepared.Snapshot, request.Input)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	executionID := ""
	if handle != nil {
		executionID = handle.ID.String()
	}

	return &CreateResponse{
		ExecutionID:     executionID,
		WorkflowID:      prepared.Snapshot.WorkflowID,
		WorkflowVersion: prepared.Snapshot.WorkflowVersion,
		Status:          executionmodels.WorkflowExecutionStatusRunning,
	}, nil
}

type preparedExecution struct {
	Snapshot *workflowruntime.Snapshot
	Executor *workflowruntime.Executor
}

func (api *API) prepareExecution(ctx context.Context, request *CreateRequest) (*preparedExecution, error) {
	const op = "workflow.executions.API.prepareExecution"

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

	if branch := strings.TrimSpace(request.Worktree); branch != "" {
		if api.worktrees == nil {
			return nil, ez.New(op, ez.EINTERNAL, "Worktree manager is unavailable", nil)
		}

		repo, isGit, err := api.worktrees.RepoRoot(ctx, shellRoot)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		if !isGit {
			return nil, ez.New(op, ez.EINVALID, shellRoot+" is not a git repository", nil)
		}

		path, _, err := api.worktrees.Resolve(ctx, repo, branch, request.Base)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		shellRoot = path
	}

	executor := workflowruntime.NewExecutor(shellRoot)
	if api.newRecorder != nil {
		executor.Recorder = api.newRecorder()
	}

	return &preparedExecution{
		Snapshot: snapshot,
		Executor: executor,
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
