package executions

import (
	"context"

	executionmodels "github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

func (api *API) Start(ctx context.Context, requester interface{}, request *CreateRequest) (*StartResponse, error) {
	const op = "workflow.executions.API.Start"

	prepared, err := api.prepareExecution(request)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	handle, err := prepared.Executor.Start(ctx, prepared.Snapshot, request.Input)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	executionID := ""
	if handle != nil {
		executionID = handle.ID.String()
	}

	return &StartResponse{
		ExecutionID:     executionID,
		WorkflowID:      prepared.Snapshot.WorkflowID,
		WorkflowVersion: prepared.Snapshot.WorkflowVersion,
		Status:          executionmodels.WorkflowExecutionStatusRunning,
	}, nil
}
