package chatgpt

import (
	"context"
	"fmt"

	"github.com/vanclief/ez"
)

func (chatgpt *ChatGPT) ValidateModel(ctx context.Context, model string) error {
	const op = "ChatGPT.ValidateModel"

	// NOTE: Probably we can get rid of this method, check once we add another
	// LLM chatgpt

	if model == "" {
		return ez.New(op, ez.EINVALID, "model is required", nil)
	}

	// Uses the official SDK's Models service (Get) to verify the model ID.
	// Any 4xx/5xx from the API bubbles up here.
	_, err := chatgpt.client.Models.Get(ctx, model)
	if err != nil {
		errMsg := fmt.Sprintf("ChatGPT model %s does not exist", model)
		return ez.New(op, ez.EINVALID, errMsg, err)
	}
	return nil
}
