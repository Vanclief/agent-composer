package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListReturnsInstalledWorkflowBlueprints(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", homeDir)

	workflowDir := filepath.Join(homeDir, "workflows")
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	err = os.WriteFile(filepath.Join(workflowDir, "beta.yaml"), []byte(`
workflow:
  id: beta
  version: "1"
  description: Beta workflow.
  inputs:
    topic: string
  outputs:
    summary:
      schema: string
      from: summarize.out
nodes:
  summarize:
    kind: inference
    outputs:
      out: string
    config:
      instruction: Summarize.
flow:
  instances:
    summarize:
      node: summarize
      inputs:
        topic: workflow_input.topic
`), 0644)
	if err != nil {
		t.Fatalf("write beta workflow: %v", err)
	}

	err = os.WriteFile(filepath.Join(workflowDir, "alpha.yaml"), []byte(`
workflow:
  id: alpha
  version: "1"
  description: Alpha workflow.
  inputs:
    title: string
  outputs:
    slug:
      schema: string
      from: slugify.out
nodes:
  slugify:
    kind: inference
    outputs:
      out: string
    config:
      instruction: Slugify.
flow:
  instances:
    slugify:
      node: slugify
      inputs:
        title: workflow_input.title
`), 0644)
	if err != nil {
		t.Fatalf("write alpha workflow: %v", err)
	}

	api := &API{}
	response, err := api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}

	if len(response.Workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(response.Workflows))
	}

	if response.Workflows[0].ID != "alpha" {
		t.Fatalf("expected alpha first, got %q", response.Workflows[0].ID)
	}

	if response.Workflows[1].ID != "beta" {
		t.Fatalf("expected beta second, got %q", response.Workflows[1].ID)
	}

	if response.Workflows[0].Inputs["title"] != "string" {
		t.Fatalf("unexpected alpha inputs: %#v", response.Workflows[0].Inputs)
	}

	if response.Workflows[1].Outputs["summary"] != "string" {
		t.Fatalf("unexpected beta outputs: %#v", response.Workflows[1].Outputs)
	}
}
