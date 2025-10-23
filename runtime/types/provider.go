package types

import "context"

// TODO: I don't like this name as it conflicts with the agent.LLMProvider which
// is an enum

type LLMProvider interface {
	Chat(ctx context.Context, model string, request *ChatRequest) (ChatResponse, error)
	ValidateModel(ctx context.Context, model string) error
}
