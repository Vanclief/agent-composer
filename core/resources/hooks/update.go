package hooks

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/ez"
)

type UpdateRequest struct {
	HookID       uuid.UUID       `json:"hook_id"`
	EventType    *hook.EventType `json:"event_type,omitempty"`
	AgentName *string         `json:"agent_name,omitempty"`
	Command      *string         `json:"command,omitempty"`
	Args         *[]string       `json:"args,omitempty"`
	Enabled      *bool           `json:"enabled,omitempty"`
}

func (r UpdateRequest) Validate() error {
	err := validation.ValidateStruct(&r,
		validation.Field(&r.HookID, validation.Required),
	)
	if err != nil {
		return ez.New(ez.EINVALID, err.Error(), nil)
	}
	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*hook.Hook, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// Get
	h, err := hook.GetHookByID(ctx, api.db, request.HookID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// TODO: permissions

	// Mutate
	changed := false

	if request.EventType != nil {
		h.EventType = *request.EventType
		changed = true
	}

	if request.AgentName != nil {
		h.AgentName = strings.TrimSpace(*request.AgentName)
		changed = true
	}

	if request.Command != nil {
		h.Command = strings.TrimSpace(*request.Command)
		changed = true
	}

	if request.Args != nil {
		h.Args = *request.Args
		changed = true
	}

	if request.Enabled != nil {
		h.Enabled = *request.Enabled
		changed = true
	}

	if !changed {
		return nil, ez.New(ez.EINVALID, "No fields to update", nil)
	}

	err = h.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// Persist
	err = h.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return h, nil
}
