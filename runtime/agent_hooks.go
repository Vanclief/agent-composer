package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	shellmcp "github.com/vanclief/agent-composer/mcp/shell"
	"github.com/vanclief/agent-composer/models/hook"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

// func (ai *AgentInstance) RunHooks(ctx context.Context, event hook.EventType, toolCall *types.ToolCall, toolCallResponse string) error {
// 	for _, h := range ai.hooks[event] {
// 		_, err := ai.runToolHooks(ctx, h, toolCall, toolCallResponse)
// 		if err != nil {
// 			return err
// 		}
// 	}
//
// 	return nil
// }

func (ai *AgentInstance) RunConversationStartedHook(ctx context.Context) error {
	for _, h := range ai.hooks[hook.EventTypeConversationStarted] {
		_, err := ai.runConversationHooks(ctx, h)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) RunConversationEndedHook(ctx context.Context) error {
	for _, h := range ai.hooks[hook.EventTypeConversationEnded] {
		_, err := ai.runConversationHooks(ctx, h)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) RunPreToolUseHook(ctx context.Context, toolCall *types.ToolCall, toolCallResponse string) error {
	for _, h := range ai.hooks[hook.EventTypePreToolUse] {
		_, err := ai.runToolHooks(ctx, h, toolCall, toolCallResponse)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) RunPostToolUseHook(ctx context.Context, toolCall *types.ToolCall, toolCallResponse string) error {
	for _, h := range ai.hooks[hook.EventTypePostToolUse] {
		_, err := ai.runToolHooks(ctx, h, toolCall, toolCallResponse)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) RunPreContextCompactionHook(ctx context.Context, compactedConversationID uuid.UUID) error {
	for _, h := range ai.hooks[hook.EventTypePreContextCompaction] {
		_, err := ai.runCompactionHooks(ctx, h, compactedConversationID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ai *AgentInstance) RunPostContextCompactionHook(ctx context.Context, compactedConversationID uuid.UUID) error {
	for _, h := range ai.hooks[hook.EventTypePostContextCompaction] {
		_, err := ai.runCompactionHooks(ctx, h, compactedConversationID)
		if err != nil {
			return err
		}
	}

	return nil
}

type ConversationStateHook struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	EventType      hook.EventType `json:"event_type"`
	AgentName      string         `json:"agent_name"`
	LastResponse   string         `json:"last_response,omitempty"`
}

func (ai *AgentInstance) runConversationHooks(ctx context.Context, h hook.Hook) (HookResult, error) {
	var lastResponse string
	lam, found := ai.LatestAssistantMessage()
	if found {
		lastResponse = lam.Content
	}

	e := ConversationStateHook{
		ID:             h.ID.String(),
		ConversationID: ai.ID.String(),
		AgentName:      ai.name,
		EventType:      h.EventType,
		LastResponse:   lastResponse,
	}

	payload, _ := json.Marshal(e)

	out, err := RunHook(ctx, h, payload)
	if out.ExitCode == 2 {
		stderrText := strings.TrimSpace(string(out.Stderr))
		if stderrText == "" {
			stderrText = "hook failed"
		}

		ai.AddMessage(types.MessageRoleUser, stderrText)
		return out, err // Return on first exit code 2
	}

	return out, nil
}

type ToolUseHook struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	EventType      hook.EventType `json:"event_type"`
	AgentName      string         `json:"agent_name"`
	LastResponse   string         `json:"last_response,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolArguments  string         `json:"tool_arguments,omitempty"`
	ToolResponse   string         `json:"tool_response,omitempty"`
}

func (ai *AgentInstance) runToolHooks(ctx context.Context, h hook.Hook, toolCall *types.ToolCall, toolCallResponse string) (HookResult, error) {
	var lastResponse string
	lam, found := ai.LatestAssistantMessage()
	if found {
		lastResponse = lam.Content
	}

	if toolCall == nil {
		return HookResult{}, ez.New("runToolHooks", ez.EINVALID, "toolCall cannot be nil", nil)
	}

	e := ToolUseHook{
		ID:             h.ID.String(),
		ConversationID: ai.ID.String(),
		AgentName:      ai.name,
		EventType:      h.EventType,
		LastResponse:   lastResponse,
		ToolName:       toolCall.Name,
		ToolArguments:  toolCall.Arguments,
		ToolResponse:   toolCallResponse,
	}

	payload, _ := json.Marshal(e)

	out, err := RunHook(ctx, h, payload)
	if out.ExitCode == 2 {
		stderrText := strings.TrimSpace(string(out.Stderr))
		if stderrText == "" {
			stderrText = "hook failed"
		}

		payload := shellmcp.ShellRunResult{
			ExitCode: 1,
			Stderr:   stderrText,
			Command:  toolCall.CommandString(),
		}

		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Error().Err(marshalErr).Msg("Failed to marshal hook error payload")
			ai.AddToolMessage(toolCall.Name, toolCall.CallID, stderrText)
		} else {
			ai.AddToolMessage(toolCall.Name, toolCall.CallID, string(encoded))
		}

		return out, err // Return on first exit code 2
	}

	return out, nil
}

type CompactionHook struct {
	ID                      string         `json:"id"`
	ConversationID          string         `json:"conversation_id"`
	CompactedConversationID string         `json:"compacted_conversation_id"`
	EventType               hook.EventType `json:"event_type"`
	AgentName               string         `json:"agent_name"`
	LastResponse            string         `json:"last_response,omitempty"`
}

func (ai *AgentInstance) runCompactionHooks(ctx context.Context, h hook.Hook, compactedConversationID uuid.UUID) (HookResult, error) {
	var lastResponse string
	lam, found := ai.LatestAssistantMessage()
	if found {
		lastResponse = lam.Content
	}

	e := CompactionHook{
		ID:                      h.ID.String(),
		ConversationID:          ai.ID.String(),
		CompactedConversationID: compactedConversationID.String(),
		AgentName:               ai.name,
		EventType:               h.EventType,
		LastResponse:            lastResponse,
	}

	payload, _ := json.Marshal(e)

	out, err := RunHook(ctx, h, payload)
	if out.ExitCode == 2 {
		stderrText := strings.TrimSpace(string(out.Stderr))
		if stderrText == "" {
			stderrText = "hook failed"
		}

		ai.AddMessage(types.MessageRoleUser, stderrText)
		return out, err // Return on first exit code 2
	}

	return out, nil
}
