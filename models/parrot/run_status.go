package parrot

import "github.com/vanclief/compose/primitives/enums"

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusPaused    RunStatus = "paused"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

var runStatusSet = enums.Set([]RunStatus{
	RunStatusQueued,
	RunStatusRunning,
	RunStatusPaused,
	RunStatusSucceeded,
	RunStatusFailed,
	RunStatusCanceled,
})

func (s RunStatus) Validate() error {
	return enums.Validate(s, runStatusSet)
}

func (s RunStatus) MarshalJSON() ([]byte, error) {
	return enums.Marshal(s, runStatusSet)
}

func (s *RunStatus) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, s, runStatusSet)
}

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCanceled:
		return true
	default:
		return false
	}
}

func (s RunStatus) InProgress() bool {
	switch s {
	case RunStatusQueued, RunStatusRunning, RunStatusPaused:
		return true
	default:
		return false
	}
}
