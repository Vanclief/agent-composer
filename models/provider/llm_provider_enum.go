package provider

import "github.com/vanclief/compose/primitives/enums"

type LLMProvider string

const (
	LLMProviderOpenAI LLMProvider = "open_ai"
)

var llmProviderSet = enums.Set([]LLMProvider{
	LLMProviderOpenAI,
})

func (e LLMProvider) Validate() error {
	return enums.Validate(e, llmProviderSet)
}

func (e LLMProvider) MarshalJSON() ([]byte, error) {
	return enums.Marshal(e, llmProviderSet)
}

func (e *LLMProvider) UnmarshalJSON(b []byte) error {
	return enums.Unmarshal(b, e, llmProviderSet)
}
