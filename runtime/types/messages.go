package types

// MessageRole represents the role of a message in the conversation.
type MessageRole string

const (
	// MessageRoleSystem is used for system-level instructions.
	MessageRoleSystem MessageRole = "system"
	// MessageRoleUser is used for user-originated messages.
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant is used for assistant (model) responses.
	MessageRoleAssistant MessageRole = "assistant"
	// MessageRoleTool is used for messages coming from tool executions.
	MessageRoleTool MessageRole = "tool"
)

// Message is a provider-agnostic representation of a single chat turn or tool result.
type Message struct {
	Role       MessageRole // Who sent the message
	Content    string      // The text or payload of the message
	Name       string      // Optional: tool name or function name
	ToolCallID string      // Optional: maps back to the provider's call identifier
	ToolCall   *ToolCall   // Optional: captures assistant-issued tool calls
}

func NewMessage(role MessageRole, content string) *Message {
	return &Message{
		Role:    role,
		Content: content,
	}
}

// NewSystemMessage creates a system role message with given content.
func NewSystemMessage(content string) *Message {
	return &Message{
		Role:    MessageRoleSystem,
		Content: content,
	}
}

// NewUserMessage creates a user role message with given content.
func NewUserMessage(content string) *Message {
	return &Message{
		Role:    MessageRoleUser,
		Content: content,
	}
}

// NewAssistantMessage creates an assistant role message with given content.
func NewAssistantMessage(content string) *Message {
	return &Message{
		Role:    MessageRoleAssistant,
		Content: content,
	}
}

// NewToolMessage creates a tool role message, including tool name and call ID.
func NewToolMessage(toolName, toolCallID, content string) *Message {
	return &Message{
		Role:       MessageRoleTool,
		Name:       toolName,
		ToolCallID: toolCallID,
		Content:    content,
	}
}

// NewAssistantToolCallMessage records a tool call emitted by the assistant.
func NewAssistantToolCallMessage(toolCall ToolCall) *Message {
	tc := toolCall
	return &Message{
		Role:       MessageRoleAssistant,
		Name:       tc.Name,
		ToolCallID: tc.CallID,
		ToolCall:   &tc,
	}
}
