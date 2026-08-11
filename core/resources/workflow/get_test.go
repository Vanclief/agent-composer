package workflow

import (
	"strings"
	"testing"
)

func TestGetReturnsWorkflowSpec(t *testing.T) {
	api, ctx := newTestAPI(t)

	spec := `
workflow:
  slug: get_workflow
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

	installSpec(t, ctx, api, "get_workflow", spec)

	response, err := api.Get(ctx, nil, &GetRequest{
		WorkflowSlug: "get_workflow",
	})
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}

	if response.WorkflowSlug != "get_workflow" {
		t.Fatalf("unexpected workflow id: %q", response.WorkflowSlug)
	}

	if strings.TrimSpace(response.Spec) != strings.TrimSpace(spec) {
		t.Fatalf("unexpected workflow spec: %q", response.Spec)
	}
}
