package workflow

import (
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"
)

// EnsureInstalledWorkflowUUIDs stamps a uuid into every installed
// blueprint that lacks one and returns slug → uuid for all installed
// workflows. Idempotent — files that already carry a uuid are only
// read.
func EnsureInstalledWorkflowUUIDs() (map[string]string, error) {
	summaries, err := ListBlueprints()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	identities := make(map[string]string, len(summaries))
	for _, summary := range summaries {
		entry, err := loadRegistryBlueprintEntryByWorkflowID(summary.ID)
		if err != nil {
			continue
		}

		existing := strings.TrimSpace(entry.Blueprint.Workflow.UUID)
		if existing != "" {
			identities[summary.ID] = existing
			continue
		}

		raw, err := os.ReadFile(entry.Path)
		if err != nil {
			continue
		}
		minted := uuid.NewString()
		stamped, err := stampWorkflowHeader(raw, "", minted)
		if err != nil {
			continue
		}
		err = writeFileAtomically(entry.Path, stamped)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		identities[summary.ID] = minted
	}

	return identities, nil
}
