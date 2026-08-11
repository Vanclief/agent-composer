package workflow

import (
	"context"

	"github.com/vanclief/agent-composer/models/execution"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

// BackfillWorkflowUUIDs gives every installed workflow a permanent
// uuid and links past runs to it. Idempotent — runs at startup so
// history recorded before uuids existed still keys on them.
func (api *API) BackfillWorkflowUUIDs(ctx context.Context) error {
	identities, err := workflowruntime.EnsureInstalledWorkflowUUIDs()
	if err != nil {
		return ez.Wrap(err)
	}

	for slug, workflowUUID := range identities {
		_, err = api.db.NewUpdate().
			Model((*execution.WorkflowExecution)(nil)).
			Set("workflow_uuid = ?", workflowUUID).
			Where("workflow_id = ? AND workflow_uuid IS NULL", slug).
			Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
