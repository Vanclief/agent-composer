package nodeexecutions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/execution"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	NodeExecutionID uuid.UUID `json:"node_execution_id"`
}

func (r GetRequest) Validate() error {
	const op = "workflow.nodeexecutions.GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.NodeExecutionID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*execution.NodeExecution, error) {
	const op = "workflow.nodeexecutions.API.Get"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	record, err := execution.GetNodeExecutionByID(ctx, api.db, request.NodeExecutionID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return record, nil
}
