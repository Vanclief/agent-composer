package agent

import "github.com/vanclief/compose/primitives/enums"

type SessionStatus string

const (
	SessionStatusQueued    SessionStatus = "queued"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusPaused    SessionStatus = "paused"
	SessionStatusSucceeded SessionStatus = "succeeded"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCanceled  SessionStatus = "canceled"
)

var sessionStatusSet = enums.Set([]SessionStatus{
	SessionStatusQueued,
	SessionStatusRunning,
	SessionStatusPaused,
	SessionStatusSucceeded,
	SessionStatusFailed,
	SessionStatusCanceled,
})

func (s SessionStatus) Validate() error {
	return enums.Validate(s, sessionStatusSet)
}

func (s SessionStatus) MarshalJSON() ([]byte, error) {
	return enums.Marshal(s, sessionStatusSet)
}

func (s *SessionStatus) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, s, sessionStatusSet)
}

func (s SessionStatus) IsTerminal() bool {
	switch s {
	case SessionStatusSucceeded, SessionStatusFailed, SessionStatusCanceled:
		return true
	default:
		return false
	}
}

func (s SessionStatus) InProgress() bool {
	switch s {
	case SessionStatusQueued, SessionStatusRunning, SessionStatusPaused:
		return true
	default:
		return false
	}
}
