package llm

import "encoding/json"

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

type ChatRequest struct {
	Messages           []Message
	Tools              []ToolDefinition
	ThinkingEffort     string
	PreviousResponseID string
}

type ChatResponse struct {
	ID                 string
	Text               string
	Model              string
	ToolCalls          []ToolCall
	PreviousResponseID string
}
