package chatgpt

import (
	"encoding/json"
	"strings"

	"github.com/pkoukk/tiktoken-go"
	"github.com/vanclief/agent-composer/runtime/types"
)

func (gpt *ChatGPT) EstimateInputTokens(model string, messages []types.Message) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	simulatedPayload := simulatePayload(messages)
	encodingName := encodingForModel(model)

	tke, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return 0, err
	}

	return len(tke.Encode(simulatedPayload, nil, nil)), nil
}

func simulatePayload(messages []types.Message) string {
	var b strings.Builder

	b.WriteString("<|begin_of_text|>\n")

	for _, msg := range messages {
		role := string(msg.Role)
		if msg.Role == types.MessageRoleTool {
			role = "tool"
		}

		b.WriteString("<|start_header_id|>")
		b.WriteString(role)
		b.WriteString("<|end_header_id|>\n")

		switch msg.Role {
		case types.MessageRoleSystem, types.MessageRoleUser, types.MessageRoleAssistant:
			if msg.ToolCall != nil {
				writeSimulatedFunctionCall(&b, msg.ToolCall)
			} else {
				b.WriteString(msg.Content)
				if !strings.HasSuffix(msg.Content, "\n") {
					b.WriteString("\n")
				}
			}
		case types.MessageRoleTool:
			writeSimulatedToolOutput(&b, msg.ToolCallID, msg.Content)
		default:
			// Unsupported roles are ignored.
		}

		b.WriteString("<|eot_id|>\n")
	}

	return b.String()
}

func writeSimulatedFunctionCall(b *strings.Builder, call *types.ToolCall) {
	payload := struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{
		Type:   "function_call",
		CallID: call.CallID,
		Name:   call.Name,
	}

	switch {
	case call.Arguments != "":
		payload.Arguments = call.Arguments
	case len(call.JSONArguments) > 0:
		payload.Arguments = string(call.JSONArguments)
	default:
		payload.Arguments = "{}"
	}

	writeSimulatedJSON(b, payload)
}

func writeSimulatedToolOutput(b *strings.Builder, callID, content string) {
	payload := struct {
		Type   string `json:"type"`
		CallID string `json:"call_id"`
		Output string `json:"output"`
	}{
		Type:   "function_call_output",
		CallID: callID,
		Output: content,
	}

	writeSimulatedJSON(b, payload)
}

func writeSimulatedJSON(b *strings.Builder, payload any) {
	b.WriteString("<|json.start|>")

	jsonPayload, err := json.Marshal(payload)
	if err == nil {
		b.Write(jsonPayload)
	} else {
		b.WriteString("{}")
	}

	b.WriteString("<|json.end|>\n")
}

func encodingForModel(model string) string {
	modelLower := strings.ToLower(model)

	switch {
	case strings.HasPrefix(modelLower, "gpt-5"),
		strings.HasPrefix(modelLower, "gpt-4o"),
		strings.HasPrefix(modelLower, "gpt-4.1"),
		strings.HasPrefix(modelLower, "o1"),
		strings.HasPrefix(modelLower, "o3"),
		strings.HasPrefix(modelLower, "o4"),
		strings.Contains(modelLower, "mini"),
		strings.Contains(modelLower, "small"),
		strings.Contains(modelLower, "large"):
		return "o200k_base"
	default:
		return "cl100k_base"
	}
}
