package specs

import (
	"context"
	"encoding/json"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type UpdateRequest struct {
	AgentSpecID   uuid.UUID        `json:"agent_spec_id"`
	Harness       *agent.Harness   `json:"harness"`
	Name          *string          `json:"name"`
	Model         *string          `json:"model"`
	HarnessConfig *json.RawMessage `json:"harness_config"`
	Instructions  *string          `json:"instructions"`
}

func (r UpdateRequest) Validate() error {
	const op = "UpdateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	if r.HarnessConfig != nil {
		err := validateHarnessConfig(*r.HarnessConfig)
		if err != nil {
			return ez.Wrap(op, err)
		}
	}

	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*agent.Spec, error) {
	const op = "specs.API.Update"

	// Step 1: Get the agent spec
	spec, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2: Update the agent spec
	// TODO: Permissions check

	shouldInsert := false

	// Step 3: Update the agent spec
	if request.Name != nil {
		spec.Name = *request.Name
		shouldInsert = true
	}

	if request.Harness != nil {
		spec.Harness = *request.Harness
		shouldInsert = true
	}

	if request.Model != nil {
		spec.Model = *request.Model
		shouldInsert = true
	}

	if request.HarnessConfig != nil {
		spec.HarnessConfig = agent.CopyRawJSON(*request.HarnessConfig)
		shouldInsert = true
	}

	if request.Instructions != nil {
		spec.Instructions = *request.Instructions
		shouldInsert = true
	}

	if !shouldInsert {
		return nil, ez.New(op, ez.EINVALID, "No fields to update", nil)
	}

	err = api.rt.ValidateHarness(ctx, spec.Harness, spec.Model, spec.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	spec.Version += 1

	err = spec.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return spec, nil
}
