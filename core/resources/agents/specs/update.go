package specs

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/google/uuid"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/ez"
)

type UpdateRequest struct {
	AgentSpecID      uuid.UUID          `json:"agent_spec_id"`
	Provider         *agent.LLMProvider `json:"provider"`
	Name             *string            `json:"name"`
	Model            *string            `json:"model"`
	Instructions     *string            `json:"instructions"`
	AutoCompact      *bool              `json:"auto_compact"`
	CompactAtPercent *int               `json:"compact_at_percent"`
	CompactionPrompt *string            `json:"compaction_prompt"`
	AllowedTools     *[]string          `json:"allowed_tools"`
}

func (r UpdateRequest) Validate() error {
	const op = "UpdateRequest.Validate"

	err := validation.ValidateStruct(&r,
		validation.Field(&r.AgentSpecID, validation.Required),
	)
	if err != nil {
		return ez.New(op, ez.EINVALID, err.Error(), nil)
	}

	if r.CompactAtPercent != nil {
		if *r.CompactAtPercent <= 0 || *r.CompactAtPercent > 100 {
			return ez.New(op, ez.EINVALID, "compact_at_percent must be between 1 and 100", nil)
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

	if request.Provider != nil {
		spec.Provider = *request.Provider
		shouldInsert = true
	}

	if request.Model != nil {
		spec.Model = *request.Model

		err = api.rt.ValidateModel(ctx, spec.Provider, spec.Model)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		shouldInsert = true
	}

	if request.Instructions != nil {
		spec.Instructions = *request.Instructions
		shouldInsert = true
	}

	if request.AutoCompact != nil {
		spec.AutoCompact = *request.AutoCompact
		shouldInsert = true
	}

	if request.CompactAtPercent != nil {
		spec.CompactAtPercent = *request.CompactAtPercent
		shouldInsert = true
	}

	if request.CompactionPrompt != nil {
		spec.CompactionPrompt = strings.TrimSpace(*request.CompactionPrompt)
		shouldInsert = true
	}

	if !shouldInsert {
		return nil, ez.New(op, ez.EINVALID, "No fields to update", nil)
	}

	spec.Version += 1

	err = spec.Update(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return spec, nil
}
