package harnesses

import (
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/types"
)

func buildInitialPrompt(conversation *agent.Conversation) string {
	if len(conversation.Messages) == 0 {
		return strings.TrimSpace(conversation.Instructions)
	}

	var b strings.Builder

	b.WriteString("Continue this Agent Composer conversation.\n")
	b.WriteString("Treat system messages as instructions and respond to the latest user request.\n\n")
	b.WriteString("Conversation transcript:\n")

	for _, msg := range conversation.Messages {
		b.WriteString(renderTranscriptMessage(msg))
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func renderTranscriptMessage(msg types.Message) string {
	var b strings.Builder

	b.WriteString("<message role=\"")
	b.WriteString(string(msg.Role))
	b.WriteString("\"")

	if msg.Name != "" {
		b.WriteString(" name=\"")
		b.WriteString(msg.Name)
		b.WriteString("\"")
	}

	if msg.ToolCallID != "" {
		b.WriteString(" tool_call_id=\"")
		b.WriteString(msg.ToolCallID)
		b.WriteString("\"")
	}

	b.WriteString(">\n")

	content := strings.TrimSpace(msg.Content)
	if content != "" {
		b.WriteString(content)
		b.WriteString("\n")
	}

	if msg.ToolCall != nil {
		b.WriteString("tool_name: ")
		b.WriteString(msg.ToolCall.Name)
		b.WriteString("\n")
		if strings.TrimSpace(msg.ToolCall.Arguments) != "" {
			b.WriteString("tool_arguments: ")
			b.WriteString(msg.ToolCall.Arguments)
			b.WriteString("\n")
		}
	}

	b.WriteString("</message>")

	return b.String()
}
