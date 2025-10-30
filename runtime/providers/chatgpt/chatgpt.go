package chatgpt

import (
	"github.com/openai/openai-go"
	"github.com/vanclief/agent-composer/runtime/types"
)

type ChatGPT struct {
	client              *openai.Client
	responsesToMessages map[string]int
}

func New(client *openai.Client) (types.LLMProvider, error) {
	gpt := &ChatGPT{client: client, responsesToMessages: make(map[string]int)}

	return gpt, nil
}
