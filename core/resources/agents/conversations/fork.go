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
}

func (r ForkRequest) Validate() error {
	const op = "ForkRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Fork(ctx context.Context, requester interface{}, request *ForkRequest) (*agent.Conversation, error) {
	const op = "conversations.API.Fork"

	// Step 1: Get the conversation
	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2: Create a new conversation from that OG conversation
	fork := conversation
	fork.ID = uuid.Nil // Reset ID for new insert

	err = fork.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return conversation, nil
}
