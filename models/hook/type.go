package hook

import "github.com/vanclief/compose/primitives/enums"

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
