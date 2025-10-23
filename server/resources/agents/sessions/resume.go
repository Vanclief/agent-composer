package sessions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type ResumeRequest struct {
	AgentSessionID uuid.UUID `json:"agent_session_id"`
	Prompt         string    `json:"prompt"`
}

func (r ResumeRequest) Validate() error {
	const op = "ResumeRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSessionID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Resume(ctx context.Context, requester interface{}, request *ResumeRequest) (uuid.UUID, error) {
	const op = "sessions.API.Resume"

	// Step 1: Get the agent session
	session, err := agent.GetAgentSessionByID(ctx, api.db, request.AgentSessionID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// Step 2: Launch

	// TODO: Permissions check
	instance, err := api.rt.NewAgentInstanceFromSession(ctx, session.ID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	api.rt.RunAgentInstance(instance, request.Prompt)

	return session.ID, nil
}
