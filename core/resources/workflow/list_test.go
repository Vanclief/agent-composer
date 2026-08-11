package workflow

import (
	"testing"
)

func TestListReturnsInstalledWorkflows(t *testing.T) {
	api, ctx := newTestAPI(t)

	installSpec(t, ctx, api, "beta", `
workflow:
  slug: beta
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
`)

	installSpec(t, ctx, api, "alpha", `
workflow:
  slug: alpha
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
`)

	response, err := api.List(ctx, nil, &ListRequest{})
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}

	if len(response.Workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(response.Workflows))
	}

	if response.Workflows[0].Slug != "alpha" {
		t.Fatalf("expected alpha first, got %q", response.Workflows[0].Slug)
	}

	if response.Workflows[1].Slug != "beta" {
		t.Fatalf("expected beta second, got %q", response.Workflows[1].Slug)
	}

	if response.Workflows[0].Inputs["title"] != "string" {
		t.Fatalf("unexpected alpha inputs: %#v", response.Workflows[0].Inputs)
	}

	if response.Workflows[1].Outputs["summary"] != "string" {
		t.Fatalf("unexpected beta outputs: %#v", response.Workflows[1].Outputs)
	}
}
