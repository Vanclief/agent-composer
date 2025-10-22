package sessions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	AgentSessionID uuid.UUID `json:"agent_session_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSessionID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*agent.Session, error) {
	const op = "sessions.API.Delete"

	// Step 1: Get the session
	session, err := agent.GetAgentSessionByID(ctx, api.db, request.AgentSessionID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	// Step 3: Delete the agent session
	err = session.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return session, nil
}
