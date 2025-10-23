package sessions

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	AgentSpecID      uuid.UUID `json:"agent_spec_id"`
	Prompt           string    `json:"prompt"`
	ParallelSessions int       `json:"parallel_sessions"`
}

func (r CreateRequest) Validate() error {
	const op = "CreateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
		validation.Field(&r.Prompt, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type CreateResponse struct {
	IDs     []uuid.UUID `json:"ids,omitempty"`
	Success bool        `json:"success"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	const op = "sessions.API.Create"

	// Step 1: Get the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	if request.ParallelSessions < 1 {
		request.ParallelSessions = 1
	}

	instances := make([]*runtime.AgentInstance, 0, request.ParallelSessions)

	for i := 0; i < request.ParallelSessions; i++ {
		instance, err := api.rt.NewAgentInstanceFromSpec(ctx, spec.ID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		api.rt.RunAgentInstance(instance, request.Prompt)

		instances = append(instances, instance)
	}

	ids := make([]uuid.UUID, len(instances))
	for i, instance := range instances {
		ids[i] = instance.ID
	}

	return &CreateResponse{IDs: ids, Success: true}, nil
}
