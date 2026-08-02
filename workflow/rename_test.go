package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const renameTargetBlueprint = `workflow:
  id: rename_target
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

const renameEmbedderBlueprint = `workflow:
  id: embedder
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
    workflow_id: rename_target
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

func writeRenameFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", home)

	registry := filepath.Join(home, "workflows")
	if err := os.MkdirAll(registry, 0755); err != nil {
		t.Fatal(err)
	}
	err := os.WriteFile(
		filepath.Join(registry, "rename_target.yaml"),
		[]byte(renameTargetBlueprint),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(
		filepath.Join(registry, "embedder.yaml"),
		[]byte(renameEmbedderBlueprint),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	return home
}

func TestRenameWorkflowIDCascades(t *testing.T) {
	home := writeRenameFixture(t)

	// A pending draft and archived version should follow the rename.
	draft := strings.Replace(
		renameTargetBlueprint,
		"instruction: Echo the text.",
		"instruction: Echo the text twice.",
		1,
	)
	if err := WriteDraft("rename_target", []byte(draft)); err != nil {
		t.Fatal(err)
	}
	versionsDir := filepath.Join(home, "versions", "rename_target")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	err := os.WriteFile(
		filepath.Join(versionsDir, "v1.yaml"),
		[]byte(renameTargetBlueprint),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := RenameWorkflowID("rename_target", "renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowID != "renamed_target" {
		t.Fatalf("expected renamed_target, got %s", result.WorkflowID)
	}
	if len(result.UpdatedRefs) != 1 || result.UpdatedRefs[0] != "embedder" {
		t.Fatalf("expected embedder in updated refs, got %v", result.UpdatedRefs)
	}

	// Registry: old file gone, new file carries the new id.
	registry := filepath.Join(home, "workflows")
	if _, err := os.Stat(filepath.Join(registry, "rename_target.yaml")); !os.IsNotExist(err) {
		t.Fatal("old registry file should be gone")
	}
	installed, err := os.ReadFile(filepath.Join(registry, "renamed_target.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "id: renamed_target") {
		t.Fatal("renamed file should carry the new id")
	}

	// The embedder now references the new id.
	embedder, err := os.ReadFile(filepath.Join(registry, "embedder.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(embedder), "workflow_id: renamed_target") {
		t.Fatal("embedder should reference the new id")
	}

	// Draft moved and rewritten.
	oldDraft, err := ReadDraft("rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if oldDraft != "" {
		t.Fatal("old draft should be gone")
	}
	newDraft, err := ReadDraft("renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(newDraft, "id: renamed_target") ||
		!strings.Contains(newDraft, "twice") {
		t.Fatal("draft should follow the rename with its content intact")
	}

	// Versions archive moved.
	if _, err := os.Stat(filepath.Join(home, "versions", "renamed_target", "v1.yaml")); err != nil {
		t.Fatal("versions archive should move to the new id")
	}
}

func TestRenameWorkflowIDRejectsCollision(t *testing.T) {
	writeRenameFixture(t)

	_, err := RenameWorkflowID("rename_target", "embedder")
	if err == nil {
		t.Fatal("expected a collision error")
	}
}

func TestSetWorkflowDisplayName(t *testing.T) {
	home := writeRenameFixture(t)

	err := SetWorkflowDisplayName("rename_target", "Fancy New Name")
	if err != nil {
		t.Fatal(err)
	}

	installed, err := os.ReadFile(
		filepath.Join(home, "workflows", "rename_target.yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "Fancy New Name") {
		t.Fatal("display name should be rewritten")
	}
	if !strings.Contains(string(installed), "id: rename_target") {
		t.Fatal("the id must not change on a name edit")
	}
}

func TestUUIDSurvivesSaveAndRename(t *testing.T) {
	home := writeRenameFixture(t)

	// First save mints the uuid.
	if err := WriteDraft("rename_target", []byte(renameTargetBlueprint)); err != nil {
		t.Fatal(err)
	}
	saved, err := SaveDraft("rename_target")
	if err != nil {
		t.Fatal(err)
	}
	first, err := LoadBlueprintByWorkflowID("rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if first.Workflow.UUID == "" {
		t.Fatal("save should mint a uuid")
	}
	_ = saved

	// A second save keeps it, even if the draft carries a bogus one.
	bogus := strings.Replace(
		renameTargetBlueprint,
		"version: \"1\"",
		"version: \"2\"\n  uuid: 00000000-0000-0000-0000-000000000bad",
		1,
	)
	if err := WriteDraft("rename_target", []byte(bogus)); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDraft("rename_target"); err != nil {
		t.Fatal(err)
	}
	second, err := LoadBlueprintByWorkflowID("rename_target")
	if err != nil {
		t.Fatal(err)
	}
	if second.Workflow.UUID != first.Workflow.UUID {
		t.Fatalf("uuid changed on save: %s -> %s", first.Workflow.UUID, second.Workflow.UUID)
	}

	// A slug rename keeps it too.
	if _, err := RenameWorkflowID("rename_target", "renamed_target"); err != nil {
		t.Fatal(err)
	}
	renamed, err := LoadBlueprintByWorkflowID("renamed_target")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Workflow.UUID != first.Workflow.UUID {
		t.Fatalf("uuid changed on rename: %s -> %s", first.Workflow.UUID, renamed.Workflow.UUID)
	}
	_ = home
}

func TestCompileRejectsReservedInstanceIDs(t *testing.T) {
	writeRenameFixture(t)

	// "step" -> "workflow-inputs" collides with the canvas's
	// synthetic node id and must not compile.
	bad := strings.Replace(
		renameTargetBlueprint,
		"    step:",
		"    workflow-inputs:",
		1,
	)
	if err := WriteDraft("rename_target", []byte(bad)); err != nil {
		t.Fatal(err)
	}
	_, err := SaveDraft("rename_target")
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected a reserved-id compile error, got %v", err)
	}
}
