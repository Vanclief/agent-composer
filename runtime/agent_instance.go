package runtime

import (
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/mcp"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	types "github.com/vanclief/agent-composer/runtime/types"
)

type ConversationInstance struct {
	*agent.Conversation
	provider types.LLMProvider
	mcpMux   *mcp.Mux
	hooks    map[hook.EventType][]hook.Hook
}

func (ci *ConversationInstance) LatestAssistantMessage() (*types.Message, bool) {
	for i := len(ci.Messages) - 1; i >= 0; i-- {
		if ci.Messages[i].Role == types.MessageRoleAssistant && ci.Messages[i].ToolCall == nil {
			return &ci.Messages[i], true
		}
	}

	return nil, false
}

func (ci *ConversationInstance) AddMessage(role types.MessageRole, content string) {
	var msg types.Message

	switch role {
	case types.MessageRoleSystem:
		msg = *types.NewSystemMessage(content)
	case types.MessageRoleUser:
		msg = *types.NewUserMessage(content)
	case types.MessageRoleAssistant:
		msg = *types.NewAssistantMessage(content)
	default:
		log.Error().Msg("Invalid message role")
		return // Invalid role; do nothing
	}

	ci.Messages = append(ci.Messages, msg)
}

func (ci *ConversationInstance) AddToolMessage(toolName, toolCallID, content string) {
	msg := *types.NewToolMessage(toolName, toolCallID, content)
	ci.Messages = append(ci.Messages, msg)
}

func (ci *ConversationInstance) AddAssistantToolCall(toolCall types.ToolCall) {
	msg := *types.NewAssistantToolCallMessage(toolCall)
	ci.Messages = append(ci.Messages, msg)
}
