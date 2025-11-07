package runtime

import (
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/mcp"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	types "github.com/vanclief/agent-composer/runtime/types"
)

type AgentInstance struct {
	*agent.Conversation
	provider types.LLMProvider
	mcpMux   *mcp.Mux
	hooks    map[hook.EventType][]hook.Hook
}

func (ai *AgentInstance) LatestAssistantMessage() (*types.Message, bool) {
	for i := len(ai.Messages) - 1; i >= 0; i-- {
		if ai.Messages[i].Role == types.MessageRoleAssistant && ai.Messages[i].ToolCall == nil {
			return &ai.Messages[i], true
		}
	}

	return nil, false
}

func (ai *AgentInstance) AddMessage(role types.MessageRole, content string) {
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

	ai.Messages = append(ai.Messages, msg)
}

func (ai *AgentInstance) AddToolMessage(toolName, toolCallID, content string) {
	msg := *types.NewToolMessage(toolName, toolCallID, content)
	ai.Messages = append(ai.Messages, msg)
}

func (ai *AgentInstance) AddAssistantToolCall(toolCall types.ToolCall) {
	msg := *types.NewAssistantToolCallMessage(toolCall)
	ai.Messages = append(ai.Messages, msg)
}
