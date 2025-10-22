package specs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type SessionRequest struct {
	AgentSpecID      uuid.UUID `json:"agent_spec_id"`
	Prompt           string    `json:"prompt"`
	Model            string    `json:"model"`
	ParallelSessions int       `json:"parallel_sessions"`
}

func (r SessionRequest) Validate() error {
	const op = "SessionRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
		validation.Field(&r.Prompt, validation.Required),
		validation.Field(&r.Model, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type SessionResponse struct {
	IDs     []uuid.UUID `json:"ids,omitempty"`
	Success bool        `json:"success"`
}

func (api *API) StartSessions(ctx context.Context, requester interface{}, request *SessionRequest) (*SessionResponse, error) {
	const op = "specs.API.StartSessions"

	// Step 1: Get the agent spec
	pt, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	if request.ParallelSessions == 0 {
		request.ParallelSessions = 1
	}

	sessions, err := api.orchestrator.CreateSessions(ctx, pt.ID, request.ParallelSessions)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	api.orchestrator.RunSessions(sessions, request.Prompt)

	ids := make([]uuid.UUID, len(sessions))
	for i, session := range sessions {
		ids[i] = session.ID
	}

	return &SessionResponse{IDs: ids, Success: true}, nil
}
