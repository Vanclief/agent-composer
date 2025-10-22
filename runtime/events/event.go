package events

import (
	"github.com/google/uuid"
	"github.com/vanclief/compose/primitives/enums"
	"github.com/vanclief/compose/types"
)

type Event struct {
	ID          uint64            `json:"id"`
	Type        EventType         `json:"type"`
	ParrotRunID uuid.UUID         `json:"parrot_run_id"`
	ParrotName  string            `json:"parrot_name"`
	Timestamp   types.UnixSeconds `json:"timestamp"`
	Data        map[string]any    `json:"data"`
}

func New(eventType EventType, parrotRunID uuid.UUID, parrotName string, data map[string]any) Event {
	return Event{
		Type:        eventType,
		ParrotRunID: parrotRunID,
		ParrotName:  parrotName,
		Data:        data,
	}
}

type EventType string

const (
	EventTypeRunStarted  EventType = "run_started"
	EventTypeRunEnded    EventType = "run_ended"
	EventTypePreToolUse  EventType = "pre_tool_use"
	EventTypePostToolUse EventType = "post_tool_use"
)

var evenTypeSet = enums.Set([]EventType{
	EventTypeRunStarted,
	EventTypeRunEnded,
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
