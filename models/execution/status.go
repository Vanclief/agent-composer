package execution

import "github.com/vanclief/compose/primitives/enums"

type WorkflowExecutionStatus string

const (
	WorkflowExecutionStatusQueued    WorkflowExecutionStatus = "queued"
	WorkflowExecutionStatusRunning   WorkflowExecutionStatus = "running"
	WorkflowExecutionStatusSucceeded WorkflowExecutionStatus = "succeeded"
	WorkflowExecutionStatusFailed    WorkflowExecutionStatus = "failed"
	WorkflowExecutionStatusCanceled  WorkflowExecutionStatus = "canceled"
	WorkflowExecutionStatusBlocked   WorkflowExecutionStatus = "blocked"
)

var workflowExecutionStatusSet = enums.Set([]WorkflowExecutionStatus{
	WorkflowExecutionStatusQueued,
	WorkflowExecutionStatusRunning,
	WorkflowExecutionStatusSucceeded,
	WorkflowExecutionStatusFailed,
	WorkflowExecutionStatusCanceled,
	WorkflowExecutionStatusBlocked,
})

func (s WorkflowExecutionStatus) Validate() error {
	return enums.Validate(s, workflowExecutionStatusSet)
}

func (s WorkflowExecutionStatus) MarshalJSON() ([]byte, error) {
	return enums.Marshal(s, workflowExecutionStatusSet)
}

func (s *WorkflowExecutionStatus) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, s, workflowExecutionStatusSet)
}

type NodeExecutionStatus string

const (
	NodeExecutionStatusQueued    NodeExecutionStatus = "queued"
	NodeExecutionStatusRunning   NodeExecutionStatus = "running"
	NodeExecutionStatusSucceeded NodeExecutionStatus = "succeeded"
	NodeExecutionStatusFailed    NodeExecutionStatus = "failed"
	NodeExecutionStatusCanceled  NodeExecutionStatus = "canceled"
	NodeExecutionStatusBlocked   NodeExecutionStatus = "blocked"
)

var nodeExecutionStatusSet = enums.Set([]NodeExecutionStatus{
	NodeExecutionStatusQueued,
	NodeExecutionStatusRunning,
	NodeExecutionStatusSucceeded,
	NodeExecutionStatusFailed,
	NodeExecutionStatusCanceled,
	NodeExecutionStatusBlocked,
})

func (s NodeExecutionStatus) Validate() error {
	return enums.Validate(s, nodeExecutionStatusSet)
}

func (s NodeExecutionStatus) MarshalJSON() ([]byte, error) {
	return enums.Marshal(s, nodeExecutionStatusSet)
}

func (s *NodeExecutionStatus) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, s, nodeExecutionStatusSet)
}
