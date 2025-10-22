package specs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/models/provider"
	"github.com/vanclief/agent-composer/runtime/llm"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Name            string               `json:"name"`
	Provider        provider.LLMProvider `json:"provider"`
	Model           string               `json:"model"`
	Instructions    string               `json:"instructions"`
	ReasoningEffort llm.ReasoningEffort  `json:"reasoning_effort"`
	AllowedTools    []string             `json:"allowed_tools"`
}

func (r CreateRequest) Validate() error {
	const op = "CreateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.Name, validation.Required),
		validation.Field(&r.Provider, validation.Required),
		validation.Field(&r.Model, validation.Required),
		validation.Field(&r.Instructions, validation.Required),
		validation.Field(&r.ReasoningEffort, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*agent.Spec, error) {
	const op = "specs.API.Create"

	// TODO: Permissions check

	pt, err := agent.NewAgentSpec(request.Name, request.Provider, request.Model, request.Instructions, request.ReasoningEffort, 1, request.AllowedTools)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	llmProvider, err := llm.NewOpenAI()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = llmProvider.ValidateModel(ctx, pt.Model)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = pt.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}
