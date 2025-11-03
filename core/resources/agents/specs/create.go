package specs

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type CreateRequest struct {
	Name                   string                       `json:"name"`
	Provider               agent.LLMProvider            `json:"provider"`
	Model                  string                       `json:"model"`
	Instructions           string                       `json:"instructions"`
	ReasoningEffort        runtimetypes.ReasoningEffort `json:"reasoning_effort"`
	AutoCompact            bool                         `json:"auto_compact"`
	CompactAtPercent       *int                         `json:"compact_at_percent"`
	CompactionPrompt       string                       `json:"compaction_prompt"`
	AllowedTools           []string                     `json:"allowed_tools"`
	ShellAccess            *bool                        `json:"shell_access"`
	WebSearch              *bool                        `json:"web_search"`
	StructuredOutput       *bool                        `json:"structured_output"`
	StructuredOutputSchema map[string]any               `json:"structured_output_schema"`
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

	if r.CompactAtPercent != nil {
		if *r.CompactAtPercent <= 0 || *r.CompactAtPercent > 100 {
			return ez.New(op, ez.EINVALID, "compact_at_percent must be between 1 and 100", nil)
		}
	}

	if r.StructuredOutput != nil && *r.StructuredOutput {
		if len(r.StructuredOutputSchema) == 0 {
			return ez.New(op, ez.EINVALID, "structured_output_schema is required when structured_output is true", nil)
		}
	}

	return nil
}

func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*agent.Spec, error) {
	const op = "specs.API.Create"

	// TODO: Permissions check

	spec, err := agent.NewAgentSpec(request.Name, request.Provider, request.Model, request.Instructions, request.ReasoningEffort, 1)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = api.rt.ValidateModel(ctx, spec.Provider, spec.Model)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	spec.AutoCompact = request.AutoCompact

	if request.CompactAtPercent != nil {
		spec.CompactAtPercent = *request.CompactAtPercent
	}

	spec.CompactionPrompt = strings.TrimSpace(request.CompactionPrompt)

	if request.ShellAccess != nil {
		spec.ShellAccess = *request.ShellAccess
	}

	if request.WebSearch != nil {
		spec.WebSearch = *request.WebSearch
	}

	if request.StructuredOutput != nil {
		spec.StructuredOutput = *request.StructuredOutput
		if spec.StructuredOutput {
			spec.StructuredOutputSchema = request.StructuredOutputSchema
		} else {
			spec.StructuredOutputSchema = nil
		}
	} else if len(request.StructuredOutputSchema) > 0 {
		// If schema was provided without toggling the flag, assume structured outputs should be enabled.
		spec.StructuredOutput = true
		spec.StructuredOutputSchema = request.StructuredOutputSchema
	}

	err = spec.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return spec, nil
}
