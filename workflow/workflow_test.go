package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
)

func TestCompilePipelineSummaryCritiqueRevise(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/article_summary.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "article_summary" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	expectedOrder := []string{"summarize_article", "critique_summary", "revise_summary"}
	if len(snapshot.Order) != len(expectedOrder) {
		t.Fatalf("unexpected node order length: %#v", snapshot.Order)
	}

	for index, expected := range expectedOrder {
		if snapshot.Order[index] != expected {
			t.Fatalf("unexpected node order at %d: %#v", index, snapshot.Order)
		}
	}
}

func TestExecutePipelineSummaryCritiqueRevise(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/article_summary.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return fakeHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"article_text": "A short article about a new bridge opening downtown.",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	finalSummary, ok := output["final_summary"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected final summary shape: %#v", output["final_summary"])
	}

	if finalSummary["text"] != "Revised summary" {
		t.Fatalf("unexpected final summary text: %#v", finalSummary["text"])
	}

	critique, ok := output["critique"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected critique shape: %#v", output["critique"])
	}

	if critique["is_accurate"] != false {
		t.Fatalf("unexpected critique accuracy: %#v", critique["is_accurate"])
	}
}

func TestCompileConnectorCollectBinaryVotes(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/binary_vote_round.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "binary_vote_round" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	if snapshot.Nodes["collect_votes"].Kind != "connector" {
		t.Fatalf("expected collect_votes to compile as connector")
	}

	if snapshot.Nodes["collect_votes"].Operation != "collect" {
		t.Fatalf("unexpected connector operation: %q", snapshot.Nodes["collect_votes"].Operation)
	}
}

func TestExecuteConnectorCollectBinaryVotes(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/binary_vote_round.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return voteHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"question": "Should we deploy today?",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	votes, ok := output["votes"].([]any)
	if !ok {
		t.Fatalf("unexpected votes shape: %#v", output["votes"])
	}

	if len(votes) != 7 {
		t.Fatalf("unexpected vote count: %d", len(votes))
	}

	firstVote, ok := votes[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first vote shape: %#v", votes[0])
	}

	if firstVote["answer"] != "yes" {
		t.Fatalf("unexpected first vote answer: %#v", firstVote["answer"])
	}
}

func TestExecuteConnectorCollectPreservesDeclarationOrder(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "collect-order.yaml")

	err := os.WriteFile(path, []byte(`
workflow:
  id: collect_order
  version: "1"
  inputs:
    first: string
    second: string
  outputs:
    out:
      schema: StringList
      from: instance.collect_values.out

schemas:
  StringList:
    type: array
    items:
      type: string

nodes:
  collect_values:
    kind: connector
    operation: collect
    inputs:
      second: string
      first: string
    outputs:
      out: StringList

flow:
  instances:
    collect_values:
      node: collect_values
      inputs:
        second: workflow_input.second
        first: workflow_input.first
`), 0644)
	if err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	blueprint, err := LoadBlueprintFile(path)
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"first":  "A",
		"second": "B",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	values, ok := output["out"].([]any)
	if !ok {
		t.Fatalf("unexpected collect output shape: %#v", output["out"])
	}

	if len(values) != 2 {
		t.Fatalf("unexpected collect output length: %d", len(values))
	}

	if values[0] != "B" || values[1] != "A" {
		t.Fatalf("unexpected collect output order: %#v", values)
	}
}

func TestCompileConnectorPack(t *testing.T) {
	blueprint := &Blueprint{
		Workflow: WorkflowSpec{
			ID:      "connector_pack",
			Version: "1",
			Inputs: map[string]string{
				"title":   "string",
				"content": "string",
			},
			Outputs: map[string]WorkflowOutputSpec{
				"out": {
					Schema: "Draft",
					From:   "instance.pack_draft.out",
				},
			},
		},
		Schemas: map[string]SchemaSpec{
			"Draft": {
				Type: "object",
				Properties: map[string]SchemaSpec{
					"title":   {Type: "string"},
					"content": {Type: "string"},
				},
			},
		},
		Nodes: map[string]NodeSpec{
			"draft_packer": {
				Kind:      "connector",
				Operation: "pack",
				Inputs: map[string]string{
					"title":   "string",
					"content": "string",
				},
				Outputs: map[string]string{
					"out": "Draft",
				},
			},
		},
		Flow: FlowSpec{
			Instances: map[string]InstanceSpec{
				"pack_draft": {
					Node: "draft_packer",
					Inputs: map[string]string{
						"title":   "workflow_input.title",
						"content": "workflow_input.content",
					},
				},
			},
		},
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	packNode, found := snapshot.Nodes["pack_draft"]
	if !found {
		t.Fatalf("expected pack_draft node to exist")
	}

	if packNode.Kind != "connector" {
		t.Fatalf("expected pack_draft to compile as connector")
	}

	if packNode.Operation != "pack" {
		t.Fatalf("unexpected connector operation: %q", packNode.Operation)
	}
}

func TestCompileRejectsOptionalObjectFieldsInInferenceStructuredOutputs(t *testing.T) {
	blueprint := &Blueprint{
		Workflow: WorkflowSpec{
			ID:      "optional_structured_output",
			Version: "1",
			Inputs: map[string]string{
				"request": "string",
			},
			Outputs: map[string]WorkflowOutputSpec{
				"out": {
					Schema: "ReviewIssueList",
					From:   "instance.review.out",
				},
			},
		},
		Schemas: map[string]SchemaSpec{
			"ReviewIssue": {
				Type: "object",
				Properties: map[string]SchemaSpec{
					"title":                {Type: "string"},
					"requires_human_input": {Type: "boolean"},
					"question_for_human": {
						Type:     "string",
						Optional: true,
					},
				},
			},
			"ReviewIssueList": {
				Type: "array",
				Items: &SchemaSpec{
					SchemaRef: "ReviewIssue",
				},
			},
		},
		Nodes: map[string]NodeSpec{
			"reviewer": {
				Kind: "inference",
				Inputs: map[string]string{
					"request": "string",
				},
				Outputs: map[string]string{
					"out": "ReviewIssueList",
				},
				Config: InferenceNodeConfig{
					Harness: map[string]any{
						"id":    "codex_cli",
						"model": "gpt-5.4-mini",
					},
					Instruction: "Review the request and return issues.",
				},
			},
		},
		Flow: FlowSpec{
			Instances: map[string]InstanceSpec{
				"review": {
					Node: "reviewer",
					Inputs: map[string]string{
						"request": "workflow_input.request",
					},
				},
			},
		},
	}

	_, err := Compile(blueprint)
	if err == nil {
		t.Fatal("expected compile to reject optional structured output fields")
	}

	if !strings.Contains(err.Error(), "optional object fields are not supported in structured outputs") {
		t.Fatalf("unexpected compile error: %v", err)
	}

	if !strings.Contains(err.Error(), "question_for_human") {
		t.Fatalf("expected property name in compile error: %v", err)
	}
}

func TestExecuteConnectorPack(t *testing.T) {
	blueprint := &Blueprint{
		Workflow: WorkflowSpec{
			ID:      "connector_pack",
			Version: "1",
			Inputs: map[string]string{
				"title":   "string",
				"content": "string",
			},
			Outputs: map[string]WorkflowOutputSpec{
				"out": {
					Schema: "Draft",
					From:   "instance.pack_draft.out",
				},
			},
		},
		Schemas: map[string]SchemaSpec{
			"Draft": {
				Type: "object",
				Properties: map[string]SchemaSpec{
					"title":   {Type: "string"},
					"content": {Type: "string"},
				},
			},
		},
		Nodes: map[string]NodeSpec{
			"draft_packer": {
				Kind:      "connector",
				Operation: "pack",
				Inputs: map[string]string{
					"title":   "string",
					"content": "string",
				},
				Outputs: map[string]string{
					"out": "Draft",
				},
			},
		},
		Flow: FlowSpec{
			Instances: map[string]InstanceSpec{
				"pack_draft": {
					Node: "draft_packer",
					Inputs: map[string]string{
						"title":   "workflow_input.title",
						"content": "workflow_input.content",
					},
				},
			},
		},
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"title":   "Bridge update",
		"content": "Widened sidewalks and new bike lanes.",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	draft, ok := output["out"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected pack output shape: %#v", output["out"])
	}

	if draft["title"] != "Bridge update" {
		t.Fatalf("unexpected title: %#v", draft["title"])
	}

	if draft["content"] != "Widened sidewalks and new bike lanes." {
		t.Fatalf("unexpected content: %#v", draft["content"])
	}
}

func TestExecuteForeachWorkflowTarget(t *testing.T) {
	tempDir := t.TempDir()

	childPath := filepath.Join(tempDir, "section-summary-single.yaml")
	err := os.WriteFile(childPath, []byte(`
workflow:
  id: section_summary_single
  version: "1"
  inputs:
    section_text: string
    tone: string
  outputs:
    out:
      schema: SectionSummary
      from: instance.summarize.out

schemas:
  SectionSummary:
    type: object
    properties:
      text:
        type: string

nodes:
  summarize:
    kind: inference
    inputs:
      section_text: string
      tone: string
    outputs:
      out: SectionSummary
    config:
      harness:
        id: codex_cli
        model: gpt-5.4-mini
      instruction: >
        Summarize one section in the requested tone.

flow:
  instances:
    summarize:
      node: summarize
      inputs:
        section_text: workflow_input.section_text
        tone: workflow_input.tone
`), 0644)
	if err != nil {
		t.Fatalf("write child blueprint: %v", err)
	}

	blueprint := &Blueprint{
		Workflow: WorkflowSpec{
			ID:      "foreach_workflow_target",
			Version: "1",
			Inputs: map[string]string{
				"section_text": "SectionTextList",
				"tone":         "string",
			},
			Outputs: map[string]WorkflowOutputSpec{
				"out": {
					Schema: "SectionSummaryList",
					From:   "instance.run_section_summary.out",
				},
			},
		},
		Schemas: map[string]SchemaSpec{
			"SectionTextList": {
				Type: "array",
				Items: &SchemaSpec{
					Type: "string",
				},
			},
			"SectionSummary": {
				Type: "object",
				Properties: map[string]SchemaSpec{
					"text": {Type: "string"},
				},
			},
			"SectionSummaryList": {
				Type: "array",
				Items: &SchemaSpec{
					SchemaRef: "SectionSummary",
				},
			},
		},
		Nodes: map[string]NodeSpec{
			"section_summary_pipeline": {
				Kind:       "workflow",
				WorkflowID: "section_summary_single",
			},
			"run_section_summary": {
				Kind:      "loop",
				Operation: "foreach",
				Executes:  "section_summary_pipeline",
				Over:      "section_text",
				Inputs: map[string]string{
					"section_text": "SectionTextList",
					"tone":         "string",
				},
				Outputs: map[string]string{
					"out": "SectionSummaryList",
				},
			},
		},
		Flow: FlowSpec{
			Instances: map[string]InstanceSpec{
				"run_section_summary": {
					Node: "run_section_summary",
					Inputs: map[string]string{
						"section_text": "workflow_input.section_text",
						"tone":         "workflow_input.tone",
					},
				},
			},
		},
		SourcePath: filepath.Join(tempDir, "parent.yaml"),
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	loopNode := snapshot.Nodes["run_section_summary"]
	if loopNode.LoopTarget == nil || loopNode.LoopTarget.Workflow == nil {
		t.Fatalf("expected foreach workflow target to be embedded")
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return sectionHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"section_text": []string{"Alpha section", "Beta section"},
		"tone":         "formal",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	summaries, ok := output["out"].([]any)
	if !ok {
		t.Fatalf("unexpected loop output shape: %#v", output["out"])
	}

	if len(summaries) != 2 {
		t.Fatalf("unexpected summary count: %d", len(summaries))
	}
}

func TestExecuteConditionalWorkflowTargets(t *testing.T) {
	tempDir := t.TempDir()

	summaryPath := filepath.Join(tempDir, "summary-branch.yaml")
	err := os.WriteFile(summaryPath, []byte(`
workflow:
  id: summary_branch
  version: "1"
  inputs:
    text: string
  outputs:
    out:
      schema: ReviewOutcome
      from: instance.summarize.out

schemas:
  ReviewOutcome:
    type: object
    properties:
      outcome:
        type: string
      text:
        type: string

nodes:
  summarize:
    kind: inference
    inputs:
      text: string
    outputs:
      out: ReviewOutcome
    config:
      harness:
        id: codex_cli
        model: gpt-5.4-mini
      instruction: >
        Summarize the text and set outcome to summary.

flow:
  instances:
    summarize:
      node: summarize
      inputs:
        text: workflow_input.text
`), 0644)
	if err != nil {
		t.Fatalf("write summary child blueprint: %v", err)
	}

	disagreementPath := filepath.Join(tempDir, "disagreement-branch.yaml")
	err = os.WriteFile(disagreementPath, []byte(`
workflow:
  id: disagreement_branch
  version: "1"
  inputs:
    text: string
  outputs:
    out:
      schema: ReviewOutcome
      from: instance.explain.out

schemas:
  ReviewOutcome:
    type: object
    properties:
      outcome:
        type: string
      text:
        type: string

nodes:
  explain:
    kind: inference
    inputs:
      text: string
    outputs:
      out: ReviewOutcome
    config:
      harness:
        id: codex_cli
        model: gpt-5.4-mini
      instruction: >
        Explain why you disagree with the text and set outcome to disagreement.

flow:
  instances:
    explain:
      node: explain
      inputs:
        text: workflow_input.text
`), 0644)
	if err != nil {
		t.Fatalf("write disagreement child blueprint: %v", err)
	}

	blueprint := &Blueprint{
		Workflow: WorkflowSpec{
			ID:      "conditional_workflow_targets",
			Version: "1",
			Inputs: map[string]string{
				"text": "string",
			},
			Outputs: map[string]WorkflowOutputSpec{
				"out": {
					Schema: "ReviewOutcome",
					From:   "instance.route_review.out",
				},
			},
		},
		Schemas: map[string]SchemaSpec{
			"ReviewOutcome": {
				Type: "object",
				Properties: map[string]SchemaSpec{
					"outcome": {Type: "string"},
					"text":    {Type: "string"},
				},
			},
		},
		Nodes: map[string]NodeSpec{
			"agreement_reviewer": {
				Kind: "inference",
				Inputs: map[string]string{
					"text": "string",
				},
				Outputs: map[string]string{
					"agrees": "boolean",
				},
				Config: InferenceNodeConfig{
					Harness: map[string]any{
						"id":    "codex_cli",
						"model": "gpt-5.4-mini",
					},
					Instruction: "Decide whether you agree with the text. Return only a boolean answer.",
				},
			},
			"summary_branch_worker": {
				Kind:       "workflow",
				WorkflowID: "summary_branch",
			},
			"disagreement_branch_worker": {
				Kind:       "workflow",
				WorkflowID: "disagreement_branch",
			},
			"route_review": {
				Kind:      "conditional",
				Operation: "if",
				RoutesOn:  "agrees",
				WhenTrue:  "summary_branch_worker",
				WhenFalse: "disagreement_branch_worker",
				Inputs: map[string]string{
					"text":   "string",
					"agrees": "boolean",
				},
				Outputs: map[string]string{
					"out": "ReviewOutcome",
				},
			},
		},
		Flow: FlowSpec{
			Instances: map[string]InstanceSpec{
				"review_agreement": {
					Node: "agreement_reviewer",
					Inputs: map[string]string{
						"text": "workflow_input.text",
					},
				},
				"route_review": {
					Node: "route_review",
					Inputs: map[string]string{
						"text":   "workflow_input.text",
						"agrees": "instance.review_agreement.agrees",
					},
				},
			},
		},
		SourcePath: filepath.Join(tempDir, "parent.yaml"),
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	conditionalNode := snapshot.Nodes["route_review"]
	if conditionalNode.TrueTarget == nil || conditionalNode.TrueTarget.Workflow == nil {
		t.Fatalf("expected true workflow branch target to be embedded")
	}

	if conditionalNode.FalseTarget == nil || conditionalNode.FalseTarget.Workflow == nil {
		t.Fatalf("expected false workflow branch target to be embedded")
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return conditionalHarness{}, nil
	}

	agreeingOutput, err := executor.Run(context.Background(), snapshot, map[string]any{
		"text": "This bridge project reduced traffic and improved pedestrian access.",
	})
	if err != nil {
		t.Fatalf("run agreeing workflow: %v", err)
	}

	agreeingReview, ok := agreeingOutput["out"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected agreeing output shape: %#v", agreeingOutput["out"])
	}

	if agreeingReview["outcome"] != "summary" {
		t.Fatalf("unexpected agreeing outcome: %#v", agreeingReview["outcome"])
	}

	disagreeingOutput, err := executor.Run(context.Background(), snapshot, map[string]any{
		"text": "This bridge project is expected to make cats pink.",
	})
	if err != nil {
		t.Fatalf("run disagreeing workflow: %v", err)
	}

	disagreeingReview, ok := disagreeingOutput["out"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected disagreeing output shape: %#v", disagreeingOutput["out"])
	}

	if disagreeingReview["outcome"] != "disagreement" {
		t.Fatalf("unexpected disagreeing outcome: %#v", disagreeingReview["outcome"])
	}
}

func TestCompileWorkflowCompositionCycle(t *testing.T) {
	tempDir := t.TempDir()

	firstPath := filepath.Join(tempDir, "first.yaml")
	err := os.WriteFile(firstPath, []byte(`
workflow:
  id: first
  version: "1"
  inputs: {}
  outputs:
    out:
      schema: string
      from: instance.second_worker.out

nodes:
  second_worker:
    kind: workflow
    workflow_id: second

flow:
  instances:
    second_worker:
      node: second_worker
      inputs: {}
`), 0644)
	if err != nil {
		t.Fatalf("write first blueprint: %v", err)
	}

	secondPath := filepath.Join(tempDir, "second.yaml")
	err = os.WriteFile(secondPath, []byte(`
workflow:
  id: second
  version: "1"
  inputs: {}
  outputs:
    out:
      schema: string
      from: instance.first_worker.out

nodes:
  first_worker:
    kind: workflow
    workflow_id: first

flow:
  instances:
    first_worker:
      node: first_worker
      inputs: {}
`), 0644)
	if err != nil {
		t.Fatalf("write second blueprint: %v", err)
	}

	blueprint, err := LoadBlueprintFile(firstPath)
	if err != nil {
		t.Fatalf("load first blueprint: %v", err)
	}

	_, err = Compile(blueprint)
	if err == nil {
		t.Fatalf("expected recursive workflow composition to fail")
	}

	if !strings.Contains(err.Error(), "workflow composition cycle detected") {
		t.Fatalf("unexpected cycle error: %v", err)
	}
}

func TestCompileCompositionArticleSummaryWithBrief(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/composed_article_summary.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "composed_article_summary" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	expectedNodes := []string{
		"make_brief",
		"summarize__critique_summary",
		"summarize__revise_summary",
		"summarize__summarize_article",
	}

	for _, nodeID := range expectedNodes {
		if _, found := snapshot.Nodes[nodeID]; !found {
			t.Fatalf("expected flattened node %q to exist", nodeID)
		}
	}

	makeBrief := snapshot.Nodes["make_brief"]
	if !makeBrief.WrapStructuredOutput {
		t.Fatalf("expected make_brief to wrap primitive structured output")
	}

	valueSchema, ok := makeBrief.StructuredOutputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected structured output schema: %#v", makeBrief.StructuredOutputSchema)
	}

	if _, found := valueSchema["value"]; !found {
		t.Fatalf("expected wrapped structured output to expose value field: %#v", makeBrief.StructuredOutputSchema)
	}
}

func TestExecuteCompositionArticleSummaryWithBrief(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/composed_article_summary.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return fakeHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"article_text": "A short article about a new bridge opening downtown.",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	brief, ok := output["executive_brief"].(string)
	if !ok {
		t.Fatalf("unexpected brief shape: %#v", output["executive_brief"])
	}

	if brief != "Executive brief" {
		t.Fatalf("unexpected executive brief: %#v", brief)
	}

	finalSummary, ok := output["final_summary"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected final summary shape: %#v", output["final_summary"])
	}

	if finalSummary["text"] != "Revised summary" {
		t.Fatalf("unexpected final summary text: %#v", finalSummary["text"])
	}
}

func TestCompileLoopForeachSectionSummary(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/section_summary_batch.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "section_summary_batch" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	loopNode, found := snapshot.Nodes["run_section_summary"]
	if !found {
		t.Fatalf("expected loop node to exist")
	}

	if loopNode.Kind != "loop" {
		t.Fatalf("expected run_section_summary to compile as loop")
	}

	if loopNode.Operation != "foreach" {
		t.Fatalf("unexpected loop operation: %q", loopNode.Operation)
	}

	if loopNode.Over != "section_text" {
		t.Fatalf("unexpected loop over input: %q", loopNode.Over)
	}

	if loopNode.LoopTarget == nil {
		t.Fatalf("expected loop target to be compiled")
	}
}

func TestExecuteLoopForeachSectionSummary(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/section_summary_batch.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return sectionHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"section_text": []string{"Alpha section", "Beta section"},
		"tone":         "formal",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	summaries, ok := output["out"].([]any)
	if !ok {
		t.Fatalf("unexpected loop output shape: %#v", output["out"])
	}

	if len(summaries) != 2 {
		t.Fatalf("unexpected summary count: %d", len(summaries))
	}

	firstSummary, ok := summaries[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first summary shape: %#v", summaries[0])
	}

	secondSummary, ok := summaries[1].(map[string]any)
	if !ok {
		t.Fatalf("unexpected second summary shape: %#v", summaries[1])
	}

	if firstSummary["text"] != "formal: Alpha section" {
		t.Fatalf("unexpected first summary text: %#v", firstSummary["text"])
	}

	if secondSummary["text"] != "formal: Beta section" {
		t.Fatalf("unexpected second summary text: %#v", secondSummary["text"])
	}
}

func TestCompileConditionalBooleanRoutingReview(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/conditional_boolean_routing_review.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "conditional_boolean_routing_review" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	conditionalNode, found := snapshot.Nodes["route_review"]
	if !found {
		t.Fatalf("expected conditional node to exist")
	}

	if conditionalNode.Kind != "conditional" {
		t.Fatalf("expected route_review to compile as conditional")
	}

	if conditionalNode.Operation != "if" {
		t.Fatalf("unexpected conditional operation: %q", conditionalNode.Operation)
	}

	if conditionalNode.RoutesOn != "agrees" {
		t.Fatalf("unexpected routes_on input: %q", conditionalNode.RoutesOn)
	}

	if conditionalNode.TrueTarget == nil || conditionalNode.FalseTarget == nil {
		t.Fatalf("expected conditional branch targets to be compiled")
	}
}

func TestExecuteConditionalBooleanRoutingReview(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/conditional_boolean_routing_review.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return conditionalHarness{}, nil
	}

	agreeingOutput, err := executor.Run(context.Background(), snapshot, map[string]any{
		"text": "This bridge project reduced traffic and improved pedestrian access.",
	})
	if err != nil {
		t.Fatalf("run agreeing workflow: %v", err)
	}

	agreeingReview, ok := agreeingOutput["out"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected agreeing output shape: %#v", agreeingOutput["out"])
	}

	if agreeingReview["outcome"] != "summary" {
		t.Fatalf("unexpected agreeing outcome: %#v", agreeingReview["outcome"])
	}

	disagreeingOutput, err := executor.Run(context.Background(), snapshot, map[string]any{
		"text": "This bridge project is expected to make cats pink.",
	})
	if err != nil {
		t.Fatalf("run disagreeing workflow: %v", err)
	}

	disagreeingReview, ok := disagreeingOutput["out"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected disagreeing output shape: %#v", disagreeingOutput["out"])
	}

	if disagreeingReview["outcome"] != "disagreement" {
		t.Fatalf("unexpected disagreeing outcome: %#v", disagreeingReview["outcome"])
	}
}

func TestCompileLoopAndConnectorParallelCodeReview(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/parallel_code_review.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "parallel_code_review" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	aggregator, found := snapshot.Nodes["aggregate_validated_issues"]
	if !found {
		t.Fatalf("expected concat connector node to exist")
	}

	if aggregator.Kind != "connector" {
		t.Fatalf("expected aggregate_validated_issues to compile as connector")
	}

	if aggregator.Operation != "concat" {
		t.Fatalf("unexpected connector operation: %q", aggregator.Operation)
	}
}

func TestExecuteLoopAndConnectorParallelCodeReview(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/parallel_code_review.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return reviewHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"compare_branch": "master",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	issues, ok := output["issues"].([]any)
	if !ok {
		t.Fatalf("unexpected issues shape: %#v", output["issues"])
	}

	if len(issues) != 3 {
		t.Fatalf("unexpected validated issue count: %d", len(issues))
	}

	firstIssue, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first issue shape: %#v", issues[0])
	}

	if firstIssue["is_valid"] != true {
		t.Fatalf("unexpected validation result: %#v", firstIssue["is_valid"])
	}
}

func TestCompileLoopWhileBinaryConsensus(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/loop_while_binary_consensus.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "loop_while_binary_consensus" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	loopNode, found := snapshot.Nodes["collect_consensus"]
	if !found {
		t.Fatalf("expected while loop node to exist")
	}

	if loopNode.Kind != "loop" {
		t.Fatalf("expected collect_consensus to compile as loop")
	}

	if loopNode.Operation != "while" {
		t.Fatalf("unexpected loop operation: %q", loopNode.Operation)
	}

	if loopNode.Updates != "vote_state" {
		t.Fatalf("unexpected updates field: %q", loopNode.Updates)
	}

	if loopNode.BreaksOn != "should_stop" {
		t.Fatalf("unexpected breaks_on field: %q", loopNode.BreaksOn)
	}

	if loopNode.MaxIterations != 10 {
		t.Fatalf("unexpected max_iterations: %d", loopNode.MaxIterations)
	}

	if loopNode.WhileTarget == nil {
		t.Fatalf("expected while target to be compiled")
	}
}

func TestExecuteLoopWhileBinaryConsensus(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/loop_while_binary_consensus.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return whileHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"question": "Should we deploy the bridge update today?",
		"vote_state": map[string]any{
			"votes":                 []any{},
			"yes_count":             0,
			"no_count":              0,
			"agreement_ratio":       0.0,
			"minimum_votes_reached": false,
			"consensus_reached":     false,
		},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	voteState, ok := output["vote_state"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected vote_state shape: %#v", output["vote_state"])
	}

	if voteState["consensus_reached"] != true {
		t.Fatalf("unexpected consensus_reached: %#v", voteState["consensus_reached"])
	}

	votes, ok := voteState["votes"].([]any)
	if !ok {
		t.Fatalf("unexpected votes shape: %#v", voteState["votes"])
	}

	if len(votes) != 5 {
		t.Fatalf("unexpected vote count: %d", len(votes))
	}
}

func TestExecuteWhileLoopStopsGracefullyAtMaxIterations(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/loop_while_binary_consensus.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return neverConsensusHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"question": "Should we deploy the bridge update today?",
		"vote_state": map[string]any{
			"votes":                 []any{},
			"yes_count":             0,
			"no_count":              0,
			"agreement_ratio":       0.0,
			"minimum_votes_reached": false,
			"consensus_reached":     false,
		},
	})
	if err != nil {
		t.Fatalf("expected graceful stop, got error: %v", err)
	}

	voteState, ok := output["vote_state"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected vote_state shape: %#v", output["vote_state"])
	}

	if voteState["consensus_reached"] != false {
		t.Fatalf("expected consensus_reached false, got: %#v", voteState["consensus_reached"])
	}

	votes, ok := voteState["votes"].([]any)
	if !ok {
		t.Fatalf("unexpected votes shape: %#v", voteState["votes"])
	}

	if len(votes) != 10 {
		t.Fatalf("expected 10 votes carried from the final iteration, got: %d", len(votes))
	}
}

func TestCompilePipelineParallelReviewFixCycle(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/review_fix_cycle.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "review_fix_cycle" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	triageNode, found := snapshot.Nodes["triage_review_results"]
	if !found {
		t.Fatalf("expected triage_review_results node to exist")
	}

	if len(triageNode.Outputs) != 2 {
		t.Fatalf("unexpected triage output count: %d", len(triageNode.Outputs))
	}

	if _, found := triageNode.Outputs["review_state"]; !found {
		t.Fatalf("expected triage_review_results.review_state output")
	}

	if _, found := triageNode.Outputs["should_stop"]; !found {
		t.Fatalf("expected triage_review_results.should_stop output")
	}
}

func TestExecutePipelineParallelReviewFixCycle(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/review_fix_cycle.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return repairHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"coding_task": "Implement the bridge task",
		"review_state": map[string]any{
			"implementation_summary": "Initial implementation",
			"actionable_issues":      []any{},
			"pending_questions":      []any{},
			"iteration":              0,
		},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	reviewState, ok := output["review_state"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected review_state shape: %#v", output["review_state"])
	}

	if output["should_stop"] != false {
		t.Fatalf("unexpected should_stop: %#v", output["should_stop"])
	}

	actionableIssues, err := toSlice(reviewState["actionable_issues"])
	if err != nil {
		t.Fatalf("unexpected actionable_issues: %v", err)
	}

	if len(actionableIssues) != 1 {
		t.Fatalf("unexpected actionable issue count: %d", len(actionableIssues))
	}

	pendingQuestions, err := toSlice(reviewState["pending_questions"])
	if err != nil {
		t.Fatalf("unexpected pending_questions: %v", err)
	}

	if len(pendingQuestions) != 1 {
		t.Fatalf("unexpected pending question count: %d", len(pendingQuestions))
	}
}

func TestCompileCompositionLoopIterativeCodeRepair(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/iterative_code_review_repair.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	if snapshot.WorkflowID != "iterative_code_review_repair" {
		t.Fatalf("unexpected workflow id: %q", snapshot.WorkflowID)
	}

	loopNode, found := snapshot.Nodes["review_fix_loop"]
	if !found {
		t.Fatalf("expected review_fix_loop node to exist")
	}

	if loopNode.WhileTarget == nil {
		t.Fatalf("expected review_fix_loop while target to be compiled")
	}

	if loopNode.WhileTarget.Workflow == nil {
		t.Fatalf("expected review_fix_loop while target to embed a workflow snapshot")
	}

	unpackNode, found := snapshot.Nodes["final_state_unpacker"]
	if !found {
		t.Fatalf("expected final_state_unpacker node to exist")
	}

	if unpackNode.Operation != "unpack" {
		t.Fatalf("unexpected unpack connector operation: %q", unpackNode.Operation)
	}
}

func TestExecuteCompositionLoopIterativeCodeRepair(t *testing.T) {
	blueprint, err := LoadBlueprintFile("../examples/iterative_code_review_repair.yaml")
	if err != nil {
		t.Fatalf("load blueprint: %v", err)
	}

	snapshot, err := Compile(blueprint)
	if err != nil {
		t.Fatalf("compile workflow: %v", err)
	}

	executor := NewExecutor("")
	executor.NewHarness = func(kind agent.Harness) (harnesses.Harness, error) {
		return repairHarness{}, nil
	}

	output, err := executor.Run(context.Background(), snapshot, map[string]any{
		"coding_task": "Implement the bridge task",
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}

	implementationSummary, ok := output["implementation_summary"].(string)
	if !ok {
		t.Fatalf("unexpected implementation_summary shape: %#v", output["implementation_summary"])
	}

	if implementationSummary != "Implementation iteration 1" {
		t.Fatalf("unexpected implementation summary: %#v", implementationSummary)
	}

	pendingQuestions, err := toSlice(output["pending_questions"])
	if err != nil {
		t.Fatalf("unexpected pending_questions: %v", err)
	}

	if len(pendingQuestions) != 1 {
		t.Fatalf("unexpected pending question count: %d", len(pendingQuestions))
	}
}

func TestSupportedExampleWorkflowsExecute(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		input      map[string]any
		newHarness func(kind agent.Harness) (harnesses.Harness, error)
		assertion  func(t *testing.T, output map[string]any)
	}{
		{
			name:  "pipeline summary critique revise",
			path:  "../examples/article_summary.yaml",
			input: map[string]any{"article_text": "A short article about a new bridge opening downtown."},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return fakeHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				finalSummary, ok := output["final_summary"].(map[string]any)
				if !ok {
					t.Fatalf("unexpected final summary shape: %#v", output["final_summary"])
				}

				if finalSummary["text"] != "Revised summary" {
					t.Fatalf("unexpected final summary text: %#v", finalSummary["text"])
				}
			},
		},
		{
			name:  "connector collect binary votes",
			path:  "../examples/binary_vote_round.yaml",
			input: map[string]any{"question": "Should we deploy today?"},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return voteHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				votes, ok := output["votes"].([]any)
				if !ok {
					t.Fatalf("unexpected votes shape: %#v", output["votes"])
				}

				if len(votes) != 7 {
					t.Fatalf("unexpected vote count: %d", len(votes))
				}
			},
		},
		{
			name:  "composition article summary with brief",
			path:  "../examples/composed_article_summary.yaml",
			input: map[string]any{"article_text": "A short article about a new bridge opening downtown."},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return fakeHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				brief, ok := output["executive_brief"].(string)
				if !ok {
					t.Fatalf("unexpected brief shape: %#v", output["executive_brief"])
				}

				if brief != "Executive brief" {
					t.Fatalf("unexpected executive brief: %#v", brief)
				}
			},
		},
		{
			name:  "loop foreach section summary",
			path:  "../examples/section_summary_batch.yaml",
			input: map[string]any{"section_text": []string{"Alpha section", "Beta section"}, "tone": "formal"},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return sectionHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				summaries, ok := output["out"].([]any)
				if !ok {
					t.Fatalf("unexpected loop output shape: %#v", output["out"])
				}

				if len(summaries) != 2 {
					t.Fatalf("unexpected summary count: %d", len(summaries))
				}
			},
		},
		{
			name:  "conditional boolean routing review",
			path:  "../examples/conditional_boolean_routing_review.yaml",
			input: map[string]any{"text": "This bridge project reduced traffic and improved pedestrian access."},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return conditionalHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				review, ok := output["out"].(map[string]any)
				if !ok {
					t.Fatalf("unexpected conditional output shape: %#v", output["out"])
				}

				if review["outcome"] != "summary" {
					t.Fatalf("unexpected conditional outcome: %#v", review["outcome"])
				}
			},
		},
		{
			name:  "loop and connector parallel code review",
			path:  "../examples/parallel_code_review.yaml",
			input: map[string]any{"compare_branch": "master"},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return reviewHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				issues, ok := output["issues"].([]any)
				if !ok {
					t.Fatalf("unexpected validated issues shape: %#v", output["issues"])
				}

				if len(issues) != 3 {
					t.Fatalf("unexpected validated issue count: %d", len(issues))
				}
			},
		},
		{
			name: "loop while binary consensus",
			path: "../examples/loop_while_binary_consensus.yaml",
			input: map[string]any{
				"question": "Should we deploy the bridge update today?",
				"vote_state": map[string]any{
					"votes":                 []any{},
					"yes_count":             0,
					"no_count":              0,
					"agreement_ratio":       0.0,
					"minimum_votes_reached": false,
					"consensus_reached":     false,
				},
			},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return whileHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				voteState, ok := output["vote_state"].(map[string]any)
				if !ok {
					t.Fatalf("unexpected vote_state shape: %#v", output["vote_state"])
				}

				if voteState["consensus_reached"] != true {
					t.Fatalf("unexpected consensus_reached: %#v", voteState["consensus_reached"])
				}
			},
		},
		{
			name: "pipeline parallel review fix cycle",
			path: "../examples/review_fix_cycle.yaml",
			input: map[string]any{
				"coding_task": "Implement the bridge task",
				"review_state": map[string]any{
					"implementation_summary": "Initial implementation",
					"actionable_issues":      []any{},
					"pending_questions":      []any{},
					"iteration":              0,
				},
			},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return repairHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				if output["should_stop"] != false {
					t.Fatalf("unexpected should_stop: %#v", output["should_stop"])
				}
			},
		},
		{
			name:  "composition loop iterative code repair",
			path:  "../examples/iterative_code_review_repair.yaml",
			input: map[string]any{"coding_task": "Implement the bridge task"},
			newHarness: func(kind agent.Harness) (harnesses.Harness, error) {
				return repairHarness{}, nil
			},
			assertion: func(t *testing.T, output map[string]any) {
				t.Helper()

				implementationSummary, ok := output["implementation_summary"].(string)
				if !ok {
					t.Fatalf("unexpected implementation_summary shape: %#v", output["implementation_summary"])
				}

				if implementationSummary != "Implementation iteration 1" {
					t.Fatalf("unexpected implementation summary: %#v", implementationSummary)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			blueprint, err := LoadBlueprintFile(testCase.path)
			if err != nil {
				t.Fatalf("load blueprint: %v", err)
			}

			snapshot, err := Compile(blueprint)
			if err != nil {
				t.Fatalf("compile workflow: %v", err)
			}

			executor := NewExecutor("")
			executor.NewHarness = testCase.newHarness

			output, err := executor.Run(context.Background(), snapshot, testCase.input)
			if err != nil {
				t.Fatalf("run workflow: %v", err)
			}

			testCase.assertion(t, output)
		})
	}
}

func TestExampleWorkflowCoverage(t *testing.T) {
	entries, err := os.ReadDir("../examples")
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}

	supportedCompiles := map[string]bool{
		"article_summary.yaml":                    true,
		"binary_vote_round.yaml":                  true,
		"blueprint-plan-cycle.yaml":               true,
		"composed_article_summary.yaml":           true,
		"section_summary_batch.yaml":              true,
		"conditional_boolean_routing_review.yaml": true,
		"parallel_code_review.yaml":               true,
		"loop_while_binary_consensus.yaml":        true,
		"plan-new-blueprint.yaml":                 true,
		"review_fix_cycle.yaml":                   true,
		"iterative_code_review_repair.yaml":       true,
	}

	seen := map[string]bool{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		seen[entry.Name()] = true

		path := filepath.Join("../examples", entry.Name())
		blueprint, err := LoadBlueprintFile(path)
		if err != nil {
			t.Fatalf("load %s: %v", entry.Name(), err)
		}

		_, err = Compile(blueprint)
		if supportedCompiles[entry.Name()] {
			if err != nil {
				t.Fatalf("compile %s: %v", entry.Name(), err)
			}
			continue
		}

		if err == nil {
			t.Fatalf("expected %s to be unsupported by the current workflow runtime", entry.Name())
		}
	}

	if !seen["article_summary.yaml"] || !seen["binary_vote_round.yaml"] {
		t.Fatalf("expected core example workflows to be present, got %#v", seen)
	}
}

type fakeHarness struct{}

func (fakeHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

type voteHarness struct{}

func (voteHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (voteHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	switch conversation.AgentName {
	case "voter_a":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"yes","rationale":"Clear benefit."}`}, nil
	case "voter_b":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"yes","rationale":"Risk is low."}`}, nil
	case "voter_c":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"no","rationale":"Need more testing."}`}, nil
	case "voter_d":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"yes","rationale":"Operationally safe."}`}, nil
	case "voter_e":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"no","rationale":"Monitoring is incomplete."}`}, nil
	case "voter_f":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"yes","rationale":"Rollback is prepared."}`}, nil
	case "voter_g":
		return &harnesses.RunResult{LastAssistantMessage: `{"answer":"yes","rationale":"The team is ready."}`}, nil
	default:
		return nil, fmt.Errorf("unexpected voter agent: %q", conversation.AgentName)
	}
}

type sectionHarness struct{}

func (sectionHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (sectionHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	sectionText, ok := input["section_text"].(string)
	if !ok {
		return nil, fmt.Errorf("missing section_text in prompt input")
	}

	tone, ok := input["tone"].(string)
	if !ok {
		return nil, fmt.Errorf("missing tone in prompt input")
	}

	return &harnesses.RunResult{
		LastAssistantMessage: fmt.Sprintf(`{"text":%q}`, tone+": "+sectionText),
	}, nil
}

type conditionalHarness struct{}

func (conditionalHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (conditionalHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	text, ok := input["text"].(string)
	if !ok {
		return nil, fmt.Errorf("missing text in prompt input")
	}

	switch {
	case strings.Contains(conversation.Instructions, "Decide whether you agree with the text. Return only a boolean answer."):
		agrees := !strings.Contains(strings.ToLower(text), "cats pink")
		if agrees {
			return &harnesses.RunResult{LastAssistantMessage: `{"value":true}`}, nil
		}

		return &harnesses.RunResult{LastAssistantMessage: `{"value":false}`}, nil
	case strings.Contains(conversation.Instructions, "Summarize the text and set outcome to summary."):
		return &harnesses.RunResult{
			LastAssistantMessage: fmt.Sprintf(`{"outcome":"summary","text":%q}`, text),
		}, nil
	case strings.Contains(conversation.Instructions, "Explain why you disagree with the text and set outcome to disagreement."):
		return &harnesses.RunResult{
			LastAssistantMessage: `{"outcome":"disagreement","text":"The claim is not credible."}`,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected instruction: %q", conversation.Instructions)
	}
}

type reviewHarness struct{}

func (reviewHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (reviewHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.Contains(conversation.Instructions, "Review the git diff against the input branch and return a list of"):
		branch, ok := input["branch"].(string)
		if !ok {
			return nil, fmt.Errorf("missing branch in prompt input")
		}

		switch conversation.AgentName {
		case "reviewer_a":
			return &harnesses.RunResult{
				LastAssistantMessage: fmt.Sprintf(`{"value":[{"path":"workflow/workflow.go","line":42,"title":"A issue on %s","description":"Reviewer A found an issue."}]}`, branch),
			}, nil
		case "reviewer_b":
			return &harnesses.RunResult{
				LastAssistantMessage: `{"value":[{"path":"workflow/workflow.go","line":84,"title":"B issue","description":"Reviewer B found an issue."}]}`,
			}, nil
		case "reviewer_c":
			return &harnesses.RunResult{
				LastAssistantMessage: `{"value":[{"path":"workflow/workflow_test.go","line":120,"title":"C issue","description":"Reviewer C found an issue."}]}`,
			}, nil
		default:
			return nil, fmt.Errorf("unexpected reviewer agent: %q", conversation.AgentName)
		}
	case strings.Contains(conversation.Instructions, "Validate whether the issue is a real problem in the reviewed diff"):
		issue, ok := input["issue"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing issue in prompt input")
		}

		returnedIssue, err := json.Marshal(issue)
		if err != nil {
			return nil, err
		}

		return &harnesses.RunResult{
			LastAssistantMessage: fmt.Sprintf(`{"issue":%s,"is_valid":true,"reason":"confirmed"}`, returnedIssue),
		}, nil
	default:
		return nil, fmt.Errorf("unexpected instruction: %q", conversation.Instructions)
	}
}

type whileHarness struct{}

func (whileHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (whileHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	voteState, ok := input["vote_state"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing vote_state in prompt input")
	}

	votes, err := toMapSlice(voteState["votes"])
	if err != nil {
		return nil, err
	}

	yesCount, err := intFromAny(voteState["yes_count"])
	if err != nil {
		return nil, err
	}

	noCount, err := intFromAny(voteState["no_count"])
	if err != nil {
		return nil, err
	}

	newVote := map[string]any{
		"answer":    "yes",
		"rationale": fmt.Sprintf("Vote %d supports deployment.", len(votes)+1),
	}

	votes = append(votes, newVote)
	yesCount++

	totalVotes := len(votes)
	agreementRatio := float64(yesCount)
	if totalVotes > 0 {
		agreementRatio = agreementRatio / float64(totalVotes)
	}

	minimumVotesReached := totalVotes >= 5
	consensusReached := minimumVotesReached && agreementRatio >= 0.66

	nextState := map[string]any{
		"votes":                 votes,
		"yes_count":             yesCount,
		"no_count":              noCount,
		"agreement_ratio":       agreementRatio,
		"minimum_votes_reached": minimumVotesReached,
		"consensus_reached":     consensusReached,
	}

	payload, err := json.Marshal(map[string]any{
		"vote_state":  nextState,
		"should_stop": consensusReached,
	})
	if err != nil {
		return nil, err
	}

	return &harnesses.RunResult{LastAssistantMessage: string(payload)}, nil
}

type neverConsensusHarness struct{}

func (neverConsensusHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (neverConsensusHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	voteState, ok := input["vote_state"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing vote_state in prompt input")
	}

	votes, err := toMapSlice(voteState["votes"])
	if err != nil {
		return nil, err
	}

	votes = append(votes, map[string]any{
		"answer":    "yes",
		"rationale": fmt.Sprintf("Vote %d never reaches consensus.", len(votes)+1),
	})

	nextState := map[string]any{
		"votes":                 votes,
		"yes_count":             len(votes),
		"no_count":              0,
		"agreement_ratio":       1.0,
		"minimum_votes_reached": false,
		"consensus_reached":     false,
	}

	payload, err := json.Marshal(map[string]any{
		"vote_state":  nextState,
		"should_stop": false,
	})
	if err != nil {
		return nil, err
	}

	return &harnesses.RunResult{LastAssistantMessage: string(payload)}, nil
}

type repairHarness struct{}

func (repairHarness) Validate(ctx context.Context, model string, config json.RawMessage) error {
	return nil
}

func (repairHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	input, err := promptInputs(prompt)
	if err != nil {
		return nil, err
	}

	switch conversation.AgentName {
	case "initial_implementation":
		payload, err := json.Marshal(map[string]any{
			"implementation_summary": "Initial implementation",
			"actionable_issues":      []any{},
			"pending_questions":      []any{},
			"iteration":              0,
		})
		if err != nil {
			return nil, err
		}

		return &harnesses.RunResult{LastAssistantMessage: string(payload)}, nil
	case "revise_code":
		reviewState, ok := input["review_state"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing review_state in prompt input")
		}

		iteration, err := intFromAny(reviewState["iteration"])
		if err != nil {
			return nil, err
		}

		return &harnesses.RunResult{
			LastAssistantMessage: fmt.Sprintf(`{"value":"Implementation iteration %d"}`, iteration),
		}, nil
	case "review_a":
		implementationSummary, ok := input["implementation_summary"].(string)
		if !ok {
			return nil, fmt.Errorf("missing implementation_summary in prompt input")
		}

		if strings.Contains(implementationSummary, "iteration 0") {
			return &harnesses.RunResult{
				LastAssistantMessage: `{"value":[{"title":"Missing test","description":"Add regression coverage.","severity":"medium","requires_human_input":false}]}`,
			}, nil
		}

		return &harnesses.RunResult{LastAssistantMessage: `{"value":[]}`}, nil
	case "review_b":
		implementationSummary, ok := input["implementation_summary"].(string)
		if !ok {
			return nil, fmt.Errorf("missing implementation_summary in prompt input")
		}

		if strings.Contains(implementationSummary, "iteration 0") {
			return &harnesses.RunResult{
				LastAssistantMessage: `{"value":[{"title":"Need product decision","description":"This depends on product intent.","severity":"low","requires_human_input":true,"question_for_human":"Should the bridge update remain behind a flag?"}]}`,
			}, nil
		}

		return &harnesses.RunResult{LastAssistantMessage: `{"value":[]}`}, nil
	case "review_c":
		return &harnesses.RunResult{LastAssistantMessage: `{"value":[]}`}, nil
	case "issue_validation_worker__issue_validator":
		issue, ok := input["issue"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing issue in prompt input")
		}

		encodedIssue, err := json.Marshal(issue)
		if err != nil {
			return nil, err
		}

		return &harnesses.RunResult{
			LastAssistantMessage: fmt.Sprintf(`{"issue":%s,"is_valid":true,"validation_reason":"confirmed"}`, encodedIssue),
		}, nil
	case "triage_review_results":
		reviewState, ok := input["review_state"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("missing review_state in prompt input")
		}

		implementationSummary, ok := input["implementation_summary"].(string)
		if !ok {
			return nil, fmt.Errorf("missing implementation_summary in prompt input")
		}

		validatedIssues, err := toMapSlice(input["validated_issues"])
		if err != nil {
			return nil, err
		}

		existingQuestions, err := toMapSlice(reviewState["pending_questions"])
		if err != nil {
			return nil, err
		}

		nextQuestions := append([]map[string]any{}, existingQuestions...)
		actionableIssues := make([]map[string]any, 0)

		for _, validatedIssue := range validatedIssues {
			isValid, ok := validatedIssue["is_valid"].(bool)
			if !ok || !isValid {
				continue
			}

			issue, ok := validatedIssue["issue"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("validated issue is missing issue payload")
			}

			requiresHumanInput, ok := issue["requires_human_input"].(bool)
			if !ok {
				return nil, fmt.Errorf("issue is missing requires_human_input")
			}

			if requiresHumanInput {
				question, ok := issue["question_for_human"].(string)
				if !ok {
					return nil, fmt.Errorf("issue is missing question_for_human")
				}

				title, ok := issue["title"].(string)
				if !ok {
					return nil, fmt.Errorf("issue is missing title")
				}

				nextQuestions = append(nextQuestions, map[string]any{
					"question":    question,
					"issue_title": title,
					"rationale":   "Need developer input before making this change.",
				})

				continue
			}

			actionableIssues = append(actionableIssues, issue)
		}

		iteration, err := intFromAny(reviewState["iteration"])
		if err != nil {
			return nil, err
		}

		nextState := map[string]any{
			"implementation_summary": implementationSummary,
			"actionable_issues":      actionableIssues,
			"pending_questions":      nextQuestions,
			"iteration":              iteration + 1,
		}

		payload, err := json.Marshal(map[string]any{
			"review_state": nextState,
			"should_stop":  len(actionableIssues) == 0,
		})
		if err != nil {
			return nil, err
		}

		return &harnesses.RunResult{LastAssistantMessage: string(payload)}, nil
	default:
		return nil, fmt.Errorf("unexpected repair agent: %q", conversation.AgentName)
	}
}

func (fakeHarness) Run(ctx context.Context, conversation *agent.Conversation, prompt string) (*harnesses.RunResult, error) {
	switch {
	case strings.Contains(conversation.Instructions, "Summarize the article into a concise draft"):
		return &harnesses.RunResult{
			LastAssistantMessage: `{"text":"Draft summary"}`,
		}, nil
	case strings.Contains(conversation.Instructions, "Review the draft summary against the source article"):
		return &harnesses.RunResult{
			LastAssistantMessage: `{"is_accurate":false,"unsupported_claims":["One detail"],"missing_key_points":["Opening date"],"clarity_issues":["Too vague"],"revision_instructions":["Mention the opening date"]}`,
		}, nil
	case strings.Contains(conversation.Instructions, "Rewrite the summary using the critique"):
		return &harnesses.RunResult{
			LastAssistantMessage: `{"text":"Revised summary"}`,
		}, nil
	case strings.Contains(conversation.Instructions, "Write a short executive brief from the final summary"):
		return &harnesses.RunResult{
			LastAssistantMessage: `{"value":"Executive brief"}`,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected instruction: %q", conversation.Instructions)
	}
}

func promptInputs(prompt string) (map[string]any, error) {
	const prefix = "Inputs:\n"
	const suffix = "\n\nReturn exactly one JSON value that matches this JSON Schema:\n"

	start := strings.Index(prompt, prefix)
	if start == -1 {
		return nil, fmt.Errorf("prompt inputs prefix not found")
	}

	start += len(prefix)
	end := strings.Index(prompt[start:], suffix)
	if end == -1 {
		return nil, fmt.Errorf("prompt inputs suffix not found")
	}

	payload := prompt[start : start+end]

	var input map[string]any
	err := json.Unmarshal([]byte(payload), &input)
	if err != nil {
		return nil, err
	}

	return input, nil
}

func toMapSlice(value any) ([]map[string]any, error) {
	items, err := toSlice(value)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		typed, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("slice item is not an object")
		}

		result = append(result, typed)
	}

	return result, nil
}

func toSlice(value any) ([]any, error) {
	typed, ok := value.([]any)
	if ok {
		return typed, nil
	}

	return nil, fmt.Errorf("value is not a slice")
}

func intFromAny(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("value is not a number")
	}
}
