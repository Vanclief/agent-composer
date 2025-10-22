package runs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/parrot"
)

type GetRequest struct {
	ParrotRunID uuid.UUID `json:"parrot_run_id"`
}

func (r GetRequest) Validate() error {
	const op = "GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ParrotRunID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*parrot.Run, error) {
	const op = "parrots.API.Get"

	// Step 1: Get the parrot run
	pr, err := parrot.GetParrotRunByID(ctx, api.db, request.ParrotRunID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	return pr, nil
}
