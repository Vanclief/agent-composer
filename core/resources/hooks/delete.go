package hooks

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/hook"
	"github.com/vanclief/ez"
)

type DeleteRequest struct {
	HookID uuid.UUID `json:"hook_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "hooks.DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.HookID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}
	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (uuid.UUID, error) {
	const op = "hooks.API.Delete"

	err := request.Validate()
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// Get
	h, err := hook.GetHookByID(ctx, api.db, request.HookID)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	// TODO: permissions

	// Delete
	err = h.Delete(ctx, api.db)
	if err != nil {
		return uuid.Nil, ez.Wrap(op, err)
	}

	return h.ID, nil
}
