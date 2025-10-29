package types

import (
	"encoding/json"
	"strings"
)

type ToolDefinition struct {
	Name        string
	Description string
	JSONSchema  map[string]any `bun:"-"`
}

type ToolCall struct {
	Name          string          // tool name the model asked for
	CallID        string          // call id to echo back when submitting results
	Arguments     string          // raw JSON string the model returned
	JSONArguments json.RawMessage // same as arguments but handy for re-marshaling
}

func (tc *ToolCall) CommandString() string {
	if tc == nil {
		return ""
	}

	if len(tc.JSONArguments) > 0 {

		var payload struct {
			Command string `json:"command"`
		}

		err := json.Unmarshal(tc.JSONArguments, &payload)
		if err == nil {
			cmd := strings.TrimSpace(payload.Command)
			if cmd != "" {
				return cmd
			}
		}
	}

	trimmed := strings.TrimSpace(tc.Arguments)
	if trimmed != "" {
		return trimmed
	}

	return tc.Name
}
