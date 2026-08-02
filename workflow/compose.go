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
// or ~/.agent_composer) — the parent of the workflow registry.
func ResolveHomeDir() (string, error) {
	workflowDir, err := ResolveWorkflowDir()
	if err != nil {
		return "", ez.Wrap("workflow.ResolveHomeDir", err)
	}

	return filepath.Dir(workflowDir), nil
}

// composerInstruction is the built-in system prompt of the workflow
// composer — the agent behind "Describe a change…". It ships with the
// binary so it cannot drift or be edited away. The agent only ever
// PROPOSES a blueprint; persistence is the server's job and nothing
// lands without the user pressing Save.
const composerInstruction = `You design AGC workflow blueprints. You propose a blueprint; you never install one — the user reviews your proposal as a draft and decides whether to save it.

A blueprint is one YAML file with four top-level sections.
workflow: id, name, version, description, inputs (name: type), outputs (name: {schema, from: instance.<id>.<output>}).
schemas: named types (type object/array/string/integer/number/boolean, properties, items, schema_ref, enum).
nodes: reusable definitions — kinds are inference (typed inputs/outputs plus config.harness {id, model, reasoning_effort, permissions} and config.instruction), connector (operation: collect | concat | pack | unpack), loop (operation: foreach | while, executes: <node>, over/updates/breaks_on/max_iterations), conditional (operation: if, routes_on, when_true/when_false: <node>), and workflow (workflow_id: <id>) for composition.
flow.instances: instance_id: {node: <node>, inputs: {port: workflow_input.<name> | instance.<id>.<output>}}.

The current_blueprint input holds the blueprint you are editing (it may already contain unsaved draft changes); it is empty when the request is to create a new workflow. The available_harnesses input lists the installed harnesses and their real model ids — config.harness.id and config.harness.model must come from that list, exactly as written; never invent or abbreviate a model name.

Do this:
1. If you need DSL patterns beyond the reference above, run "agc workflow list" and "agc workflow show --id <id>" to study installed blueprints. Never use "agc workflow import" or "agc workflow delete".
2. Write the complete new or updated blueprint to a scratch file in your working directory. Never write inside the workflows directory — the registry treats every YAML file there as installed.
3. Validate it with "agc workflow compile --file <scratch>". Fix and re-compile until it passes, then delete the scratch file.
4. Return the final blueprint as the yaml field of your result.

Rules: when editing, keep workflow.id and workflow.uuid unchanged and change only what the request asks for — preserve every unrelated node, prompt, and schema. For a new workflow pick a short kebab-case id that is not already installed. Prefer the fewest nodes that satisfy the request, and reuse one node definition for parallel instances. If the request is impossible or too ambiguous to act on safely, return action unchanged with an empty yaml and explain why in the summary.

Return workflow_id (the final id), action (created for a new id, updated when you changed an existing workflow, unchanged when you propose nothing), yaml (the complete blueprint, or "" when unchanged), and a 1-3 sentence summary.`

var composeResultSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"workflow_id": map[string]any{"type": "string"},
		"action": map[string]any{
			"type": "string",
			"enum": []any{"created", "updated", "unchanged"},
		},
		"yaml":    map[string]any{"type": "string"},
		"summary": map[string]any{"type": "string"},
	},
}

type ComposeOptions struct {
	// WorkflowID is empty when the request should create a workflow.
	WorkflowID string
	// BaseSpec is the blueprint being edited — the draft when one
	// exists, else the saved file. Empty for a create.
	BaseSpec string
	Request  string
	Harness  agent.Harness
	Model    string
	// Catalog lists installed harnesses and their real model ids, so
	// the agent never invents a model name.
	Catalog string
}

type ComposeResult struct {
	WorkflowID string
	Action     string
	YAML       string
	Summary    string
}

// Compose runs one composer conversation: the agent edits the registry
// through the agc CLI and reports what it did. The working directory
// is the agc home, so a workspace-scoped harness sandbox still reaches
// the registry.
func Compose(ctx context.Context, opts ComposeOptions) (*ComposeResult, error) {
	const op = "workflow.Compose"

	if strings.TrimSpace(opts.Request) == "" {
		return nil, ez.New(op, ez.EINVALID, "request is required", nil)
	}

	err := opts.Harness.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	if strings.TrimSpace(opts.Model) == "" {
		return nil, ez.New(op, ez.EINVALID, "model is required", nil)
	}

	home, err := ResolveHomeDir()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	schemaRaw, err := json.Marshal(composeResultSchema)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	executor := NewExecutor(home)
	value, err := executor.runStructuredNode(
		ctx,
		"workflow-composer",
		composerInstruction,
		opts.Harness,
		strings.TrimSpace(opts.Model),
		runtimetypes.ReasoningEffortMedium,
		json.RawMessage(`{"permissions":"exec"}`),
		composeResultSchema,
		schemaRaw,
		map[string]any{
			"workflow_id":         strings.TrimSpace(opts.WorkflowID),
			"current_blueprint":   opts.BaseSpec,
			"request":             strings.TrimSpace(opts.Request),
			"available_harnesses": opts.Catalog,
		},
		nil,
	)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	fields, ok := value.(map[string]any)
	if !ok {
		return nil, ez.New(op, ez.EINTERNAL, "composer returned an unexpected result shape", nil)
	}

	text := func(key string) string {
		raw, _ := fields[key].(string)
		return strings.TrimSpace(raw)
	}

	rawYAML, _ := fields["yaml"].(string)

	return &ComposeResult{
		WorkflowID: text("workflow_id"),
		Action:     text("action"),
		YAML:       strings.TrimSpace(rawYAML),
		Summary:    text("summary"),
	}, nil
}

// VerifyProposedBlueprint compiles a composer proposal and confirms
// its workflow.id, before the proposal may become a draft.
func VerifyProposedBlueprint(raw []byte, expectedID string) (string, error) {
	const op = "workflow.VerifyProposedBlueprint"

	scratch, err := os.CreateTemp("", "agc-proposal-*.yaml")
	if err != nil {
		return "", ez.Wrap(op, err)
	}
	scratchPath := scratch.Name()
	defer os.Remove(scratchPath)

	_, err = scratch.Write(raw)
	if closeErr := scratch.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	blueprint, err := LoadBlueprintFile(scratchPath)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	_, err = Compile(blueprint)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	proposedID := strings.TrimSpace(blueprint.Workflow.ID)
	if expectedID != "" && proposedID != expectedID {
		return "", ez.New(op, ez.EINVALID, "The proposal changed workflow.id from "+expectedID+" to "+proposedID, nil)
	}

	return proposedID, nil
}
