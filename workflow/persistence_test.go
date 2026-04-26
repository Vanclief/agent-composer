package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/agent-composer/runtime/harnesses"
)

type recordedWorkflowExecution struct {
	handle    WorkflowExecutionHandle
	snapshot  *Snapshot
	input     map[string]any
	output    map[string]any
	shellRoot string
	status    executionmodels.WorkflowExecutionStatus
}

type recordedNodeExecution struct {
	handle   NodeExecutionHandle
	node     NodeSnapshot
	input    map[string]any
	output   map[string]any
	scope    NodeExecutionScope
	status   executionmodels.NodeExecutionStatus
	trace    map[string]any
	workflow WorkflowExecutionHandle
}

type recordedConversation struct {
	nodeHandle     NodeExecutionHandle
	conversation   *agent.Conversation
	inputSnapshot  map[string]any
	outputSnapshot map[string]any
}

type recordingRecorder struct {
	workflows               []recordedWorkflowExecution
	nodes                   []recordedNodeExecution
	conversations           []recordedConversation
	workflowIndexByID       map[uuid.UUID]int
	nodeIndexByID           map[uuid.UUID]int
	conversationIndexByNode map[uuid.UUID]int
}

func newRecordingRecorder() *recordingRecorder {
	return &recordingRecorder{
		workflowIndexByID:       make(map[uuid.UUID]int),
		nodeIndexByID:           make(map[uuid.UUID]int),
		conversationIndexByNode: make(map[uuid.UUID]int),
	}
}

func (r *recordingRecorder) StartWorkflow(ctx context.Context, snapshot *Snapshot, input map[string]any, shellRoot string) (WorkflowExecutionHandle, error) {
	handle := WorkflowExecutionHandle{ID: uuid.New()}
	index := len(r.workflows)
	r.workflowIndexByID[handle.ID] = index
	r.workflows = append(r.workflows, recordedWorkflowExecution{
		handle:    handle,
		snapshot:  snapshot,
		input:     cloneMap(input),
		shellRoot: shellRoot,
		status:    executionmodels.WorkflowExecutionStatusRunning,
	})
	return handle, nil
}

func (r *recordingRecorder) FinishWorkflow(ctx context.Context, handle WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error {
	index, found := r.workflowIndexByID[handle.ID]
	if !found {
		return nil
	}

	r.workflows[index].output = cloneMap(output)
	r.workflows[index].status = status
	return nil
}

func (r *recordingRecorder) StartNode(ctx context.Context, workflowHandle WorkflowExecutionHandle, node NodeSnapshot, input map[string]any, scope NodeExecutionScope) (NodeExecutionHandle, error) {
	handle := NodeExecutionHandle{ID: uuid.New()}
	index := len(r.nodes)
	r.nodeIndexByID[handle.ID] = index
	r.nodes = append(r.nodes, recordedNodeExecution{
		handle:   handle,
		node:     node,
		input:    cloneMap(input),
		scope:    scope,
		status:   executionmodels.NodeExecutionStatusRunning,
		workflow: workflowHandle,
	})
	return handle, nil
}

func (r *recordingRecorder) FinishNode(ctx context.Context, handle NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error {
	index, found := r.nodeIndexByID[handle.ID]
	if !found {
		return nil
	}

	r.nodes[index].output = cloneMap(output)
	r.nodes[index].status = status
	r.nodes[index].trace = cloneMap(trace)
	return nil
}

func (r *recordingRecorder) StartConversation(ctx context.Context, handle NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error {
	conversation.NodeExecutionID = handle.ID
	conversation.InputSnapshot = cloneMap(input)
	index := len(r.conversations)
	r.conversationIndexByNode[handle.ID] = index
	r.conversations = append(r.conversations, recordedConversation{
		nodeHandle:     handle,
		conversation:   conversation,
		inputSnapshot:  cloneMap(input),
		outputSnapshot: nil,
	})
	return nil
}

func (r *recordingRecorder) FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error {
	index, found := r.conversationIndexByNode[conversation.NodeExecutionID]
	if !found {
		return nil
	}

	normalized := normalizeConversationOutput(output)
	conversation.OutputSnapshot = normalized
	r.conversations[index].outputSnapshot = normalized
	r.conversations[index].conversation = conversation
	return nil
}

func TestRunPersistsWorkflowNodeAndConversationRecords(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/article_summary.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	recorder := newRecordingRecorder()
	executor := NewExecutor("/tmp/workflow-shell")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return fakeHarness{}, nil
	}
	executor.Recorder = recorder

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"article_text": "A short article about a new bridge opening downtown.",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if len(recorder.workflows) != 1 {
		t.Fatalf("unexpected workflow executions: %d", len(recorder.workflows))
	}

	workflowRecord := recorder.workflows[0]
	if workflowRecord.status != executionmodels.WorkflowExecutionStatusSucceeded {
		t.Fatalf("unexpected workflow status: %q", workflowRecord.status)
	}
	if workflowRecord.shellRoot != "/tmp/workflow-shell" {
		t.Fatalf("unexpected workflow shell root: %q", workflowRecord.shellRoot)
	}
	if workflowRecord.output["final_summary"] == nil {
		t.Fatalf("missing persisted workflow output: %#v", workflowRecord.output)
	}

	if len(recorder.nodes) != 3 {
		t.Fatalf("unexpected node execution count: %d", len(recorder.nodes))
	}

	expectedNodeIDs := []string{"summarize_article", "critique_summary", "revise_summary"}
	for index, expectedNodeID := range expectedNodeIDs {
		record := recorder.nodes[index]
		if record.node.InstanceID != expectedNodeID {
			t.Fatalf("unexpected node execution at %d: %q", index, record.node.InstanceID)
		}
		if record.status != executionmodels.NodeExecutionStatusSucceeded {
			t.Fatalf("unexpected node status at %d: %q", index, record.status)
		}
		if record.scope.ParentNodeExecutionID != uuid.Nil {
			t.Fatalf("unexpected parent node id for top-level node %q", record.node.InstanceID)
		}
	}

	if len(recorder.conversations) != 3 {
		t.Fatalf("unexpected conversation count: %d", len(recorder.conversations))
	}

	for _, record := range recorder.conversations {
		if record.conversation.NodeExecutionID == uuid.Nil {
			t.Fatalf("conversation missing node execution id")
		}
		if record.conversation.Status != agent.ConversationStatusSucceeded {
			t.Fatalf("unexpected conversation status: %q", record.conversation.Status)
		}
		if record.conversation.ShellRoot != "/tmp/workflow-shell" {
			t.Fatalf("unexpected conversation shell root: %q", record.conversation.ShellRoot)
		}
		if len(record.conversation.Messages) != 3 {
			t.Fatalf("unexpected conversation message count: %d", len(record.conversation.Messages))
		}
		if record.inputSnapshot == nil {
			t.Fatalf("conversation input snapshot was not persisted")
		}
		if record.outputSnapshot == nil {
			t.Fatalf("conversation output snapshot was not persisted")
		}
	}

	if output["final_summary"] == nil {
		t.Fatalf("missing final workflow output: %#v", output)
	}
}

func TestMarshalWorkflowSnapshotWithConnectorNode(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/binary_vote_round.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	_, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal workflow snapshot: %v", err)
	}
}

func TestRunPersistsForeachChildNodeScope(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/section_summary_batch.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	recorder := newRecordingRecorder()
	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return sectionHarness{}, nil
	}
	executor.Recorder = recorder

	_, err = executor.Run(context.Background(), snapshot, map[string]any{
		"section_text": []any{"alpha", "beta"},
		"tone":         "concise",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if len(recorder.nodes) != 3 {
		t.Fatalf("unexpected node execution count: %d", len(recorder.nodes))
	}

	loopRecord := recorder.nodes[0]
	if loopRecord.node.Kind != "loop" {
		t.Fatalf("expected first node execution to be the loop, got %q", loopRecord.node.Kind)
	}

	for index, record := range recorder.nodes[1:] {
		if record.node.Kind != "inference" {
			t.Fatalf("unexpected child node kind: %q", record.node.Kind)
		}
		if record.scope.ParentNodeExecutionID != loopRecord.handle.ID {
			t.Fatalf("unexpected parent node id for foreach child: %s", record.scope.ParentNodeExecutionID)
		}
		if record.scope.IterationIndex == nil || *record.scope.IterationIndex != index {
			t.Fatalf("unexpected iteration index for foreach child: %#v", record.scope.IterationIndex)
		}
	}
}

func TestRunPersistsConditionalChildNodeScope(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/conditional_boolean_routing_review.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	recorder := newRecordingRecorder()
	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return conditionalHarness{}, nil
	}
	executor.Recorder = recorder

	_, err = executor.Run(context.Background(), snapshot, map[string]any{
		"text": "The bridge is expected to make cats pink.",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if len(recorder.nodes) != 3 {
		t.Fatalf("unexpected node execution count: %d", len(recorder.nodes))
	}

	conditionalRecord := recorder.nodes[1]
	if conditionalRecord.node.Kind != "conditional" {
		t.Fatalf("expected second node execution to be conditional, got %q", conditionalRecord.node.Kind)
	}

	branchRecord := recorder.nodes[2]
	if branchRecord.scope.ParentNodeExecutionID != conditionalRecord.handle.ID {
		t.Fatalf("unexpected conditional child parent id: %s", branchRecord.scope.ParentNodeExecutionID)
	}
	if branchRecord.scope.BranchName != "when_false" {
		t.Fatalf("unexpected conditional branch name: %q", branchRecord.scope.BranchName)
	}
	if branchRecord.node.InstanceID != "review_router__disagreement_explainer" {
		t.Fatalf("unexpected conditional child node id: %q", branchRecord.node.InstanceID)
	}
}

func TestRunPersistsWhileChildNodeScope(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/loop_while_binary_consensus.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	recorder := newRecordingRecorder()
	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return whileHarness{}, nil
	}
	executor.Recorder = recorder

	_, err = executor.Run(context.Background(), snapshot, map[string]any{
		"question": "Should we deploy the bridge update today?",
		"vote_state": map[string]any{
			"votes":                 []any{},
			"yes_count":             0,
			"no_count":              0,
			"agreement_ratio":       0.0,
			"minimum_votes_reached": false,
			"consensus_reached":     false,
		},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	if len(recorder.nodes) != 6 {
		t.Fatalf("unexpected node execution count: %d", len(recorder.nodes))
	}

	loopRecord := recorder.nodes[0]
	if loopRecord.node.Kind != "loop" {
		t.Fatalf("expected first node execution to be the while loop, got %q", loopRecord.node.Kind)
	}

	for index, record := range recorder.nodes[1:] {
		if record.node.Kind != "inference" {
			t.Fatalf("unexpected while child node kind: %q", record.node.Kind)
		}
		if record.scope.ParentNodeExecutionID != loopRecord.handle.ID {
			t.Fatalf("unexpected parent node id for while child: %s", record.scope.ParentNodeExecutionID)
		}
		if record.scope.IterationIndex == nil || *record.scope.IterationIndex != index {
			t.Fatalf("unexpected iteration index for while child: %#v", record.scope.IterationIndex)
		}
	}
}
