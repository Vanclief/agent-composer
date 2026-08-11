package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

const renameTargetSpec = `workflow:
  slug: rename_target
  name: "Rename Target"
  version: "1"
  inputs:
    text: string
  outputs:
    out:
      schema: string
      from: instance.step.out

nodes:
  echo:
    kind: inference
    inputs:
      text: string
    outputs:
      out: string
    config:
      harness:
        id: codex_cli
        model: gpt-5
      instruction: Echo the text.

flow:
  instances:
    step:
      node: echo
      inputs:
        text: workflow_input.text
`

const renameEmbedderSpec = `workflow:
  slug: embedder
  name: "Embedder"
  version: "1"
  inputs:
    text: string
  outputs:
    out:
      schema: string
      from: instance.child.out

nodes:
  run_child:
    kind: workflow
    workflow_slug: rename_target
    inputs:
      text: string
    outputs:
      out: string

flow:
  instances:
    child:
      node: run_child
      inputs:
        text: workflow_input.text
`

// importRenameFixture installs a target workflow plus one that embeds
// it. The target must land first so the embedder compiles.
func importRenameFixture(t *testing.T) (*Registry, context.Context) {
	t.Helper()

	registry, ctx := newTestRegistry(t)
	importYAML(t, ctx, registry, renameTargetSpec)
	importYAML(t, ctx, registry, renameEmbedderSpec)

	return registry, ctx
}

func TestRenameWorkflowIDCascades(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	// A pending draft should follow the rename.
	draft := strings.Replace(
		renameTargetSpec,
		"instruction: Echo the text.",
		"instruction: Echo the text twice.",
		1,
	)
	err := registry.WriteDraft(ctx, "rename_target", []byte(draft))
	if err != nil {
		t.Fatal(err)
	}

	result, err := registry.Rename(ctx, "rename_target", "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowSlug != "renamed_target" {
		t.Fatalf("expected renamed_target, got %s", result.WorkflowSlug)
	}
	if len(result.UpdatedRefs) != 1 || result.UpdatedRefs[0] != "embedder" {
		t.Fatalf("expected embedder in updated refs, got %v", result.UpdatedRefs)
	}

	// Registry: the old id is gone, the new one carries the new id.
	_, err = registry.Load(ctx, "rename_target")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("the old id should be gone, got: %v", err)
	}
	installed, err := registry.SpecBytes(ctx, "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "slug: renamed_target") {
		t.Fatal("the renamed spec should carry the new id")
	}

	// The embedder now references the new id.
	embedder, err := registry.SpecBytes(ctx, "embedder")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(embedder), "workflow_slug: renamed_target") {
		t.Fatal("embedder should reference the new id")
	}

	// Draft moved and rewritten.
	oldDraft, err := registry.ReadDraft(ctx, "rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if oldDraft != "" {
		t.Fatal("old draft should be gone")
	}
	newDraft, err := registry.ReadDraft(ctx, "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(newDraft, "slug: renamed_target") ||
		!strings.Contains(newDraft, "twice") {
		t.Fatal("draft should follow the rename with its content intact")
	}

	// Version history stays attached to the workflow across the
	// rename, and the rename itself is a recorded version.
	versions, err := registry.ListVersions(ctx, "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected the import and the rename in the history, got %d entries", len(versions))
	}
}

func TestRenameWorkflowIDRejectsCollision(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	_, err := registry.Rename(ctx, "rename_target", "embedder")
	if err == nil {
		t.Fatal("expected a collision error")
	}
}

func TestSetWorkflowDisplayName(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	description := "A fancier description."
	err := registry.SetHeader(ctx, "rename_target", "Fancy New Name", &description)
	if err != nil {
		t.Fatal(err)
	}

	installed, err := registry.SpecBytes(ctx, "rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "Fancy New Name") {
		t.Fatal("display name should be rewritten")
	}
	if !strings.Contains(string(installed), "slug: rename_target") {
		t.Fatal("the id must not change on a name edit")
	}
	if !strings.Contains(string(installed), "A fancier description.") {
		t.Fatal("the description should be rewritten")
	}
}

func TestUUIDSurvivesSaveAndRename(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	first, err := registry.Load(ctx, "rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if first.Workflow.ID == "" {
		t.Fatal("import should mint a uuid")
	}

	// A save keeps it, even if the draft carries a bogus one.
	bogus := strings.Replace(
		renameTargetSpec,
		"version: \"1\"",
		"version: \"2\"\n  id: 00000000-0000-0000-0000-000000000bad",
		1,
	)
	err = registry.WriteDraft(ctx, "rename_target", []byte(bogus))
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.SaveDraft(ctx, "rename_target")
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Load(ctx, "rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if second.Workflow.ID != first.Workflow.ID {
		t.Fatalf("uuid changed on save: %s -> %s", first.Workflow.ID, second.Workflow.ID)
	}

	// A slug rename keeps it too.
	_, err = registry.Rename(ctx, "rename_target", "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := registry.Load(ctx, "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Workflow.ID != first.Workflow.ID {
		t.Fatalf("uuid changed on rename: %s -> %s", first.Workflow.ID, renamed.Workflow.ID)
	}
}

func TestCompileRejectsReservedInstanceIDs(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	// "step" -> "workflow-inputs" collides with the canvas's
	// synthetic node id and must not compile.
	bad := strings.Replace(
		renameTargetSpec,
		"    step:",
		"    workflow-inputs:",
		1,
	)
	err := registry.WriteDraft(ctx, "rename_target", []byte(bad))
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.SaveDraft(ctx, "rename_target")
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected a reserved-id compile error, got %v", err)
	}
}
