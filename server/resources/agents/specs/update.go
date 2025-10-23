package specs

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/llm"
	"github.com/vanclief/ez"
)

type UpdateRequest struct {
	AgentSpecID  uuid.UUID          `json:"agent_spec_id"`
	Provider     *agent.LLMProvider `json:"provider"`
	Name         *string            `json:"name"`
	Model        *string            `json:"model"`
	Instructions *string            `json:"instructions"`
	AllowedTools *[]string          `json:"allowed_tools"`
}

func (r UpdateRequest) Validate() error {
	const op = "UpdateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	return nil
}

func (api *API) Update(ctx context.Context, requester interface{}, request *UpdateRequest) (*agent.Spec, error) {
	const op = "specs.API.Update"

	// Step 1: Get the agent spec
	pt, err := agent.GetAgentSpecByID(ctx, api.db, request.AgentSpecID)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 2: Update the agent spec
	// TODO: Permissions check

	shouldInsert := false

	// Step 3: Update the agent spec
	if request.Name != nil {
		pt.Name = *request.Name
		shouldInsert = true
	}

	if request.Provider != nil {
		pt.Provider = *request.Provider
		shouldInsert = true
	}

	if request.Model != nil {
		pt.Model = *request.Model

		llmProvider, err := llm.NewOpenAI()
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		err = llmProvider.ValidateModel(ctx, pt.Model)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		shouldInsert = true
	}

	if request.Instructions != nil {
		pt.Instructions = *request.Instructions
		shouldInsert = true
	}

	if request.AllowedTools != nil {
		pt.AllowedTools = *request.AllowedTools
		shouldInsert = true
	}

	if !shouldInsert {
		return nil, ez.New(op, ez.EINVALID, "No fields to update", nil)
	}

	pt.Version += 1

	err = pt.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}
