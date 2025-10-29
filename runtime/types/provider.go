package types

import (
	"context"
)

// TODO: I don't like this name as it conflicts with the agent.LLMProvider which
// is an enum

type LLMProvider interface {
	Chat(ctx context.Context, model string, request *ChatRequest) (ChatResponse, error)
	EstimateInputTokens(model string, messages []Message) (int, error)
	ValidateModel(ctx context.Context, model string) error
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
	TokenUsage         TokenUsage
}

type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens,omitempty"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens,omitempty"`
}
