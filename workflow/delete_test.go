package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteWorkflowRemovesFileAndDraft(t *testing.T) {
	home := writeRenameFixture(t)

	// The embedder is embedded by nobody, so it can go. Give it a
	// draft to confirm the draft goes with it.
	if err := WriteDraft("embedder", []byte(renameEmbedderBlueprint)); err != nil {
		t.Fatal(err)
	}

	if err := DeleteWorkflow("embedder"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(home, "workflows", "embedder.yaml")); !os.IsNotExist(err) {
		t.Fatal("registry file should be gone")
	}
	draft, err := ReadDraft("embedder")
	if err != nil {
		t.Fatal(err)
	}
	if draft != "" {
		t.Fatal("draft should be gone")
	}
}

func TestDeleteWorkflowRefusesWhenEmbedded(t *testing.T) {
	home := writeRenameFixture(t)

	err := DeleteWorkflow("rename_target")
	if err == nil {
		t.Fatal("expected refusal — rename_target is embedded by embedder")
	}

	if _, err := os.Stat(filepath.Join(home, "workflows", "rename_target.yaml")); err != nil {
		t.Fatal("registry file must remain after a refused delete")
	}
}

func TestDeleteWorkflowKeepsVersionsArchive(t *testing.T) {
	home := writeRenameFixture(t)

	versionsDir := filepath.Join(home, "versions", "embedder")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	err := os.WriteFile(
		filepath.Join(versionsDir, "v1.yaml"),
		[]byte(renameEmbedderBlueprint),
		0644,
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := DeleteWorkflow("embedder"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(versionsDir, "v1.yaml")); err != nil {
		t.Fatal("the versions archive must survive a delete")
	}
}
