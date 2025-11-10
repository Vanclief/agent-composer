package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	AgentSpecID           uuid.UUID `json:"agent_spec_id"`
	Prompt                string    `json:"prompt"`
	ParallelConversations int       `json:"parallel_conversations"`
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
	Conversations []ConversationID `json:"conversations"`
}

type ConversationID struct {
	ID uuid.UUID `json:"id"`
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*CreateResponse, error) {
	const op = "conversations.API.Create"

	// Step 1: Get the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	if request.ParallelConversations < 1 {
		request.ParallelConversations = 1
	}

	instances := make([]*runtime.ConversationInstance, 0, request.ParallelConversations)

	for i := 0; i < request.ParallelConversations; i++ {
		instance, err := api.rt.NewConversationInstanceFromSpec(ctx, spec.ID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		api.rt.RunConversationInstance(instance, request.Prompt)

		instances = append(instances, instance)
	}

	response := &CreateResponse{
		Conversations: make([]ConversationID, len(instances)),
	}

	for i, instance := range instances {
		response.Conversations[i] = ConversationID{ID: instance.ID}
	}

	return response, nil
}
