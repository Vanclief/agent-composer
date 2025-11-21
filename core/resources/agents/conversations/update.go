package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type UpdateRequest struct {
	ConversationID uuid.UUID       `json:"conversation_id"`
	Metadata       *map[string]any `json:"metadata"`
}

func (r UpdateRequest) Validate() error {
	const op = "UpdateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	if r.Metadata == nil {
		return ez.New(op, ez.EINVALID, "No fields to update", nil)
	}

	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*agent.Conversation, error) {
	const op = "conversations.API.Update"

	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	conversation.Metadata = cloneMetadata(*request.Metadata)

	err = conversation.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return conversation, nil
}

func cloneMetadata(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}

	return dst
}
