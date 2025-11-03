package runtime

import (
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/mcp"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	types "github.com/vanclief/agent-composer/runtime/types"
)

type AgentInstance struct {
	ID                     uuid.UUID
	conversation           *agent.Conversation
	provider               types.LLMProvider
	name                   string
	model                  string
	instructions           string
	reasoningEffort        types.ReasoningEffort
	mcpMux                 *mcp.Mux
	tools                  []types.ToolDefinition
	messages               []types.Message
	hooks                  map[hook.EventType][]hook.Hook
	compactAtPercent       int
	autoCompact            bool
	compactionPrompt       string
	shellAccess            bool
	webSearch              bool
	structuredOutput       bool
	structuredOutputSchema map[string]any
}

func (ai *AgentInstance) LatestAssistantMessage() (*types.Message, bool) {
	for i := len(ai.messages) - 1; i >= 0; i-- {
		if ai.messages[i].Role == types.MessageRoleAssistant && ai.messages[i].ToolCall == nil {
			return &ai.messages[i], true
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

	ai.messages = append(ai.messages, msg)
}

func (ai *AgentInstance) AddToolMessage(toolName, toolCallID, content string) {
	msg := *types.NewToolMessage(toolName, toolCallID, content)
	ai.messages = append(ai.messages, msg)
}

func (ai *AgentInstance) AddAssistantToolCall(toolCall types.ToolCall) {
	msg := *types.NewAssistantToolCallMessage(toolCall)
	ai.messages = append(ai.messages, msg)
}
