package templates

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/parrot"
)

type RunRequest struct {
	ParrotTemplateID uuid.UUID `json:"parrot_template_id"`
	Prompt           string    `json:"prompt"`
	Model            string    `json:"model"`
	ParallelRuns     int       `json:"parallel_runs"`
}

func (r RunRequest) Validate() error {
	const op = "RunRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.ParrotTemplateID, validation.Required),
		validation.Field(&r.Prompt, validation.Required),
		validation.Field(&r.Model, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

type RunResponse struct {
	IDs     []uuid.UUID `json:"ids,omitempty"`
	Success bool        `json:"success"`
}

func (api *API) Run(ctx context.Context, requester interface{}, request *RunRequest) (*RunResponse, error) {
	const op = "templates.API.Run"

	// Step 1: Get the template
	pt, err := parrot.GetParrotTemplateByID(ctx, api.db, request.ParrotTemplateID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// TODO: Permissions check

	if request.ParallelRuns == 0 {
		request.ParallelRuns = 1
	}

	runs, err := api.orchestrator.CreateRuns(ctx, pt.ID, request.ParallelRuns)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	api.orchestrator.RunInstances(runs, request.Prompt)

	ids := make([]uuid.UUID, len(runs))
	for i, run := range runs {
		ids[i] = run.ID
	}

	return &RunResponse{IDs: ids, Success: true}, nil
}
