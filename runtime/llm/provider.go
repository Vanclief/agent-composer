package llm

import "context"

type Provider interface {
	Chat(ctx context.Context, model string, request *ChatRequest) (ChatResponse, error)
	ValidateModel(ctx context.Context, model string) error
}
