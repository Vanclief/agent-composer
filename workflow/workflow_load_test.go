package workflow

import (
	"fmt"
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

func TestListBlueprints(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	firstPath := filepath.Join(workflowDir, "binary-vote.yaml")
	err = os.WriteFile(firstPath, []byte(`
workflow:
  id: binary_vote_round
  version: "1"
  description: Collect binary votes from multiple agents.
  inputs:
    question: string
  outputs:
    consensus:
      schema: binary_vote
      from: collector.consensus

nodes:
  collector:
    kind: connector
    operation: collect_binary_votes
    inputs:
      question: string
    outputs:
      consensus: binary_vote

flow:
  instances:
    collector:
      node: collector
      inputs:
        question: workflow_input.question
`), 0644)
	if err != nil {
		t.Fatalf("write first workflow file: %v", err)
	}

	secondPath := filepath.Join(workflowDir, "summary.yaml")
	err = os.WriteFile(secondPath, []byte(`
workflow:
  id: article_summary
  version: "1"
  description: Summarize an article.
  inputs:
    article_text: string
  outputs:
    summary:
      schema: string
      from: summarize.out

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
		t.Fatalf("write second workflow file: %v", err)
	}

	summaries, err := ListBlueprints()
	if err != nil {
		t.Fatalf("list blueprints: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("unexpected workflow count: %d", len(summaries))
	}

	if summaries[0].ID != "article_summary" {
		t.Fatalf("unexpected first workflow id: %q", summaries[0].ID)
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

	if summaries[1].ID != "binary_vote_round" {
		t.Fatalf("unexpected second workflow id: %q", summaries[1].ID)
	}

	if summaries[1].Inputs["question"] != "string" {
		t.Fatalf("unexpected second workflow input schema: %q", summaries[1].Inputs["question"])
	}

	if summaries[1].Outputs["consensus"] != "binary_vote" {
		t.Fatalf("unexpected second workflow output schema: %q", summaries[1].Outputs["consensus"])
	}
}

func TestImportBlueprintFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	err := os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "binary-vote.yaml")
	content := binaryVoteWorkflowYAML("Collect binary votes from multiple agents.")
	err = os.WriteFile(sourcePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("write source workflow file: %v", err)
	}

	summary, err := ImportBlueprintFile(sourcePath, false)
	if err != nil {
		t.Fatalf("import workflow: %v", err)
	}

	if summary.ID != "binary_vote_round" {
		t.Fatalf("unexpected imported workflow id: %q", summary.ID)
	}

	registryPath := filepath.Join(tempDir, defaultWorkflowSubdir, "binary_vote_round.yaml")
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read imported workflow file: %v", err)
	}

	if string(raw) != content {
		t.Fatalf("unexpected imported workflow content: %q", string(raw))
	}

	blueprint, err := LoadBlueprintByWorkflowID("binary_vote_round")
	if err != nil {
		t.Fatalf("load imported workflow by id: %v", err)
	}

	if blueprint.SourcePath != registryPath {
		t.Fatalf("unexpected imported source path: %q", blueprint.SourcePath)
	}
}

func TestImportBlueprintFileOverwriteCanonicalizesPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	legacyPath := filepath.Join(workflowDir, "legacy-name.yaml")
	err = os.WriteFile(legacyPath, []byte(binaryVoteWorkflowYAML("Old description.")), 0644)
	if err != nil {
		t.Fatalf("write legacy workflow file: %v", err)
	}

	sourceDir := filepath.Join(tempDir, "source")
	err = os.MkdirAll(sourceDir, 0755)
	if err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	sourcePath := filepath.Join(sourceDir, "binary-vote.yaml")
	newContent := binaryVoteWorkflowYAML("New description.")
	err = os.WriteFile(sourcePath, []byte(newContent), 0644)
	if err != nil {
		t.Fatalf("write new workflow file: %v", err)
	}

	_, err = ImportBlueprintFile(sourcePath, true)
	if err != nil {
		t.Fatalf("overwrite import workflow: %v", err)
	}

	_, err = os.Stat(legacyPath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected legacy path to be removed, got: %v", err)
	}

	registryPath := filepath.Join(workflowDir, "binary_vote_round.yaml")
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read canonical workflow file: %v", err)
	}

	if string(raw) != newContent {
		t.Fatalf("unexpected canonical workflow content: %q", string(raw))
	}

	blueprint, err := LoadBlueprintByWorkflowID("binary_vote_round")
	if err != nil {
		t.Fatalf("load overwritten workflow by id: %v", err)
	}

	if blueprint.SourcePath != registryPath {
		t.Fatalf("unexpected canonical source path: %q", blueprint.SourcePath)
	}

	if blueprint.Workflow.Description != "New description." {
		t.Fatalf("unexpected canonical description: %q", blueprint.Workflow.Description)
	}
}

func TestDeleteBlueprintByWorkflowID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	path := filepath.Join(workflowDir, "binary_vote_round.yaml")
	err = os.WriteFile(path, []byte(binaryVoteWorkflowYAML("Collect binary votes from multiple agents.")), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	err = DeleteBlueprintByWorkflowID("binary_vote_round")
	if err != nil {
		t.Fatalf("delete workflow by id: %v", err)
	}

	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected workflow file to be removed, got: %v", err)
	}
}

func TestDeleteBlueprintByWorkflowIDRemovesDuplicateEntries(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	firstPath := filepath.Join(workflowDir, "binary_vote_round.yaml")
	err = os.WriteFile(firstPath, []byte(binaryVoteWorkflowYAML("First description.")), 0644)
	if err != nil {
		t.Fatalf("write first workflow file: %v", err)
	}

	secondPath := filepath.Join(workflowDir, "legacy-binary-vote.yaml")
	err = os.WriteFile(secondPath, []byte(binaryVoteWorkflowYAML("Second description.")), 0644)
	if err != nil {
		t.Fatalf("write second workflow file: %v", err)
	}

	err = DeleteBlueprintByWorkflowID("binary_vote_round")
	if err != nil {
		t.Fatalf("delete duplicate workflow entries by id: %v", err)
	}

	_, err = os.Stat(firstPath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected first duplicate workflow file to be removed, got: %v", err)
	}

	_, err = os.Stat(secondPath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected second duplicate workflow file to be removed, got: %v", err)
	}
}

func TestExportBlueprintByWorkflowID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	content := binaryVoteWorkflowYAML("Collect binary votes from multiple agents.")
	registryPath := filepath.Join(workflowDir, "binary_vote_round.yaml")
	err = os.WriteFile(registryPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	targetPath := filepath.Join(tempDir, "exports", "binary_vote_round.yaml")
	err = os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		t.Fatalf("mkdir export dir: %v", err)
	}

	err = ExportBlueprintByWorkflowID("binary_vote_round", targetPath, false)
	if err != nil {
		t.Fatalf("export workflow by id: %v", err)
	}

	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read exported workflow file: %v", err)
	}

	if string(raw) != content {
		t.Fatalf("unexpected exported workflow content: %q", string(raw))
	}
}

func TestReadBlueprintBytesByWorkflowID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(workflowHomeEnvVar, tempDir)

	workflowDir := filepath.Join(tempDir, defaultWorkflowSubdir)
	err := os.MkdirAll(workflowDir, 0755)
	if err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	content := binaryVoteWorkflowYAML("Collect binary votes from multiple agents.")
	registryPath := filepath.Join(workflowDir, "binary_vote_round.yaml")
	err = os.WriteFile(registryPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("write workflow file: %v", err)
	}

	raw, err := ReadBlueprintBytesByWorkflowID("binary_vote_round")
	if err != nil {
		t.Fatalf("read workflow bytes by id: %v", err)
	}

	if string(raw) != content {
		t.Fatalf("unexpected workflow bytes: %q", string(raw))
	}
}

func binaryVoteWorkflowYAML(description string) string {
	return fmt.Sprintf(`workflow:
  id: binary_vote_round
  version: "1"
  description: %s
  inputs:
    question: string
  outputs:
    consensus:
      schema: binary_vote
      from: collector.consensus

nodes:
  collector:
    kind: connector
    operation: collect_binary_votes
    inputs:
      question: string
    outputs:
      consensus: binary_vote

flow:
  instances:
    collector:
      node: collector
      inputs:
        question: workflow_input.question
`, description)
}
