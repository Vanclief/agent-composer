package hooks

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	EventType    hook.EventType `json:"event_type"`
	TemplateName string         `json:"template_name"` // empty = wildcard
	Command      string         `json:"command"`
	Args         []string       `json:"args"`
	Enabled      bool           `json:"enabled"`
}

func (r CreateRequest) Validate() error {
	const op = "hooks.CreateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.EventType, validation.Required),
		validation.Field(&r.Command, validation.Required),
		validation.Field(&r.Enabled, validation.Required),
		// TemplateName optional, Args optional
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}
	return nil
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*hook.Hook, error) {
	const op = "hooks.API.Create"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	h, err := hook.NewHook(request.EventType, strings.TrimSpace(request.TemplateName), strings.TrimSpace(request.Command), request.Args, request.Enabled)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = h.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return h, nil
}
