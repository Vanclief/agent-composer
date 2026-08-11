package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
)

type observerEvents struct {
	started  []string
	finished []string
}

func (o *observerEvents) NodeStarted(instanceID string) {
	o.started = append(o.started, instanceID)
}

func (o *observerEvents) NodeFinished(instanceID string, status executionmodels.NodeExecutionStatus) {
	o.finished = append(o.finished, instanceID+":"+string(status))
}

func TestObservedRecorderReportsTopLevelNodes(t *testing.T) {
	inner := &stubObserverInner{}
	events := &observerEvents{}
	recorder := &ObservedRecorder{Inner: inner, Observer: events}

	ctx := context.Background()
	workflow := WorkflowExecutionHandle{ID: uuid.New()}

	topLevel, err := recorder.StartNode(ctx, workflow, NodeSnapshot{InstanceID: "review_a"}, nil, NodeExecutionScope{})
	if err != nil {
		t.Fatal(err)
	}

	// A loop iteration has a parent scope and must stay silent.
	child, err := recorder.StartNode(ctx, workflow, NodeSnapshot{InstanceID: "iteration"}, nil, NodeExecutionScope{
		ParentNodeExecutionID: topLevel.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.FinishNode(ctx, child, nil, executionmodels.NodeExecutionStatusSucceeded, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.FinishNode(ctx, topLevel, nil, executionmodels.NodeExecutionStatusSucceeded, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(events.started) != 1 || events.started[0] != "review_a" {
		t.Fatalf("expected only the top-level start, got %v", events.started)
	}
	if len(events.finished) != 1 || events.finished[0] != "review_a:succeeded" {
		t.Fatalf("expected only the top-level finish, got %v", events.finished)
	}
	if inner.startedNodes != 2 || inner.finishedNodes != 2 {
		t.Fatalf("the inner recorder must see everything: %d/%d", inner.startedNodes, inner.finishedNodes)
	}
}

type stubObserverInner struct {
	startedNodes  int
	finishedNodes int
}

func (s *stubObserverInner) StartWorkflow(ctx context.Context, snapshot *Snapshot, input map[string]any, projectDir string) (WorkflowExecutionHandle, error) {
	return WorkflowExecutionHandle{ID: uuid.New()}, nil
}

func (s *stubObserverInner) FinishWorkflow(ctx context.Context, handle WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error {
	return nil
}

func (s *stubObserverInner) StartNode(ctx context.Context, workflow WorkflowExecutionHandle, node NodeSnapshot, input map[string]any, scope NodeExecutionScope) (NodeExecutionHandle, error) {
	s.startedNodes++
	return NodeExecutionHandle{ID: uuid.New()}, nil
}

func (s *stubObserverInner) FinishNode(ctx context.Context, handle NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error {
	s.finishedNodes++
	return nil
}

func (s *stubObserverInner) StartConversation(ctx context.Context, handle NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error {
	return nil
}

func (s *stubObserverInner) FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error {
	return nil
}
