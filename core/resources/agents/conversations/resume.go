package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type ResumeRequest struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Prompt         string    `json:"prompt"`
}

func (r ResumeRequest) Validate() error {
	const op = "ResumeRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ConversationID, validation.Required),
		validation.Field(&r.Prompt, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Resume(ctx context.Context, requester interface{}, request *ResumeRequest) (uuid.UUID, error) {
	const op = "conversations.API.Resume"

	// Step 1: Get the conversation
	conversation, err := agent.GetConversationByID(ctx, api.db, request.ConversationID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// Step 2: Launch

	// TODO: Permissions check
	instance, err := api.rt.NewConversationInstance(ctx, conversation.ID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	api.rt.RunConversationInstance(instance, request.Prompt)

	return conversation.ID, nil
}
