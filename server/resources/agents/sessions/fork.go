package sessions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type ForkRequest struct {
	AgentSessionID uuid.UUID `json:"agent_session_id"`
}

func (r ForkRequest) Validate() error {
	const op = "ForkRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSessionID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Fork(ctx context.Context, requester interface{}, request *ForkRequest) (*agent.Session, error) {
	const op = "sessions.API.Fork"

	// Step 1: Get the agent session
	session, err := agent.GetAgentSessionByID(ctx, api.db, request.AgentSessionID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2: Create a new session from that OG session
	fork := session
	fork.ID = uuid.Nil // Reset ID for new insert

	err = fork.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return session, nil
}
