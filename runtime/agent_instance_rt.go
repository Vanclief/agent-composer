package runtime

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	types "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

// TODO: Try to take out the Runtime

func (rt *Runtime) NewConversationInstanceFromSpec(ctx context.Context, agentSpecID uuid.UUID, sessionID string, metadata map[string]any, shellRoot string) (*ConversationInstance, error) {
	const op = "runtime.NewConversationInstanceFromSpec"

	// Step 1) Fetch the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, rt.db, agentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	msgs := []types.Message{*types.NewSystemMessage(spec.Instructions)}

	// Step 2) Create the a new conversation
	conversation, err := agent.NewConversation(spec, msgs, metadata)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	conversation.ShellRoot = strings.TrimSpace(shellRoot)
	if conversation.ShellRoot == "" {
		conversation.ShellRoot = rt.shellRoot
	}
	conversation.SessionID = sessionID

	return rt.newAgentInstance(ctx, conversation, true)
}

func (rt *Runtime) NewConversationInstance(ctx context.Context, conversationID uuid.UUID) (*ConversationInstance, error) {
	const op = "runtime.NewConversationInstance"

	// Step 1) Load the existing conversation
	conversation, err := agent.GetConversationByID(ctx, rt.db, conversationID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt.newAgentInstance(ctx, conversation, false)
}

func (rt *Runtime) newAgentInstance(ctx context.Context, conversation *agent.Conversation, new bool) (*ConversationInstance, error) {
	const op = "runtime.NewAgentInstance"

	err := validateLegacyConversationOptions(conversation)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	harness, err := harnesses.New(conversation.Harness)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = harness.Validate(ctx, conversation.Model, conversation.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if new {
		err = conversation.Insert(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	} else {
		err = conversation.Update(ctx, rt.db)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}
	}

	hooks, err := loadInstanceHooks(ctx, rt.db, conversation.AgentName)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if hasEnabledHooks(hooks) {
		return nil, ez.New(op, ez.EINVALID, "hooks are not supported for harness-based conversations", nil)
	}

	ci := &ConversationInstance{
		Conversation: conversation,
		harness:      harness,
		hooks:        hooks,
	}

	return ci, nil
}

func validateLegacyConversationOptions(conversation *agent.Conversation) error {
	const op = "runtime.validateLegacyConversationOptions"

	if conversation == nil {
		return ez.New(op, ez.EINVALID, "conversation is nil", nil)
	}

	if conversation.AutoCompact {
		return ez.New(op, ez.EINVALID, "auto_compact is not supported for harness-based conversations", nil)
	}

	if conversation.WebSearch {
		return ez.New(op, ez.EINVALID, "web_search must be configured through harness_config", nil)
	}

	if conversation.StructuredOutput {
		return ez.New(op, ez.EINVALID, "structured_output must be configured through harness_config", nil)
	}

	return nil
}

func hasEnabledHooks(items map[hook.EventType][]hook.Hook) bool {
	for _, hooks := range items {
		if len(hooks) > 0 {
			return true
		}
	}

	return false
}
