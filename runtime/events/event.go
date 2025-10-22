package events

import (
	"github.com/google/uuid"
	"github.com/vanclief/compose/primitives/enums"
	"github.com/vanclief/compose/types"
)

type Event struct {
	ID             uint64            `json:"id"`
	Type           EventType         `json:"type"`
	AgentSessionID uuid.UUID         `json:"agent_session_id"`
	AgentName      string            `json:"agent_name"`
	Timestamp      types.UnixSeconds `json:"timestamp"`
	Data           map[string]any    `json:"data"`
}

func New(eventType EventType, agentSessionID uuid.UUID, agentName string, data map[string]any) Event {
	return Event{
		Type:           eventType,
		AgentSessionID: agentSessionID,
		AgentName:      agentName,
		Data:           data,
	}
}

type EventType string

const (
	EventTypeSessionStarted EventType = "session_started"
	EventTypeSessionEnded   EventType = "session_ended"
	EventTypePreToolUse     EventType = "pre_tool_use"
	EventTypePostToolUse    EventType = "post_tool_use"
)

var evenTypeSet = enums.Set([]EventType{
	EventTypeSessionStarted,
	EventTypeSessionEnded,
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
