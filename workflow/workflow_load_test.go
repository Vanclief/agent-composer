package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanclief/ez"
)

func TestRegistryImportAndLoad(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	summary := importYAML(t, ctx, registry, binaryVoteWorkflowYAML("Collect binary votes from multiple agents."))

	if summary.Slug != "binary_vote_round" {
		t.Fatalf("unexpected imported workflow slug: %q", summary.Slug)
	}
	if summary.ID == "" {
		t.Fatal("import should mint a permanent uuid")
	}
	if summary.Version != "1" {
		t.Fatalf("first import should install version 1, got %q", summary.Version)
	}

	spec, err := registry.Load(ctx, "binary_vote_round")
	if err != nil {
		t.Fatalf("load by workflow id: %v", err)
	}
	if spec.Workflow.Slug != "binary_vote_round" {
		t.Fatalf("unexpected workflow id: %q", spec.Workflow.Slug)
	}
	if spec.Workflow.ID != summary.ID {
		t.Fatalf("the stored spec should carry the minted uuid, got %q", spec.Workflow.ID)
	}
}

func TestRegistryList(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	importYAML(t, ctx, registry, binaryVoteWorkflowYAML("Collect binary votes from multiple agents."))
	importYAML(t, ctx, registry, `workflow:
  slug: article_summary
  version: "1"
  description: Summarize an article.
  inputs:
    article_text: string
  outputs:
    summary:
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
`)

	summaries, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("list specs: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("unexpected workflow count: %d", len(summaries))
	}

	if summaries[0].Slug != "article_summary" {
		t.Fatalf("unexpected first workflow id: %q", summaries[0].Slug)
	}
	if summaries[0].Description != "Summarize an article." {
		t.Fatalf("unexpected first workflow description: %q", summaries[0].Description)
	}
	if summaries[0].Inputs["article_text"] != "string" {
		t.Fatalf("unexpected first workflow input schema: %q", summaries[0].Inputs["article_text"])
	}
	if summaries[0].Outputs["summary"] != "string" {
		t.Fatalf("unexpected first workflow output schema: %q", summaries[0].Outputs["summary"])
	}

	if summaries[1].Slug != "binary_vote_round" {
		t.Fatalf("unexpected second workflow id: %q", summaries[1].Slug)
	}
	if summaries[1].Inputs["question"] != "string" {
		t.Fatalf("unexpected second workflow input schema: %q", summaries[1].Inputs["question"])
	}
	if summaries[1].Outputs["consensus"] != "TextList" {
		t.Fatalf("unexpected second workflow output schema: %q", summaries[1].Outputs["consensus"])
	}
}

func TestRegistryImportOverwriteContinuesHistory(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	first := importYAML(t, ctx, registry, binaryVoteWorkflowYAML("First description."))

	path := filepath.Join(t.TempDir(), "updated.yaml")
	err := os.WriteFile(path, []byte(binaryVoteWorkflowYAML("Second description.")), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.ImportFile(ctx, path, false)
	if err == nil {
		t.Fatal("importing over an installed workflow must require overwrite")
	}

	second, err := registry.ImportFile(ctx, path, true)
	if err != nil {
		t.Fatalf("overwrite import: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("overwrite must keep the workflow identity: %q -> %q", first.ID, second.ID)
	}
	if second.Version != "2" {
		t.Fatalf("overwrite should continue the version counter, got %q", second.Version)
	}

	versions, err := registry.ListVersions(ctx, "binary_vote_round")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(versions))
	}
	if versions[0].Version != 2 || !versions[0].Current {
		t.Fatalf("newest history entry should be the current version 2: %+v", versions[0])
	}
	if versions[1].Version != 1 || versions[1].Current {
		t.Fatalf("oldest history entry should be the non-current version 1: %+v", versions[1])
	}
}

func TestRegistryDelete(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	importYAML(t, ctx, registry, binaryVoteWorkflowYAML("Collect binary votes from multiple agents."))

	err := registry.Delete(ctx, "binary_vote_round")
	if err != nil {
		t.Fatalf("delete workflow by id: %v", err)
	}

	_, err = registry.Load(ctx, "binary_vote_round")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected the workflow to be gone, got: %v", err)
	}
}

func TestRegistryExport(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	importYAML(t, ctx, registry, binaryVoteWorkflowYAML("Collect binary votes from multiple agents."))

	targetPath := filepath.Join(t.TempDir(), "exports", "binary_vote_round.yaml")
	err := os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = registry.ExportToFile(ctx, "binary_vote_round", targetPath, false)
	if err != nil {
		t.Fatalf("export workflow by id: %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}

	stored, err := registry.SpecBytes(ctx, "binary_vote_round")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(stored) {
		t.Fatalf("exported bytes should match the stored spec:\n%s", raw)
	}

	err = registry.ExportToFile(ctx, "binary_vote_round", targetPath, false)
	if err == nil {
		t.Fatal("exporting onto an existing file must require overwrite")
	}
}

func TestRegistryRestoreVersion(t *testing.T) {
	registry, ctx := newTestRegistry(t)

	importYAML(t, ctx, registry, binaryVoteWorkflowYAML("First description."))

	path := filepath.Join(t.TempDir(), "updated.yaml")
	err := os.WriteFile(path, []byte(binaryVoteWorkflowYAML("Second description.")), 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.ImportFile(ctx, path, true)
	if err != nil {
		t.Fatal(err)
	}

	restored, err := registry.RestoreVersion(ctx, "binary_vote_round", 1)
	if err != nil {
		t.Fatalf("restore version: %v", err)
	}
	if restored.Version != "3" {
		t.Fatalf("a restore should install a new head version 3, got %q", restored.Version)
	}
	if !strings.Contains(restored.Spec, "First description.") {
		t.Fatal("the restored head should carry the old content")
	}

	versions, err := registry.ListVersions(ctx, "binary_vote_round")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("a restore must extend the history, not rewrite it: %d entries", len(versions))
	}
}

func binaryVoteWorkflowYAML(description string) string {
	return fmt.Sprintf(`workflow:
  slug: binary_vote_round
  version: "1"
  description: %s
  inputs:
    question: string
  outputs:
    consensus:
      schema: TextList
      from: instance.collector.out

schemas:
  TextList:
    type: array
    items:
      type: string

nodes:
  collector:
    kind: connector
    operation: collect
    inputs:
      question: string
    outputs:
      out: TextList

flow:
  instances:
    collector:
      node: collector
      inputs:
        question: workflow_input.question
`, description)
}
