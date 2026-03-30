package specs

import (
	"context"
	"encoding/json"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Name          string          `json:"name"`
	Harness       agent.Harness   `json:"harness"`
	Model         string          `json:"model"`
	HarnessConfig json.RawMessage `json:"harness_config"`
	Instructions  string          `json:"instructions"`
}

func (r CreateRequest) Validate() error {
	const op = "CreateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Harness, validation.Required),
		validation.Field(&r.Model, validation.Required),
		validation.Field(&r.Instructions, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	err = validateHarnessConfig(r.HarnessConfig)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*agent.Spec, error) {
	const op = "specs.API.Create"

	// TODO: Permissions check

	spec, err := agent.NewAgentSpec(request.Name, request.Harness, request.Model, request.HarnessConfig, request.Instructions, "", 1)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = api.rt.ValidateHarness(ctx, spec.Harness, spec.Model, spec.HarnessConfig)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = spec.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return spec, nil
}

func validateHarnessConfig(raw json.RawMessage) error {
	const op = "validateHarnessConfig"

	if len(raw) == 0 {
		return nil
	}

	var payload map[string]any
	err := json.Unmarshal(raw, &payload)
	if err != nil {
		return ez.New(op, ez.EINVALID, "harness_config must be valid JSON object", err)
	}

	return nil
}
