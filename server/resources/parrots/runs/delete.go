package runs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/parrot"
)

type DeleteRequest struct {
	ParrotRunID uuid.UUID `json:"parrot_template_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ParrotRunID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*parrot.Run, error) {
	const op = "runs.API.Delete"

	// Step 1: Get the template
	pr, err := parrot.GetParrotRunByID(ctx, api.db, request.ParrotRunID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	// Step 3: Delete the parrot run
	err = pr.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pr, nil
}
