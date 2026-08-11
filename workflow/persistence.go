package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type ExecutionRecorder interface {
	StartWorkflow(ctx context.Context, snapshot *Snapshot, input map[string]any, project string) (WorkflowExecutionHandle, error)
	FinishWorkflow(ctx context.Context, handle WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error
	StartNode(ctx context.Context, workflow WorkflowExecutionHandle, node NodeSnapshot, input map[string]any, scope NodeExecutionScope) (NodeExecutionHandle, error)
	FinishNode(ctx context.Context, handle NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error
	StartConversation(ctx context.Context, handle NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error
	FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error
}

type WorkflowExecutionHandle struct {
	ID uuid.UUID
}

type NodeExecutionHandle struct {
	ID uuid.UUID
}

type NodeExecutionScope struct {
	ParentNodeExecutionID uuid.UUID
	IterationIndex        *int
	BranchName            string
}

type DBRecorder struct {
	db bun.IDB
}

func NewDBRecorder(db bun.IDB) *DBRecorder {
	if db == nil {
		return nil
	}

	return &DBRecorder{db: db}
}

func (r *DBRecorder) StartWorkflow(ctx context.Context, snapshot *Snapshot, input map[string]any, project string) (WorkflowExecutionHandle, error) {
	if snapshot == nil {
		return WorkflowExecutionHandle{}, ez.New(ez.EINVALID, "workflow snapshot is nil", nil)
	}

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return WorkflowExecutionHandle{}, ez.Wrap(err)
	}

	now := time.Now().UTC()
	record := &executionmodels.WorkflowExecution{
		WorkflowSlug:     snapshot.WorkflowSlug,
		WorkflowVersion:  snapshot.WorkflowVersion,
		WorkflowID:       parseWorkflowID(snapshot.WorkflowID),
		WorkflowSnapshot: snapshotJSON,
		InputSnapshot:    cloneMap(input),
		Status:           executionmodels.WorkflowExecutionStatusRunning,
		ProjectDir:       project,
		StartedAt:        &now,
		CreatedAt:        now,
	}

	err = record.Insert(ctx, r.db)
	if err != nil {
		return WorkflowExecutionHandle{}, ez.Wrap(err)
	}

	return WorkflowExecutionHandle{ID: record.ID}, nil
}

func (r *DBRecorder) FinishWorkflow(ctx context.Context, handle WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error {
	record := &executionmodels.WorkflowExecution{
		ID:             handle.ID,
		OutputSnapshot: cloneMap(output),
		Status:         status,
	}
	finishedAt := time.Now().UTC()
	record.FinishedAt = &finishedAt

	_, err := r.db.NewUpdate().
		Model(record).
		Column("output_snapshot", "status", "finished_at").
		Where("id = ?", handle.ID).
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (r *DBRecorder) StartNode(ctx context.Context, workflow WorkflowExecutionHandle, node NodeSnapshot, input map[string]any, scope NodeExecutionScope) (NodeExecutionHandle, error) {
	snapshotJSON, err := json.Marshal(node)
	if err != nil {
		return NodeExecutionHandle{}, ez.Wrap(err)
	}

	now := time.Now().UTC()
	record := &executionmodels.NodeExecution{
		WorkflowExecutionID:   workflow.ID,
		ParentNodeExecutionID: scope.ParentNodeExecutionID,
		NodeID:                node.InstanceID,
		Kind:                  node.Kind,
		Status:                executionmodels.NodeExecutionStatusRunning,
		NodeSnapshot:          snapshotJSON,
		InputSnapshot:         cloneMap(input),
		IterationIndex:        scope.IterationIndex,
		BranchName:            scope.BranchName,
		StartedAt:             &now,
		CreatedAt:             now,
	}

	err = record.Insert(ctx, r.db)
	if err != nil {
		return NodeExecutionHandle{}, ez.Wrap(err)
	}

	return NodeExecutionHandle{ID: record.ID}, nil
}

func (r *DBRecorder) FinishNode(ctx context.Context, handle NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error {
	record := &executionmodels.NodeExecution{
		ID:             handle.ID,
		OutputSnapshot: cloneMap(output),
		Status:         status,
		Trace:          cloneMap(trace),
	}
	finishedAt := time.Now().UTC()
	record.FinishedAt = &finishedAt

	_, err := r.db.NewUpdate().
		Model(record).
		Column("output_snapshot", "status", "trace", "finished_at").
		Where("id = ?", handle.ID).
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (r *DBRecorder) StartConversation(ctx context.Context, handle NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error {
	if conversation == nil {
		return ez.New(ez.EINVALID, "conversation is nil", nil)
	}

	conversation.NodeExecutionID = handle.ID
	conversation.InputSnapshot = cloneMap(input)

	err := conversation.Insert(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (r *DBRecorder) FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error {
	if conversation == nil {
		return ez.New(ez.EINVALID, "conversation is nil", nil)
	}

	conversation.OutputSnapshot = normalizeConversationOutput(output)

	err := conversation.Update(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// parseWorkflowID maps the snapshot's uuid string to the DB column;
// legacy specs without one record NULL.
func parseWorkflowID(value string) uuid.UUID {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}

	return dst
}

func normalizeConversationOutput(output any) map[string]any {
	if output == nil {
		return nil
	}

	typed, ok := output.(map[string]any)
	if ok {
		return cloneMap(typed)
	}

	return map[string]any{
		"value": output,
	}
}

func makeErrorTrace(err error) map[string]any {
	if err == nil {
		return nil
	}

	return map[string]any{
		"error": err.Error(),
	}
}

func whileTargetSnapshotNode(target WhileTargetSnapshot) NodeSnapshot {
	outputs := map[string]Port{
		target.UpdateOutputName: {
			Name:   target.UpdateOutputName,
			Schema: target.UpdateOutputSchema,
		},
		target.BreakOutputName: {
			Name:   target.BreakOutputName,
			Schema: map[string]any{"type": "boolean"},
		},
	}

	return NodeSnapshot{
		InstanceID:                target.InstanceID,
		NodeName:                  target.NodeName,
		Kind:                      "inference",
		Instruction:               target.Instruction,
		Harness:                   target.Harness,
		Model:                     target.Model,
		ReasoningEffort:           target.ReasoningEffort,
		HarnessConfig:             target.HarnessConfig,
		Inputs:                    target.Inputs,
		Outputs:                   outputs,
		StructuredOutputSchema:    target.StructuredOutputSchema,
		StructuredOutputSchemaRaw: target.StructuredOutputSchemaRaw,
	}
}
