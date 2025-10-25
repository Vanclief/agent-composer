package agent

import "github.com/vanclief/compose/primitives/enums"

type ConversationStatus string

const (
	ConversationStatusQueued    ConversationStatus = "queued"
	ConversationStatusRunning   ConversationStatus = "running"
	ConversationStatusPaused    ConversationStatus = "paused"
	ConversationStatusSucceeded ConversationStatus = "succeeded"
	ConversationStatusFailed    ConversationStatus = "failed"
	ConversationStatusCanceled  ConversationStatus = "canceled"
)

var conversationStatusSet = enums.Set([]ConversationStatus{
	ConversationStatusQueued,
	ConversationStatusRunning,
	ConversationStatusPaused,
	ConversationStatusSucceeded,
	ConversationStatusFailed,
	ConversationStatusCanceled,
})

func (s ConversationStatus) Validate() error {
	return enums.Validate(s, conversationStatusSet)
}

func (s ConversationStatus) MarshalJSON() ([]byte, error) {
	return enums.Marshal(s, conversationStatusSet)
}

func (s *ConversationStatus) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, s, conversationStatusSet)
}
