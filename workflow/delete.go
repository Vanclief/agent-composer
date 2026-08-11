package workflow

import (
	"context"
	"strings"

	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
)

// listEmbedders returns installed workflows that embed targetID via a
// workflow node.
func (r *Registry) listEmbedders(ctx context.Context, targetID string) ([]string, error) {
	records, err := workflowmodels.ListWorkflows(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	embedders := []string{}
	for _, record := range records {
		if record.Slug == targetID || record.Spec == "" {
			continue
		}

		spec, err := ParseSpec([]byte(record.Spec), "")
		if err != nil {
			continue
		}

		for _, node := range spec.Nodes {
			if strings.TrimSpace(node.WorkflowSlug) == targetID {
				embedders = append(embedders, record.Slug)
				break
			}
		}
	}

	return embedders, nil
}

// Delete removes a workflow from the library: the installed spec and
// any pending draft. Run history and the version history deliberately
// stay — deleting a workflow does not rewrite the past.
func (r *Registry) Delete(ctx context.Context, workflowID string) error {
	trimmedID := strings.TrimSpace(workflowID)
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, trimmedID)
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return ez.New(ez.ENOTFOUND, "Workflow "+trimmedID+" was not found", nil)
		}

		return ez.Wrap(err)
	}

	if record.Spec != "" {
		embedders, err := r.listEmbedders(ctx, trimmedID)
		if err != nil {
			return ez.Wrap(err)
		}
		if len(embedders) > 0 {
			return ez.New(
				ez.EINVALID,
				"Workflow "+trimmedID+" is embedded by "+
					strings.Join(embedders, ", ")+
					" — remove those references first",
				nil,
			)
		}
	}

	err = record.Delete(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
