package executions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	WorkflowExecutionID uuid.UUID `json:"workflow_execution_id"`
}

func (r GetRequest) Validate() error {
	err := validation.ValidateStruct(&r,
		validation.Field(&r.WorkflowExecutionID, validation.Required),
	)
	if err != nil {
		return ez.New(ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*execution.WorkflowExecution, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := execution.GetWorkflowExecutionByID(ctx, api.db, request.WorkflowExecutionID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}
