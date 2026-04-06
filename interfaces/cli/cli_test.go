package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWorkflowInputFromString(t *testing.T) {
	path := writeWorkflowTestFile(t, `
workflow:
  id: single_string_input
  version: "1"
  inputs:
    question: string
  outputs: {}
nodes: {}
flow:
  instances: {}
`)

	input, err := loadWorkflowInput("", path, "", "", "Are cats blue?", false, false, true)
	if err != nil {
		t.Fatalf("load workflow input: %v", err)
	}

	value, found := input["question"]
	if !found {
		t.Fatalf("question input missing: %#v", input)
	}

	if value != "Are cats blue?" {
		t.Fatalf("unexpected question input: %#v", value)
	}
}

func TestLoadWorkflowInputRejectsMultipleSources(t *testing.T) {
	path := writeWorkflowTestFile(t, `
workflow:
  id: single_string_input
  version: "1"
  inputs:
    question: string
  outputs: {}
nodes: {}
flow:
  instances: {}
`)

	_, err := loadWorkflowInput("", path, "", `{"question":"json"}`, "string", false, true, true)
	if err == nil {
		t.Fatal("expected error for multiple input sources")
	}
}

func TestLoadWorkflowInputRejectsStringForMultipleInputs(t *testing.T) {
	path := writeWorkflowTestFile(t, `
workflow:
  id: multiple_inputs
  version: "1"
  inputs:
    question: string
    tone: string
  outputs: {}
nodes: {}
flow:
  instances: {}
`)

	_, err := loadWorkflowInput("", path, "", "", "Are cats blue?", false, false, true)
	if err == nil {
		t.Fatal("expected error for multiple workflow inputs")
	}
}

func TestLoadWorkflowInputRejectsStringForNonStringInput(t *testing.T) {
	path := writeWorkflowTestFile(t, `
workflow:
  id: non_string_input
  version: "1"
  inputs:
    retries: integer
  outputs: {}
nodes: {}
flow:
  instances: {}
`)

	_, err := loadWorkflowInput("", path, "", "", "3", false, false, true)
	if err == nil {
		t.Fatal("expected error for non-string workflow input")
	}
}

func writeWorkflowTestFile(t *testing.T, raw string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")

	err := os.WriteFile(path, []byte(raw), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	return path
}
