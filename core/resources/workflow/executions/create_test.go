package executions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/agent-composer/runtime"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
)

type stubRecorder struct {
	startedWorkflows  int
	finishedWorkflows int
	startedNodes      int
	finishedNodes     int
	workflowStatuses  chan executionmodels.WorkflowExecutionStatus
}

func (r *stubRecorder) StartWorkflow(ctx context.Context, snapshot *workflowruntime.Snapshot, input map[string]any, shellRoot string) (workflowruntime.WorkflowExecutionHandle, error) {
	r.startedWorkflows++
	return workflowruntime.WorkflowExecutionHandle{ID: uuid.New()}, nil
}

func (r *stubRecorder) FinishWorkflow(ctx context.Context, handle workflowruntime.WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error {
	r.finishedWorkflows++
	if r.workflowStatuses != nil {
		r.workflowStatuses <- status
	}
	return nil
}

func (r *stubRecorder) StartNode(ctx context.Context, workflow workflowruntime.WorkflowExecutionHandle, node workflowruntime.NodeSnapshot, input map[string]any, scope workflowruntime.NodeExecutionScope) (workflowruntime.NodeExecutionHandle, error) {
	r.startedNodes++
	return workflowruntime.NodeExecutionHandle{}, nil
}

func (r *stubRecorder) FinishNode(ctx context.Context, handle workflowruntime.NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error {
	r.finishedNodes++
	return nil
}

func (r *stubRecorder) StartConversation(ctx context.Context, handle workflowruntime.NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error {
	return nil
}

func (r *stubRecorder) FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error {
	return nil
}

func TestCreate(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "pack-workflow.yaml")

	err := os.WriteFile(workflowPath, []byte(`
workflow:
  id: workflow_execution_api_test
  version: "1"
  inputs:
    title: string
    content: string
  outputs:
    out:
      schema: Draft
      from: instance.pack_draft.out

schemas:
  Draft:
    type: object
    properties:
      title:
        type: string
      content:
        type: string

nodes:
  pack_draft:
    kind: connector
    operation: pack
    inputs:
      title: string
      content: string
    outputs:
      out: Draft

flow:
  instances:
    pack_draft:
      node: pack_draft
      inputs:
        title: workflow_input.title
        content: workflow_input.content
`), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	recorder := &stubRecorder{
		workflowStatuses: make(chan executionmodels.WorkflowExecutionStatus, 1),
	}
	api := &API{
		rt: &runtime.Runtime{},
		newRecorder: func() workflowruntime.ExecutionRecorder {
			return recorder
		},
	}

	response, err := api.Create(context.Background(), nil, &CreateRequest{
		File: workflowPath,
		Input: map[string]any{
			"title":   "Bridge update",
			"content": "Widened sidewalks and new bike lanes.",
		},
	})
	if err != nil {
		t.Fatalf("create workflow execution: %v", err)
	}

	if response.WorkflowID != "workflow_execution_api_test" {
		t.Fatalf("unexpected workflow id: %q", response.WorkflowID)
	}

	if response.ExecutionID == "" {
		t.Fatalf("expected execution id to be returned")
	}

	if response.Status != executionmodels.WorkflowExecutionStatusRunning {
		t.Fatalf("unexpected create status: %q", response.Status)
	}

	select {
	case status := <-recorder.workflowStatuses:
		if status != executionmodels.WorkflowExecutionStatusSucceeded {
			t.Fatalf("unexpected finished status: %q", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached workflow completion")
	}

	if recorder.startedWorkflows != 1 || recorder.finishedWorkflows != 1 {
		t.Fatalf("unexpected workflow persistence calls: start=%d finish=%d", recorder.startedWorkflows, recorder.finishedWorkflows)
	}

	if recorder.startedNodes != 1 || recorder.finishedNodes != 1 {
		t.Fatalf("unexpected node persistence calls: start=%d finish=%d", recorder.startedNodes, recorder.finishedNodes)
	}
}

func TestCreateWithWorkflowID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", tempDir)

	workflowDir := filepath.Join(tempDir, "workflows")
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowPath := filepath.Join(workflowDir, "registry-pack-workflow.yaml")
	err = os.WriteFile(workflowPath, []byte(`
workflow:
  id: workflow_execution_registry_test
  version: "1"
  inputs:
    title: string
    content: string
  outputs:
    out:
      schema: Draft
      from: instance.pack_draft.out

schemas:
  Draft:
    type: object
    properties:
      title:
        type: string
      content:
        type: string

nodes:
  pack_draft:
    kind: connector
    operation: pack
    inputs:
      title: string
      content: string
    outputs:
      out: Draft

flow:
  instances:
    pack_draft:
      node: pack_draft
      inputs:
        title: workflow_input.title
        content: workflow_input.content
`), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	recorder := &stubRecorder{
		workflowStatuses: make(chan executionmodels.WorkflowExecutionStatus, 1),
	}
	api := &API{
		rt: &runtime.Runtime{},
		newRecorder: func() workflowruntime.ExecutionRecorder {
			return recorder
		},
	}

	response, err := api.Create(context.Background(), nil, &CreateRequest{
		WorkflowID: "workflow_execution_registry_test",
		Input: map[string]any{
			"title":   "Registry bridge update",
			"content": "Copied starter workflow and ran it from the registry.",
		},
	})
	if err != nil {
		t.Fatalf("create workflow execution by workflow id: %v", err)
	}

	if response.WorkflowID != "workflow_execution_registry_test" {
		t.Fatalf("unexpected workflow id: %q", response.WorkflowID)
	}

	if response.Status != executionmodels.WorkflowExecutionStatusRunning {
		t.Fatalf("unexpected create status: %q", response.Status)
	}

	select {
	case status := <-recorder.workflowStatuses:
		if status != executionmodels.WorkflowExecutionStatusSucceeded {
			t.Fatalf("unexpected finished status: %q", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached workflow completion")
	}
}

func TestCreateRecordsFailedStatusOnRuntimeFailure(t *testing.T) {
	tempDir := t.TempDir()
	workflowPath := filepath.Join(tempDir, "failing-workflow.yaml")

	err := os.WriteFile(workflowPath, []byte(`
workflow:
  id: workflow_execution_failure_test
  version: "1"
  inputs:
    source: Draft
  outputs:
    title:
      schema: string
      from: instance.unpack_draft.title

schemas:
  Draft:
    type: object
    properties:
      title:
        type: string

nodes:
  unpack_draft:
    kind: connector
    operation: unpack
    inputs:
      source: Draft
    outputs:
      title: string

flow:
  instances:
    unpack_draft:
      node: unpack_draft
      inputs:
        source: workflow_input.source
`), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	recorder := &stubRecorder{
		workflowStatuses: make(chan executionmodels.WorkflowExecutionStatus, 1),
	}
	api := &API{
		rt: &runtime.Runtime{},
		newRecorder: func() workflowruntime.ExecutionRecorder {
			return recorder
		},
	}

	response, err := api.Create(context.Background(), nil, &CreateRequest{
		File: workflowPath,
		Input: map[string]any{
			"source": "not an object",
		},
	})
	if err != nil {
		t.Fatalf("create workflow execution: %v", err)
	}

	if response.ExecutionID == "" {
		t.Fatal("expected execution id to be returned")
	}

	if response.Status != executionmodels.WorkflowExecutionStatusRunning {
		t.Fatalf("unexpected create status: %q", response.Status)
	}

	select {
	case status := <-recorder.workflowStatuses:
		if status != executionmodels.WorkflowExecutionStatusFailed {
			t.Fatalf("unexpected finished status: %q", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for detached workflow failure")
	}
}

func TestExecutionFailedErrorIncludesHarnessDetailsInErrorString(t *testing.T) {
	err := (&ExecutionFailedError{
		Details: ExecutionFailureDetails{
			ExecutionID:     "workflow-123",
			WorkflowID:      "plan-new-blueprint",
			NodeID:          "initial_plan_state_builder",
			NodeExecutionID: "node-456",
			ConversationID:  "conversation-789",
			HarnessExitCode: 1,
			HarnessError:    "failed to connect to codex backend",
		},
	}).Error()

	if !strings.Contains(err, "workflow-123") {
		t.Fatalf("expected execution id in error string: %q", err)
	}

	if !strings.Contains(err, "harness_exit_code=1") {
		t.Fatalf("expected harness exit code in error string: %q", err)
	}

	if !strings.Contains(err, "failed to connect to codex backend") {
		t.Fatalf("expected harness error in error string: %q", err)
	}
}
