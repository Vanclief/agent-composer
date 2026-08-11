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
	sourceID, err := uuid.Parse(strings.TrimSpace(request.ResumeFromExecutionID))
	if err != nil {
		return nil, ez.New(ez.EINVALID, "resume_from_execution_id must be a UUID", err)
	}

	source, err := executionmodels.GetWorkflowExecutionByID(ctx, api.db, sourceID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	target := strings.TrimSpace(request.ResumeFromNode)

	var snapshot *workflowruntime.Snapshot
	if request.UseCurrentSpec {
		spec, err := api.registry.Load(ctx, source.WorkflowSlug)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		snapshot, err = api.registry.Compile(ctx, spec)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	} else {
		snapshot = &workflowruntime.Snapshot{}
		err = json.Unmarshal(source.WorkflowSnapshot, snapshot)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	if _, exists := snapshot.Nodes[target]; !exists {
		return nil, ez.New(
			ez.EINVALID,
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
		return nil, ez.Wrap(err)
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

	project := strings.TrimSpace(request.ProjectDir)
	if project == "" {
		project = source.ProjectDir
	}
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

	// The source may have run in a worktree that was deleted since.
	info, statErr := os.Stat(project)
	if statErr != nil || !info.IsDir() {
		return nil, ez.New(
			ez.EINVALID,
			"The original run's directory "+project+" no longer exists (its workspace may have been deleted). Launch the workflow again with a project/workspace, or recreate the workspace first.",
			statErr,
		)
	}

	executor := workflowruntime.NewExecutor(project)
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
