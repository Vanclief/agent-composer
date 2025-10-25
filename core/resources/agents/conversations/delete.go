package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (uuid.UUID, error) {
	const op = "conversations.API.Delete"

	// Step 1: Get the conversation
	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	// Step 3: Delete the conversation
	err = conversation.Delete(ctx, api.db)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	return conversation.ID, nil
}
