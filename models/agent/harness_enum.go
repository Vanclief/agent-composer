package agent

import "github.com/vanclief/compose/primitives/enums"

type Harness string

const (
	HarnessCodexCLI   Harness = "codex_cli"
	HarnessClaudeCode Harness = "claude_code"
)

var harnessSet = enums.Set([]Harness{
	HarnessCodexCLI,
	HarnessClaudeCode,
})

func (e Harness) Validate() error {
	return enums.Validate(e, harnessSet)
}

func (e Harness) MarshalJSON() ([]byte, error) {
	return enums.Marshal(e, harnessSet)
}

func (e *Harness) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, e, harnessSet)
}
