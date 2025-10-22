package sessions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	AgentSessionID uuid.UUID `json:"agent_session_id"`
}

func (r GetRequest) Validate() error {
	const op = "GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSessionID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*agent.Session, error) {
	const op = "sessions.API.Get"

	// Step 1: Get the agent session
	session, err := agent.GetAgentSessionByID(ctx, api.db, request.AgentSessionID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	return session, nil
}
