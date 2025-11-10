package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type ForkRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Prompt         string    `json:"prompt"`
}

func (r ForkRequest) Validate() error {
	const op = "ForkRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
		validation.Field(&r.Prompt, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Fork(ctx context.Context, requester interface{}, request *ForkRequest) (uuid.UUID, error) {
	const op = "conversations.API.Fork"

	// Step 1: Get the conversation
	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// Step 2: Create a new conversation from that OG conversation
	fork, err := conversation.Clone(ctx, api.db, false)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// Step 3: Launch the fork
	instance, err := api.rt.NewConversationInstance(ctx, fork.ID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	api.rt.RunConversationInstance(instance, request.Prompt)

	return fork.ID, nil
}
