package templates

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/parrot"
)

type GetRequest struct {
	ParrotTemplateID uuid.UUID `json:"parrot_template_id"`
}

func (r GetRequest) Validate() error {
	const op = "GetRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ParrotTemplateID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Get(ctx context.Context, requester interface{}, request *GetRequest) (*parrot.Template, error) {
	const op = "templates.API.Get"

	// Step 1: Get the template
	pt, err := parrot.GetParrotTemplateByID(ctx, api.db, request.ParrotTemplateID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	return pt, nil
}
