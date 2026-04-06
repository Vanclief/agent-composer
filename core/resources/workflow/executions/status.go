package executions

import (
	"context"
	"time"

	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type StatusResponse struct {
	ExecutionID     string                                  `json:"execution_id"`
	WorkflowID      string                                  `json:"workflow_id"`
	WorkflowVersion string                                  `json:"workflow_version"`
	Status          executionmodels.WorkflowExecutionStatus `json:"status"`
	StartedAt       *time.Time                              `json:"started_at,omitempty"`
	FinishedAt      *time.Time                              `json:"finished_at,omitempty"`
	Output          map[string]any                          `json:"output,omitempty"`
	Failure         *WorkflowFailureDetails                 `json:"failure,omitempty"`
}

type WorkflowFailureDetails struct {
	NodeExecutionID string `json:"node_execution_id,omitempty"`
	NodeID          string `json:"node_id,omitempty"`
	NodeError       string `json:"node_error,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
	HarnessExitCode int    `json:"harness_exit_code,omitempty"`
	HarnessError    string `json:"harness_error,omitempty"`
}

func (api *API) Status(ctx context.Context, requester interface{}, request *GetRequest) (*StatusResponse, error) {
	const op = "workflow.executions.API.Status"

	record, err := api.Get(ctx, requester, request)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	response := &StatusResponse{
		ExecutionID:     record.ID.String(),
		WorkflowID:      record.WorkflowID,
		WorkflowVersion: record.WorkflowVersion,
		Status:          record.Status,
		StartedAt:       record.StartedAt,
		FinishedAt:      record.FinishedAt,
		Output:          record.OutputSnapshot,
	}

	if record.Status != executionmodels.WorkflowExecutionStatusFailed {
		return response, nil
	}

	failure, err := loadExecutionFailureDetails(ctx, api.db, ExecutionFailureDetails{
		ExecutionID:     record.ID.String(),
		WorkflowID:      record.WorkflowID,
		WorkflowVersion: record.WorkflowVersion,
	})
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	response.Failure = &WorkflowFailureDetails{
		NodeExecutionID: failure.NodeExecutionID,
		NodeID:          failure.NodeID,
		NodeError:       failure.NodeError,
		ConversationID:  failure.ConversationID,
		HarnessExitCode: failure.HarnessExitCode,
		HarnessError:    failure.HarnessError,
	}

	return response, nil
}
