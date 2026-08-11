package conversations

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	"github.com/vanclief/ez"
)

type ListRequest struct {
	NodeExecutionID uuid.UUID `json:"node_execution_id"`
}

func (r *ListRequest) Validate() error {
	err := validation.ValidateStruct(r,
		validation.Field(&r.NodeExecutionID, validation.Required),
	)
	if err != nil {
		return ez.New(ez.EINVALID, err.Error(), nil)
	}

	return nil
}

// ConversationWithTrace decorates a conversation with the step-by-step
// trace parsed from its raw harness output.
type ConversationWithTrace struct {
	*agent.Conversation
	Trace []harnesses.TraceEvent `json:"trace,omitempty"`
}

type ListResponse struct {
	Conversations []ConversationWithTrace `json:"conversations"`
}

func (api *API) List(ctx context.Context, requester interface{}, request *ListRequest) (*ListResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	items, err := agent.GetConversationsByNodeExecutionID(ctx, api.db, request.NodeExecutionID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	conversations := make([]ConversationWithTrace, 0, len(items))
	for _, item := range items {
		conversations = append(conversations, ConversationWithTrace{
			Conversation: item,
			Trace: harnesses.ParseTrace(
				item.Harness,
				item.RawHarnessOutput,
			),
		})
	}

	return &ListResponse{Conversations: conversations}, nil
}
