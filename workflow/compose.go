package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

// ResolveHomeDir returns the agc home directory (AGENT_COMPOSER_HOME,
// or ~/.agent_composer) — where the database, config, and settings
// live.
func ResolveHomeDir() (string, error) {
	configRoot := strings.TrimSpace(os.Getenv(workflowHomeEnvVar))
	if configRoot != "" {
		return configRoot, nil
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", ez.Wrap(err)
	}

	return filepath.Join(userHome, defaultWorkflowHome), nil
}

// composerInstruction is the built-in system prompt of the workflow
// composer — the agent behind "Describe a change…". It ships with the
// binary so it cannot drift or be edited away. The agent only ever
// PROPOSES a spec; persistence is the server's job and nothing
// lands without the user pressing Save.
const composerInstruction = `You design AGC workflow specs. You propose a spec; you never install one — the user reviews your proposal as a draft and decides whether to save it.

A spec is one YAML file with four top-level sections.
workflow: slug, name, version, description, inputs (name: type), outputs (name: {schema, from: instance.<id>.<output>}).
schemas: named types (type object/array/string/integer/number/boolean, properties, items, schema_ref, enum).
nodes: reusable definitions — kinds are inference (typed inputs/outputs plus config.harness {id, model, reasoning_effort, permissions} and config.instruction), connector (operation: collect | concat | pack | unpack), loop (operation: foreach | while, executes: <node>, over/updates/breaks_on/max_iterations), conditional (operation: if, routes_on, when_true/when_false: <node>), and workflow (workflow_slug: <slug>) for composition.
flow.instances: instance_id: {node: <node>, inputs: {port: workflow_input.<name> | instance.<id>.<output>}}.

The current_spec input holds the spec you are editing (it may already contain unsaved draft changes); it is empty when the request is to create a new workflow. The available_harnesses input lists the installed harnesses and their real model ids — config.harness.id and config.harness.model must come from that list, exactly as written; never invent or abbreviate a model name.

Do this:
1. If you need DSL patterns beyond the reference above, run "agc workflow list" and "agc workflow show --slug <slug>" to study installed specs. Never use "agc workflow import" or "agc workflow delete".
2. Write the complete new or updated spec to a scratch file in your working directory. The registry lives in a database — a YAML file only becomes installed through an explicit import or save.
3. Validate it with "agc workflow compile --file <scratch>". Fix and re-compile until it passes, then delete the scratch file.
4. Return the final spec as the yaml field of your result.

Rules: when editing, keep workflow.slug and workflow.id unchanged and change only what the request asks for — preserve every unrelated node, prompt, and schema. For a new workflow pick a short kebab-case slug that is not already installed. Prefer the fewest nodes that satisfy the request, and reuse one node definition for parallel instances. If the request is impossible or too ambiguous to act on safely, return action unchanged with an empty yaml and explain why in the summary.

Return workflow_slug (the final slug), action (created for a new slug, updated when you changed an existing workflow, unchanged when you propose nothing), yaml (the complete spec, or "" when unchanged), and a 1-3 sentence summary.`

// composeResultSchema must stay strict (every property required,
// additionalProperties false) — OpenAI structured outputs rejects
// anything looser.
var composeResultSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"workflow_slug": map[string]any{"type": "string"},
		"action": map[string]any{
			"type": "string",
			"enum": []any{"created", "updated", "unchanged"},
		},
		"yaml":    map[string]any{"type": "string"},
		"summary": map[string]any{"type": "string"},
	},
	"required":             []any{"workflow_slug", "action", "yaml", "summary"},
	"additionalProperties": false,
}

type ComposeOptions struct {
	// WorkflowSlug is empty when the request should create a workflow.
	WorkflowSlug string
	// BaseSpec is the spec being edited — the draft when one
	// exists, else the saved file. Empty for a create.
	BaseSpec string
	Request  string
	Harness  agent.Harness
	Model    string
	// ReasoningEffort is medium when empty.
	ReasoningEffort runtimetypes.ReasoningEffort
	// Catalog lists installed harnesses and their real model ids, so
	// the agent never invents a model name.
	Catalog string
}

type ComposeResult struct {
	WorkflowSlug string
	Action       string
	YAML         string
	Summary      string
}

// Compose runs one composer conversation: the agent edits the registry
// through the agc CLI and reports what it did. The working directory
// is the agc home, so a workspace-scoped harness sandbox still reaches
// the registry.
func Compose(ctx context.Context, opts ComposeOptions) (*ComposeResult, error) {
	if strings.TrimSpace(opts.Request) == "" {
		return nil, ez.New(ez.EINVALID, "request is required", nil)
	}

	err := opts.Harness.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if strings.TrimSpace(opts.Model) == "" {
		return nil, ez.New(ez.EINVALID, "model is required", nil)
	}

	effort := opts.ReasoningEffort
	if strings.TrimSpace(string(effort)) == "" {
		effort = runtimetypes.ReasoningEffortMedium
	}
	err = effort.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	home, err := ResolveHomeDir()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	schemaRaw, err := json.Marshal(composeResultSchema)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	executor := NewExecutor(home)
	value, err := executor.runStructuredNode(
		ctx,
		"workflow-composer",
		composerInstruction,
		opts.Harness,
		strings.TrimSpace(opts.Model),
		effort,
		// The agc home is not a git repository — codex refuses to run
		// in untrusted directories unless the check is skipped.
		json.RawMessage(`{"permissions":"exec","skip_git_repo_check":true}`),
		composeResultSchema,
		schemaRaw,
		map[string]any{
			"workflow_slug":       strings.TrimSpace(opts.WorkflowSlug),
			"current_spec":        opts.BaseSpec,
			"request":             strings.TrimSpace(opts.Request),
			"available_harnesses": opts.Catalog,
		},
		nil,
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	fields, ok := value.(map[string]any)
	if !ok {
		return nil, ez.New(ez.EINTERNAL, "composer returned an unexpected result shape", nil)
	}

	text := func(key string) string {
		raw, _ := fields[key].(string)
		return strings.TrimSpace(raw)
	}

	rawYAML, _ := fields["yaml"].(string)

	return &ComposeResult{
		WorkflowSlug: text("workflow_slug"),
		Action:       text("action"),
		YAML:         strings.TrimSpace(rawYAML),
		Summary:      text("summary"),
	}, nil
}
