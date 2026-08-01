package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const draftTestBlueprint = `workflow:
  id: test-wf
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

func TestSaveDraftPromotesAndArchives(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", home)

	registry := filepath.Join(home, "workflows")
	if err := os.MkdirAll(registry, 0755); err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(registry, "test-wf.yaml")
	if err := os.WriteFile(installedPath, []byte(draftTestBlueprint), 0644); err != nil {
		t.Fatal(err)
	}

	draft := strings.Replace(
		draftTestBlueprint,
		"instruction: Echo the text.",
		"instruction: Echo the text twice.",
		1,
	)
	if err := WriteDraft("test-wf", []byte(draft)); err != nil {
		t.Fatal(err)
	}

	saved, err := SaveDraft("test-wf")
	if err != nil {
		t.Fatal(err)
	}

	if saved.Version != "2" {
		t.Fatalf("version = %q, want 2", saved.Version)
	}

	installed, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "Echo the text twice.") {
		t.Fatal("registry file was not replaced with the draft")
	}
	if !strings.Contains(string(installed), `version: "2"`) {
		t.Fatalf("registry file was not stamped with version 2:\n%s", installed)
	}

	archived, err := os.ReadFile(
		filepath.Join(home, "versions", "test-wf", "v1.yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archived), "instruction: Echo the text.") {
		t.Fatal("archive does not hold the outgoing version")
	}

	remaining, err := ReadDraft("test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if remaining != "" {
		t.Fatal("draft should be deleted after save")
	}
}

func TestSaveDraftFirstInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", home)

	if err := WriteDraft("test-wf", []byte(draftTestBlueprint)); err != nil {
		t.Fatal(err)
	}

	saved, err := SaveDraft("test-wf")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != "1" {
		t.Fatalf("version = %q, want 1", saved.Version)
	}

	if _, err := os.Stat(
		filepath.Join(home, "workflows", "test-wf.yaml"),
	); err != nil {
		t.Fatal("first save should install into the registry")
	}

	if _, err := os.Stat(filepath.Join(home, "versions", "test-wf")); !os.IsNotExist(err) {
		t.Fatal("first save has no outgoing version to archive")
	}
}

func TestSaveDraftRejectsBrokenDraft(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", home)

	broken := strings.Replace(
		draftTestBlueprint,
		"from: instance.step.out",
		"from: instance.missing.out",
		1,
	)
	if err := WriteDraft("test-wf", []byte(broken)); err != nil {
		t.Fatal(err)
	}

	_, err := SaveDraft("test-wf")
	if err == nil {
		t.Fatal("a draft that does not compile must not save")
	}

	if _, statErr := os.Stat(
		filepath.Join(home, "workflows", "test-wf.yaml"),
	); !os.IsNotExist(statErr) {
		t.Fatal("a failed save must not touch the registry")
	}
}
