package workflow

import (
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

const draftTestSpec = `workflow:
  slug: test-wf
  name: "Test"
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

func TestSaveDraftPromotesAndKeepsHistory(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	importYAML(t, ctx, registry, draftTestSpec)

	draft := strings.Replace(
		draftTestSpec,
		"instruction: Echo the text.",
		"instruction: Echo the text twice.",
		1,
	)
	err := registry.WriteDraft(ctx, "test-wf", []byte(draft))
	if err != nil {
		t.Fatal(err)
	}

	saved, err := registry.SaveDraft(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}

	if saved.Version != "2" {
		t.Fatalf("version = %q, want 2", saved.Version)
	}

	installed, err := registry.SpecBytes(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "Echo the text twice.") {
		t.Fatal("the head was not replaced with the draft")
	}
	if !strings.Contains(string(installed), `version: "2"`) {
		t.Fatalf("the head was not stamped with version 2:\n%s", installed)
	}

	outgoing, err := registry.GetVersionSpec(ctx, "test-wf", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(outgoing, "instruction: Echo the text.") {
		t.Fatal("the history does not hold the outgoing version")
	}

	remaining, err := registry.ReadDraft(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if remaining != "" {
		t.Fatal("draft should be deleted after save")
	}
}

func TestSaveDraftFirstInstall(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	err := registry.WriteDraft(ctx, "test-wf", []byte(draftTestSpec))
	if err != nil {
		t.Fatal(err)
	}

	saved, err := registry.SaveDraft(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != "1" {
		t.Fatalf("version = %q, want 1", saved.Version)
	}

	_, err = registry.Load(ctx, "test-wf")
	if err != nil {
		t.Fatal("first save should install the workflow")
	}

	versions, err := registry.ListVersions(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("first save should record exactly one version, got %d", len(versions))
	}
}

func TestSaveDraftRejectsBrokenDraft(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	broken := strings.Replace(
		draftTestSpec,
		"from: instance.step.out",
		"from: instance.missing.out",
		1,
	)
	err := registry.WriteDraft(ctx, "test-wf", []byte(broken))
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.SaveDraft(ctx, "test-wf")
	if err == nil {
		t.Fatal("a draft that does not compile must not save")
	}

	_, err = registry.Load(ctx, "test-wf")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("a failed save must not install anything, got: %v", err)
	}

	remaining, err := registry.ReadDraft(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if remaining == "" {
		t.Fatal("a failed save must keep the draft for another attempt")
	}
}
