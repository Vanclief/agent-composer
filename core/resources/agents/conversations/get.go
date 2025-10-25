package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type GetRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
}

func (r GetRequest) Validate() error {
	const op = "GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*agent.Conversation, error) {
	const op = "conversations.API.Get"

	// Step 1: Get the conversation
	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check
	return conversation, nil
}
