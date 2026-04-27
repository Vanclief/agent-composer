package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetReturnsWorkflowBlueprintSpec(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("AGENT_COMPOSER_HOME", homeDir)

	workflowDir := filepath.Join(homeDir, "workflows")
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	spec := `
workflow:
  id: get_workflow
  version: "1"
  description: Get workflow.
  inputs:
    prompt: string
  outputs:
    answer:
      schema: string
      from: answer.out
nodes:
  answer:
    kind: inference
    outputs:
      out: string
    config:
      instruction: Answer.
flow:
  instances:
    answer:
      node: answer
      inputs:
        prompt: workflow_input.prompt
`

	err = os.WriteFile(filepath.Join(workflowDir, "get-workflow.yaml"), []byte(spec), 0644)
	if err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	api := &API{}
	response, err := api.Get(context.Background(), nil, &GetRequest{
		WorkflowID: "get_workflow",
	})
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}

	if response.WorkflowID != "get_workflow" {
		t.Fatalf("unexpected workflow id: %q", response.WorkflowID)
	}

	if strings.TrimSpace(response.Spec) != strings.TrimSpace(spec) {
		t.Fatalf("unexpected workflow spec: %q", response.Spec)
	}
}
