package workflow

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/core/resources/settings"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

type ComposeRequest struct {
	// WorkflowSlug is empty when the request should create a workflow.
	WorkflowSlug string `json:"workflow_slug"`
	Request      string `json:"request"`
	// Harness and Model override the settings choice for this call
	// only — empty means "use the configured default".
	Harness string `json:"harness"`
	Model   string `json:"model"`
}

func (r *ComposeRequest) Validate() error {
	if strings.TrimSpace(r.Request) == "" {
		return ez.New(ez.EINVALID, "request is required", nil)
	}

	harness := strings.TrimSpace(r.Harness)
	if harness != "" {
		err := agent.Harness(harness).Validate()
		if err != nil {
			return ez.New(ez.EINVALID, "Unknown harness "+harness, err)
		}
	}

	if harness == "" && strings.TrimSpace(r.Model) != "" {
		return ez.New(ez.EINVALID, "harness is required when a model is set", nil)
	}

	return nil
}

type ComposeResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Action       string `json:"action"`
	Summary      string `json:"summary"`
	Harness      string `json:"harness"`
	Model        string `json:"model"`
	// Draft is the proposed spec now waiting for Save — empty
	// when the composer proposed nothing.
	Draft string `json:"draft,omitempty"`
}

// workflowIDFromSpec reads workflow.id (the permanent uuid) out of
// spec YAML — "" when absent or unparsable.
func workflowIDFromSpec(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return ""
	}
	var doc struct {
		Workflow struct {
			ID string `yaml:"id"`
		} `yaml:"workflow"`
	}
	if err := yaml.Unmarshal([]byte(spec), &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Workflow.ID)
}

// harnessCatalogText renders the installed harnesses and their model
// ids for the composer agent — its only source of truth for
// config.harness values. Pi's huge catalog is truncated; its ids
// follow provider/model and the agent can list more via the CLI.
func harnessCatalogText(ctx context.Context) string {
	lines := []string{}
	for _, info := range harnesses.ListHarnessInfo(ctx) {
		if !info.Available {
			continue
		}
		models := info.Models
		suffix := ""
		if len(models) > 24 {
			models = models[:24]
			suffix = ", …"
		}
		lines = append(
			lines,
			string(info.ID)+": "+strings.Join(models, ", ")+suffix,
		)
	}
	return strings.Join(lines, "\n")
}

// composerAgent resolves which harness/model the composer runs on:
// the request's override, the settings choice, or the first installed
// harness in the catalog. An override harness without a model gets
// that harness's first catalog model.
func composerAgent(ctx context.Context, overrideHarness, overrideModel string) (agent.Harness, string, error) {
	harness := strings.TrimSpace(overrideHarness)
	model := strings.TrimSpace(overrideModel)

	if harness == "" {
		data, err := settings.Load()
		if err != nil {
			return "", "", ez.Wrap(err)
		}

		harness = strings.TrimSpace(data.Composer.Harness)
		model = strings.TrimSpace(data.Composer.Model)
	}

	if harness != "" && model != "" {
		return agent.Harness(harness), model, nil
	}

	for _, info := range harnesses.ListHarnessInfo(ctx) {
		if !info.Available || len(info.Models) == 0 {
			continue
		}
		if harness != "" && string(info.ID) != harness {
			continue
		}

		return info.ID, info.Models[0], nil
	}

	return "", "", ez.New(ez.EINVALID, "No harness is available for the composer — pick one in Settings", nil)
}

func (api *API) Compose(ctx context.Context, requester interface{}, request *ComposeRequest) (*ComposeResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	harness, model, err := composerAgent(ctx, request.Harness, request.Model)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	workflowID := strings.TrimSpace(request.WorkflowSlug)

	// The edit base: an unsaved draft when one exists, else the saved
	// spec. Empty for a create.
	baseSpec := ""
	if workflowID != "" {
		baseSpec, err = api.Registry.ReadDraft(ctx, workflowID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		if baseSpec == "" {
			raw, err := api.Registry.SpecBytes(ctx, workflowID)
			if err != nil {
				return nil, ez.Wrap(err)
			}
			baseSpec = string(raw)
		}
	}

	result, err := workflowruntime.Compose(ctx, workflowruntime.ComposeOptions{
		WorkflowSlug: workflowID,
		BaseSpec:     baseSpec,
		Request:      request.Request,
		Harness:      harness,
		Model:        model,
		Catalog:      harnessCatalogText(ctx),
	})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	response := &ComposeResponse{
		WorkflowSlug: result.WorkflowSlug,
		Action:       result.Action,
		Summary:      result.Summary,
		Harness:      string(harness),
		Model:        model,
	}

	if result.Action == "unchanged" || result.YAML == "" {
		return response, nil
	}

	// Trust but verify: the proposal must compile on the server's own
	// compiler and keep its id before it becomes the draft.
	proposedID, err := api.Registry.VerifyProposedSpec(
		ctx,
		[]byte(result.YAML),
		workflowID,
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// The permanent id is not the agent's to manage — carry the
	// base's identity into the proposal.
	draftBytes := []byte(result.YAML)
	baseID := workflowIDFromSpec(baseSpec)
	if baseID != "" {
		draftBytes, err = workflowruntime.StampWorkflowID(
			draftBytes,
			baseID,
		)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	err = api.Registry.WriteDraft(ctx, proposedID, draftBytes)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	response.WorkflowSlug = proposedID
	response.Draft = string(draftBytes)

	return response, nil
}

type SaveDraftRequest struct {
	WorkflowSlug string `json:"workflow_slug"`
}

func (r *SaveDraftRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowSlug) == "" {
		return ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}

	return nil
}

type SaveDraftResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Version      string `json:"version"`
	Spec         string `json:"spec"`
}

func (api *API) SaveDraft(ctx context.Context, requester interface{}, request *SaveDraftRequest) (*SaveDraftResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	saved, err := api.Registry.SaveDraft(ctx, request.WorkflowSlug)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &SaveDraftResponse{
		WorkflowSlug: saved.WorkflowSlug,
		Version:      saved.Version,
		Spec:         saved.Spec,
	}, nil
}

type DeleteDraftRequest struct {
	WorkflowSlug string `json:"workflow_slug"`
}

func (r *DeleteDraftRequest) Validate() error {
	if strings.TrimSpace(r.WorkflowSlug) == "" {
		return ez.New(ez.EINVALID, "workflow_slug is required", nil)
	}

	return nil
}

type DeleteDraftResponse struct {
	WorkflowSlug string `json:"workflow_slug"`
	Deleted      bool   `json:"deleted"`
}

func (api *API) DeleteDraft(ctx context.Context, requester interface{}, request *DeleteDraftRequest) (*DeleteDraftResponse, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.Registry.DeleteDraft(ctx, request.WorkflowSlug)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &DeleteDraftResponse{
		WorkflowSlug: strings.TrimSpace(request.WorkflowSlug),
		Deleted:      true,
	}, nil
}
