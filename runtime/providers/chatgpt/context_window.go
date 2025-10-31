package chatgpt

import (
	"fmt"
	"strings"

	"github.com/vanclief/ez"
)

func (gpt *ChatGPT) CheckContextWindow(model string, totalInputTokens, compactAtPercent int) error {
	maxTokens, ok := ctxWindow[strings.ToLower(model)]

	if ok && totalInputTokens > (maxTokens*compactAtPercent/100) {
		errMsg := fmt.Sprintf("Input tokens %d exceed context window %d for model %s", totalInputTokens, maxTokens, model)
		return ez.New("ChatGPT.CheckContextWindow", ez.EINVALID, errMsg, nil)
	}

	return nil
}

var ctxWindow = map[string]int{
	"gpt-5": 400000,
	"o3":    200000,
}
