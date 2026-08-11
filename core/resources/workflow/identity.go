package workflow

import (
	"context"

	"github.com/vanclief/agent-composer/models/execution"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
)

// BackfillWorkflowIDs links past runs to each installed workflow's
// permanent id. Idempotent — runs at startup so history recorded
// before a workflow was imported (e.g. from --file runs) still keys
// on it.
func (api *API) BackfillWorkflowIDs(ctx context.Context) error {
	records, err := workflowmodels.ListWorkflows(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	for _, record := range records {
		if record.Spec == "" {
			continue
		}

		_, err = api.db.NewUpdate().
			Model((*execution.WorkflowExecution)(nil)).
			Set("workflow_id = ?", record.ID).
			Where("workflow_slug = ? AND workflow_id IS NULL", record.Slug).
			Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}
