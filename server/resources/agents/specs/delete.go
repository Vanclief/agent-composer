package specs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	AgentSpecID uuid.UUID `json:"agent_spec_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*agent.Spec, error) {
	const op = "specs.API.Delete"

	// Step 1: Get the agent spec
	pt, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	// Step 2: Delete the agent spec
	err = pt.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}
