package templates

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/parrot"
)

type DeleteRequest struct {
	ParrotTemplateID uuid.UUID `json:"parrot_template_id"`
}

func (r DeleteRequest) Validate() error {
	const op = "DeleteRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ParrotTemplateID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Delete(ctx context.Context, requester interface{}, request *DeleteRequest) (*parrot.Template, error) {
	const op = "templates.API.Delete"

	// Step 1: Get the template
	pt, err := parrot.GetParrotTemplateByID(ctx, api.db, request.ParrotTemplateID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	// Step 2: Make sure there are no runs for this template
	prs, err := parrot.GetParrotRunsByTemplateID(ctx, api.db, pt.ID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if len(prs) > 0 {
		return nil, ez.New(op, ez.EINVALID, "Cannot delete parrot template with existing runs", nil)
	}

	// Step 3: Delete the parrot
	err = pt.Delete(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}
