package workflow

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	executionmodels "github.com/vanclief/agent-composer/models/execution"
)

// RunObserver receives coarse progress while a workflow runs — one
// callback pair per top-level node. The CLI prints these; servers
// have no observer.
type RunObserver interface {
	NodeStarted(instanceID string)
	NodeFinished(instanceID string, status executionmodels.NodeExecutionStatus)
}

// ObservedRecorder decorates an ExecutionRecorder with RunObserver
// callbacks. Recording stays the inner recorder's job — the observer
// only watches, so a failing print can never corrupt a run. Child
// scopes (loop iterations, branches) are deliberately not reported.
type ObservedRecorder struct {
	Inner    ExecutionRecorder
	Observer RunObserver

	mu sync.Mutex
	// names correlates the inner recorder's node handles back to
	// instance ids, which FinishNode alone does not carry.
	names map[uuid.UUID]string
}

func (r *ObservedRecorder) StartWorkflow(ctx context.Context, snapshot *Snapshot, input map[string]any, projectDir string) (WorkflowExecutionHandle, error) {
	return r.Inner.StartWorkflow(ctx, snapshot, input, projectDir)
}

func (r *ObservedRecorder) FinishWorkflow(ctx context.Context, handle WorkflowExecutionHandle, output map[string]any, status executionmodels.WorkflowExecutionStatus) error {
	return r.Inner.FinishWorkflow(ctx, handle, output, status)
}

func (r *ObservedRecorder) StartNode(ctx context.Context, workflow WorkflowExecutionHandle, node NodeSnapshot, input map[string]any, scope NodeExecutionScope) (NodeExecutionHandle, error) {
	handle, err := r.Inner.StartNode(ctx, workflow, node, input, scope)
	if err != nil {
		return handle, err
	}

	if scope.ParentNodeExecutionID == uuid.Nil {
		r.mu.Lock()
		if r.names == nil {
			r.names = map[uuid.UUID]string{}
		}
		r.names[handle.ID] = node.InstanceID
		r.mu.Unlock()

		r.Observer.NodeStarted(node.InstanceID)
	}

	return handle, nil
}

func (r *ObservedRecorder) FinishNode(ctx context.Context, handle NodeExecutionHandle, output map[string]any, status executionmodels.NodeExecutionStatus, trace map[string]any) error {
	r.mu.Lock()
	instanceID, found := r.names[handle.ID]
	if found {
		delete(r.names, handle.ID)
	}
	r.mu.Unlock()

	if found {
		r.Observer.NodeFinished(instanceID, status)
	}

	return r.Inner.FinishNode(ctx, handle, output, status, trace)
}

func (r *ObservedRecorder) StartConversation(ctx context.Context, handle NodeExecutionHandle, conversation *agent.Conversation, input map[string]any) error {
	return r.Inner.StartConversation(ctx, handle, conversation, input)
}

func (r *ObservedRecorder) FinishConversation(ctx context.Context, conversation *agent.Conversation, output any) error {
	return r.Inner.FinishConversation(ctx, conversation, output)
}
