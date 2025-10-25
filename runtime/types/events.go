package types

import (
	"github.com/google/uuid"
	"github.com/vanclief/compose/primitives/enums"
	"github.com/vanclief/compose/types"
)

type Event struct {
	ID             uint64            `json:"id"`
	Type           EventType         `json:"type"`
	ConversationID uuid.UUID         `json:"conversation_id"`
	AgentName      string            `json:"agent_name"`
	Timestamp      types.UnixSeconds `json:"timestamp"`
	Data           map[string]any    `json:"data"`
}

func New(eventType EventType, conversationID uuid.UUID, agentName string, data map[string]any) Event {
	return Event{
		Type:           eventType,
		ConversationID: conversationID,
		AgentName:      agentName,
		Data:           data,
	}
}

type EventType string

const (
	EventTypeConversationStarted EventType = "conversation_started"
	EventTypeConversationEnded   EventType = "conversation_ended"
	EventTypePreToolUse          EventType = "pre_tool_use"
	EventTypePostToolUse         EventType = "post_tool_use"
)

var evenTypeSet = enums.Set([]EventType{
	EventTypeConversationStarted,
	EventTypeConversationEnded,
	EventTypePreToolUse,
	EventTypePostToolUse,
})

func (e EventType) Validate() error {
	return enums.Validate(e, evenTypeSet)
}

func (e EventType) MarshalJSON() ([]byte, error) {
	return enums.Marshal(e, evenTypeSet)
}

func (e *EventType) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, e, evenTypeSet)
}
