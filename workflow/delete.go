package workflow

import (
	"strings"

	"github.com/vanclief/ez"
)

// listEmbedders returns installed workflows that embed targetID via a
// workflow node.
func listEmbedders(targetID string) ([]string, error) {
	const op = "workflow.listEmbedders"

	summaries, err := ListBlueprints()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	embedders := []string{}
	for _, summary := range summaries {
		if summary.ID == targetID {
			continue
		}
		entry, err := loadRegistryBlueprintEntryByWorkflowID(summary.ID)
		if err != nil {
			continue
		}
		for _, node := range entry.Blueprint.Nodes {
			if strings.TrimSpace(node.WorkflowID) == targetID {
				embedders = append(embedders, summary.ID)
				break
			}
		}
	}

	return embedders, nil
}

// DeleteWorkflow removes a workflow from the library: the registry
// file and any pending draft. Run history and the versions archive
// deliberately stay — deleting a workflow does not rewrite the past.
func DeleteWorkflow(workflowID string) error {
	const op = "workflow.DeleteWorkflow"

	workflowID = strings.TrimSpace(workflowID)

	installed := true
	_, err := loadRegistryBlueprintEntryByWorkflowID(workflowID)
	if err != nil {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return ez.Wrap(op, err)
		}
		installed = false
	}
	draft, err := ReadDraft(workflowID)
	if err != nil {
		return ez.Wrap(op, err)
	}
	if !installed && draft == "" {
		return ez.New(op, ez.ENOTFOUND, "Workflow "+workflowID+" was not found", nil)
	}

	if installed {
		embedders, err := listEmbedders(workflowID)
		if err != nil {
			return ez.Wrap(op, err)
		}
		if len(embedders) > 0 {
			return ez.New(
				op,
				ez.EINVALID,
				"Workflow "+workflowID+" is embedded by "+
					strings.Join(embedders, ", ")+
					" — remove those references first",
				nil,
			)
		}

		err = DeleteBlueprintByWorkflowID(workflowID)
		if err != nil {
			return ez.Wrap(op, err)
		}
	}

	err = DeleteDraft(workflowID)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}
