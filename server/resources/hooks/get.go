package hooks

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/hook"
)

type GetRequest struct {
	HookID uuid.UUID `json:"hook_id"`
}

func (r GetRequest) Validate() error {
	const op = "hooks.GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.HookID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}
	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*hook.Hook, error) {
	const op = "hooks.API.Get"

	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: permissions

	var h hook.Hook
	err = api.db.NewSelect().
		Model(&h).
		Where("id = ?", request.HookID).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &h, nil
}
