package executions

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/google/uuid"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

// prepareResume builds an execution that re-runs one node of a prior
// execution plus everything downstream of it. Upstream outputs are
// seeded in memory and referenced (never copied) in the new
// execution's metadata: resumed_from + reused_nodes.
func (api *API) prepareResume(ctx context.Context, request *CreateRequest) (*preparedExecution, error) {
	const op = "workflow.executions.API.prepareResume"

	sourceID, err := uuid.Parse(strings.TrimSpace(request.ResumeFromExecutionID))
	if err != nil {
		return nil, ez.New(op, ez.EINVALID, "resume_from_execution_id must be a UUID", err)
	}

	source, err := executionmodels.GetWorkflowExecutionByID(ctx, api.db, sourceID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	target := strings.TrimSpace(request.ResumeFromNode)

	var snapshot *workflowruntime.Snapshot
	if request.UseCurrentSpec {
		blueprint, err := workflowruntime.LoadBlueprintByWorkflowID(source.WorkflowID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
		snapshot, err = workflowruntime.Compile(blueprint)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	} else {
		snapshot = &workflowruntime.Snapshot{}
		err = json.Unmarshal(source.WorkflowSnapshot, snapshot)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	if _, exists := snapshot.Nodes[target]; !exists {
		return nil, ez.New(
			op, ez.EINVALID,
			"Node "+target+" does not exist in the workflow snapshot",
			nil,
		)
	}

	// The source's top-level node executions, latest per node.
	rows := []executionmodels.NodeExecution{}
	err = api.db.NewSelect().
		Model(&rows).
		Where("workflow_execution_id = ?", sourceID).
		Where("parent_node_execution_id IS NULL").
		Order("created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	latest := map[string]executionmodels.NodeExecution{}
	for _, row := range rows {
		latest[row.NodeID] = row
	}

	downstream := workflowruntime.DownstreamOf(snapshot, target)

	seeds := map[string]map[string]any{}
	reused := map[string]any{}
	for nodeID, row := range latest {
		if downstream[nodeID] {
			continue
		}
		if row.Status != executionmodels.NodeExecutionStatusSucceeded {
			continue
		}
		if _, exists := snapshot.Nodes[nodeID]; !exists {
			// The current spec no longer has this node; it re-runs.
			continue
		}
		if row.OutputSnapshot == nil {
			continue
		}
		seeds[nodeID] = row.OutputSnapshot
		reused[nodeID] = row.ID.String()
	}

	input := request.Input
	if len(input) == 0 {
		input = source.InputSnapshot
	}

	shellRoot := strings.TrimSpace(request.ShellRoot)
	if shellRoot == "" {
		shellRoot = source.ShellRoot
	}
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

	// The source may have run in a worktree that was deleted since.
	info, statErr := os.Stat(shellRoot)
	if statErr != nil || !info.IsDir() {
		return nil, ez.New(
			op, ez.EINVALID,
			"The original run's directory "+shellRoot+" no longer exists (its workspace may have been deleted). Launch the workflow again with a project/workspace, or recreate the workspace first.",
			statErr,
		)
	}

	executor := workflowruntime.NewExecutor(shellRoot)
	if api.newRecorder != nil {
		executor.Recorder = api.newRecorder()
	}
	executor.SeedOutputs = seeds

	return &preparedExecution{
		Snapshot: snapshot,
		Executor: executor,
		Input:    input,
		Metadata: map[string]any{
			"resumed_from": sourceID.String(),
			"resume_node":  target,
			"reused_nodes": reused,
		},
	}, nil
}
