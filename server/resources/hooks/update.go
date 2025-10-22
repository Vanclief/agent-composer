package hooks

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/hook"
)

type UpdateRequest struct {
	HookID       uuid.UUID       `json:"hook_id"`
	EventType    *hook.EventType `json:"event_type,omitempty"`
	TemplateName *string         `json:"template_name,omitempty"`
	Command      *string         `json:"command,omitempty"`
	Args         *[]string       `json:"args,omitempty"`
	Enabled      *bool           `json:"enabled,omitempty"`
}

func (r UpdateRequest) Validate() error {
	const op = "hooks.UpdateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.HookID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}
	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*hook.Hook, error) {
	const op = "hooks.API.Update"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Get
	var h hook.Hook
	err = api.db.NewSelect().
		Model(&h).
		Where("id = ?", request.HookID).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: permissions

	// Mutate
	changed := false

	if request.EventType != nil {
		h.EventType = *request.EventType
		changed = true
	}

	if request.TemplateName != nil {
		h.TemplateName = strings.TrimSpace(*request.TemplateName)
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
		return nil, ez.New(op, ez.EINVALID, "No fields to update", nil)
	}

	err = h.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Persist
	err = h.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &h, nil
}
