package hook

import "github.com/vanclief/compose/primitives/enums"

type EventType string

const (
	EventTypeConversationStarted   EventType = "conversation_started"
	EventTypeConversationEnded     EventType = "conversation_ended"
	EventTypeContextExceeded       EventType = "context_exceeded" // TODO: consider deprecating
	EventTypePreContextCompaction  EventType = "pre_context_compaction"
	EventTypePostContextCompaction EventType = "post_context_compaction"
	EventTypePreToolUse            EventType = "pre_tool_use"
	EventTypePostToolUse           EventType = "post_tool_use"
)

var evenTypeSet = enums.Set([]EventType{
	EventTypeConversationStarted,
	EventTypeConversationEnded,
	EventTypeContextExceeded,
	EventTypePreContextCompaction,
	EventTypePostContextCompaction,
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
