package types

import "github.com/vanclief/compose/primitives/enums"

type ReasoningEffort string

const (
	ReasoningEffortHigh   ReasoningEffort = "high"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortLow    ReasoningEffort = "low"
)

var reasoningEffortSet = enums.Set([]ReasoningEffort{
	ReasoningEffortHigh,
	ReasoningEffortMedium,
	ReasoningEffortLow,
})

func (e ReasoningEffort) Validate() error {
	return enums.Validate(e, reasoningEffortSet)
}

func (e ReasoningEffort) MarshalJSON() ([]byte, error) {
	return enums.Marshal(e, reasoningEffortSet)
}

func (e *ReasoningEffort) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, e, reasoningEffortSet)
}
