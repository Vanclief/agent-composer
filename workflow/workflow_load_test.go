package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBlueprintByWorkflowID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	path := filepath.Join(workflowDir, "registry-summary.yaml")
	err = os.WriteFile(path, []byte(`
workflow:
  id: registry_summary
  version: "1"
  inputs:
    article_text: string
  outputs:
    out:
      schema: string
      from: instance.summarize.out

nodes:
  summarize:
    kind: inference
    inputs:
      article_text: string
    outputs:
      out: string
    config:
      harness:
        id: codex_cli
        model: gpt-5.4-mini
        reasoning_effort: medium
      instruction: >
        Summarize the article.

flow:
  instances:
    summarize:
      node: summarize
      inputs:
        article_text: workflow_input.article_text
`), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	blueprint, err := LoadBlueprintByWorkflowID("registry_summary")
	if err != nil {
		t.Fatalf("load by workflow id: %v", err)
	}

	if blueprint.Workflow.ID != "registry_summary" {
		t.Fatalf("unexpected workflow id: %q", blueprint.Workflow.ID)
	}

	if blueprint.SourcePath != path {
		t.Fatalf("unexpected source path: %q", blueprint.SourcePath)
	}
}
