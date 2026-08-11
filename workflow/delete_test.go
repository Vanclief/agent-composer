package workflow

import (
	"testing"

	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
)

func TestDeleteWorkflowRemovesSpecAndDraft(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	// The embedder is embedded by nobody, so it can go. Give it a
	// draft to confirm the draft goes with it.
	err := registry.WriteDraft(ctx, "embedder", []byte(renameEmbedderSpec))
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Delete(ctx, "embedder")
	if err != nil {
		t.Fatal(err)
	}

	_, err = registry.Load(ctx, "embedder")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("the workflow should be gone, got: %v", err)
	}
	draft, err := registry.ReadDraft(ctx, "embedder")
	if err != nil {
		t.Fatal(err)
	}
	if draft != "" {
		t.Fatal("draft should be gone")
	}
}

func TestDeleteWorkflowRefusesWhenEmbedded(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	err := registry.Delete(ctx, "rename_target")
	if err == nil {
		t.Fatal("expected refusal — rename_target is embedded by embedder")
	}

	_, err = registry.Load(ctx, "rename_target")
	if err != nil {
		t.Fatal("the workflow must remain after a refused delete")
	}
}

func TestDeleteWorkflowKeepsVersionHistory(t *testing.T) {
	registry, ctx := importRenameFixture(t)

	record, err := workflowmodels.GetWorkflowBySlug(ctx, registry.db, "embedder")
	if err != nil {
		t.Fatal(err)
	}

	err = registry.Delete(ctx, "embedder")
	if err != nil {
		t.Fatal(err)
	}

	versions, err := workflowmodels.ListWorkflowVersions(ctx, registry.db, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("the version history must survive a delete")
	}
}
