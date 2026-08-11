package executions

import (
	"context"
	"encoding/json"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	executionmodels "github.com/vanclief/agent-composer/models/execution"

	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	WorkflowSlug string         `json:"workflow_slug,omitempty"`
	File         string         `json:"file,omitempty"`
	Input        map[string]any `json:"input"`
	ProjectDir   string         `json:"project_dir,omitempty"`
	// Worktree is a branch name; the run executes in that branch's
	// worktree of the ProjectDir repository (created on demand).
	Worktree string `json:"worktree,omitempty"`
	// Base is the start point when Worktree creates a new branch.
	Base string `json:"base,omitempty"`
	// ResumeFromExecutionID + ResumeFromNode re-run a prior execution
	// from one node: upstream outputs are reused by reference, the node
	// and everything downstream of it run again.
	ResumeFromExecutionID string `json:"resume_from_execution_id,omitempty"`
	ResumeFromNode        string `json:"resume_from_node,omitempty"`
	// UseCurrentSpec recompiles the workflow's current YAML instead of
	// replaying the stored snapshot (e.g. after a prompt edit).
	UseCurrentSpec bool `json:"use_current_spec,omitempty"`
}

func (r CreateRequest) isResume() bool {
	return strings.TrimSpace(r.ResumeFromExecutionID) != "" ||
		strings.TrimSpace(r.ResumeFromNode) != ""
}

func (r CreateRequest) Validate() error {
	if r.isResume() {
		if strings.TrimSpace(r.ResumeFromExecutionID) == "" ||
			strings.TrimSpace(r.ResumeFromNode) == "" {
			return ez.New(ez.EINVALID, "resume_from_execution_id and resume_from_node are required together", nil)
		}
		return nil
	}

	workflowID := strings.TrimSpace(r.WorkflowSlug)
	file := strings.TrimSpace(r.File)
	if workflowID == "" && file == "" {
		return ez.New(ez.EINVALID, "one of workflow_id or file is required", nil)
	}

	err := validation.ValidateStruct(&r,
		validation.Field(&r.Input, validation.Required),
	)
	if err != nil {
		return ez.New(ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type CreateResponse struct {
	ExecutionID     string                                  `json:"execution_id,omitempty"`
	WorkflowSlug    string                                  `json:"workflow_slug"`
	WorkflowVersion string                                  `json:"workflow_version"`
	Status          executionmodels.WorkflowExecutionStatus `json:"status"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	prepared, err := api.prepareExecution(ctx, request)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	handle, err := prepared.Executor.Start(ctx, prepared.Snapshot, prepared.Input)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	executionID := ""
	if handle != nil {
		executionID = handle.ID.String()

		err = api.writeResumeMetadata(ctx, handle, prepared.Metadata)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return &CreateResponse{
		ExecutionID:     executionID,
		WorkflowSlug:    prepared.Snapshot.WorkflowSlug,
		WorkflowVersion: prepared.Snapshot.WorkflowVersion,
		Status:          executionmodels.WorkflowExecutionStatusRunning,
	}, nil
}

// RunResponse is the result of a synchronous workflow run.
type RunResponse struct {
	ExecutionID     string                                  `json:"execution_id,omitempty"`
	WorkflowSlug    string                                  `json:"workflow_slug"`
	WorkflowVersion string                                  `json:"workflow_version"`
	Status          executionmodels.WorkflowExecutionStatus `json:"status"`
	Output          map[string]any                          `json:"output,omitempty"`
}

// Run executes a workflow synchronously and returns its final output. The
// CLI uses it so the process stays alive until the run finishes, unlike
// Create, which detaches the run and only makes sense inside a long-lived
// process like the REST server.
func (api *API) Run(ctx context.Context, requester interface{}, request *CreateRequest) (*RunResponse, error) {
	prepared, err := api.prepareExecution(ctx, request)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	output, handle, runErr := prepared.Executor.RunWithHandle(ctx, prepared.Snapshot, prepared.Input)

	executionID := ""
	if handle != nil {
		executionID = handle.ID.String()

		err = api.writeResumeMetadata(ctx, handle, prepared.Metadata)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	if runErr != nil {
		return nil, ez.Wrap(runErr)
	}

	return &RunResponse{
		ExecutionID:     executionID,
		WorkflowSlug:    prepared.Snapshot.WorkflowSlug,
		WorkflowVersion: prepared.Snapshot.WorkflowVersion,
		Status:          executionmodels.WorkflowExecutionStatusSucceeded,
		Output:          output,
	}, nil
}

// writeResumeMetadata records resume provenance on the execution row: which
// run it came from and which node executions were reused by reference
// rather than re-run.
func (api *API) writeResumeMetadata(ctx context.Context, handle *workflowruntime.WorkflowExecutionHandle, metadata map[string]any) error {
	if handle == nil || len(metadata) == 0 {
		return nil
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return ez.Wrap(err)
	}

	// No explicit jsonb cast so the query works on both PostgreSQL
	// (which infers jsonb from the column) and SQLite
	_, err = api.db.NewUpdate().
		Model((*executionmodels.WorkflowExecution)(nil)).
		Set("metadata = ?", string(metadataJSON)).
		Where("id = ?", handle.ID).
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

type preparedExecution struct {
	Snapshot *workflowruntime.Snapshot
	Executor *workflowruntime.Executor
	Input    map[string]any
	Metadata map[string]any
}

func (api *API) prepareExecution(ctx context.Context, request *CreateRequest) (*preparedExecution, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if request.isResume() {
		prepared, err := api.prepareResume(ctx, request)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		return prepared, nil
	}

	spec, err := api.loadSpec(ctx, request)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	snapshot, err := api.registry.Compile(ctx, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	project := strings.TrimSpace(request.ProjectDir)
	if project == "" && api.rt != nil {
		project = api.rt.ProjectDir()
	}

	if branch := strings.TrimSpace(request.Worktree); branch != "" {
		if api.worktrees == nil {
			return nil, ez.New(ez.EINTERNAL, "Worktree manager is unavailable", nil)
		}

		repo, isGit, err := api.worktrees.RepoRoot(ctx, project)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		if !isGit {
			return nil, ez.New(ez.EINVALID, project+" is not a git repository", nil)
		}

		path, _, err := api.worktrees.Resolve(ctx, repo, branch, request.Base)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		project = path
	}

	executor := workflowruntime.NewExecutor(project)
	if api.newRecorder != nil {
		executor.Recorder = api.newRecorder()
	}

	return &preparedExecution{
		Snapshot: snapshot,
		Executor: executor,
		Input:    request.Input,
	}, nil
}

func (api *API) loadSpec(ctx context.Context, request *CreateRequest) (*workflowruntime.Spec, error) {
	workflowID := strings.TrimSpace(request.WorkflowSlug)
	if workflowID != "" {
		spec, err := api.registry.Load(ctx, workflowID)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		return spec, nil
	}

	spec, err := workflowruntime.LoadSpecFile(strings.TrimSpace(request.File))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return spec, nil
}
