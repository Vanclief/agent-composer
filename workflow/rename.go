package workflow

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

// Workflow ids are friendly slugs, so they double as URLs, CLI
// arguments, and cross-workflow references. Renaming one therefore
// cascades: the registry row, its draft, and every spec that
// embeds the workflow all follow the new id. The executions table is
// the caller's to update.

var workflowIDPattern = regexp.MustCompile(`^[a-z0-9]+([_-][a-z0-9]+)*$`)

// ValidateWorkflowID accepts lowercase slug ids: letters and digits
// separated by single underscores or hyphens.
func ValidateWorkflowID(id string) error {
	if !workflowIDPattern.MatchString(id) {
		return ez.New(ez.EINVALID, "Workflow slugs are lowercase, like parallel_pr_review", nil)
	}

	return nil
}

type RenameResult struct {
	WorkflowSlug string
	// UpdatedRefs lists workflow ids whose specs embedded the
	// renamed workflow and were rewritten to the new id.
	UpdatedRefs []string
}

// encodeYAMLDoc renders an edited yaml.v3 document back to bytes.
func encodeYAMLDoc(doc *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err := encoder.Encode(doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = encoder.Close()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return buffer.Bytes(), nil
}

// rewriteWorkflowHeader surgically sets workflow.slug, workflow.name,
// and/or workflow.description in a spec's bytes. Empty id/name
// leave the field unchanged; a nil description does too, while ""
// removes it.
func rewriteWorkflowHeader(raw []byte, id, name string, description *string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, ez.New(ez.EINVALID, "The workflow file is empty", nil)
	}

	workflowMap := findMapValue(doc.Content[0], "workflow")
	if workflowMap == nil {
		return nil, ez.New(ez.EINVALID, "The file has no workflow section", nil)
	}

	if id != "" {
		setScalarValue(workflowMap, "slug", id)
	}
	if name != "" {
		setScalarValue(workflowMap, "name", name)
	}
	if description != nil {
		if *description == "" {
			removeMapKey(workflowMap, "description")
		} else {
			setScalarValue(workflowMap, "description", *description)
		}
	}

	return encodeYAMLDoc(&doc)
}

// rewriteWorkflowReferences rewrites nodes.*.workflow_slug values from
// oldID to newID. Returns nil bytes when nothing referenced oldID.
func rewriteWorkflowReferences(raw []byte, oldID, newID string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}

	nodesMap := findMapValue(doc.Content[0], "nodes")
	if nodesMap == nil || nodesMap.Kind != yaml.MappingNode {
		return nil, nil
	}

	changed := false
	for i := 0; i+1 < len(nodesMap.Content); i += 2 {
		reference := findMapValue(nodesMap.Content[i+1], "workflow_slug")
		if reference != nil && reference.Value == oldID {
			reference.Value = newID
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}

	return encodeYAMLDoc(&doc)
}

// Rename moves a workflow (installed, drafted, or both) to a new id
// and updates every spec that embeds it. Each rewritten workflow
// gets a new version — a rename is a modification like any other.
func (r *Registry) Rename(ctx context.Context, oldID, newID string) (*RenameResult, error) {
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if oldID == newID {
		return &RenameResult{WorkflowSlug: newID}, nil
	}
	err := ValidateWorkflowID(newID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	// The new id must be free — installed or drafted.
	colliding, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, newID)
	if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}
	if err == nil {
		if colliding.Spec != "" {
			return nil, ez.New(ez.EINVALID, "Workflow "+newID+" already exists", nil)
		}

		return nil, ez.New(ez.EINVALID, "A draft for "+newID+" already exists", nil)
	}

	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, oldID)
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return nil, ez.New(ez.ENOTFOUND, "Workflow "+oldID+" was not found", nil)
		}

		return nil, ez.Wrap(err)
	}

	// 1. The draft follows the new id in memory, so the head save (or
	// the draft-only update) persists both in one write.
	if record.Draft != "" {
		renamedDraft, err := rewriteWorkflowHeader([]byte(record.Draft), newID, "", nil)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		record.Draft = string(renamedDraft)
	}

	// 2. The renamed workflow moves first, so reference rewrites
	// compile against the new id.
	if record.Spec != "" {
		renamed, err := rewriteWorkflowHeader([]byte(record.Spec), newID, "", nil)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		_, err = r.saveHead(ctx, record, renamed, record.Version+1)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	} else {
		record.Slug = newID

		err = record.Update(ctx, r.db)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	// 3. Specs embedding the old id follow it — installed specs
	// get a compile-checked new version, drafts are re-verified at
	// their own save.
	updatedRefs := []string{}
	records, err := workflowmodels.ListWorkflows(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for index := range records {
		other := &records[index]
		if other.ID == record.ID {
			continue
		}

		draftChanged := false
		if other.Draft != "" {
			rewrittenDraft, err := rewriteWorkflowReferences([]byte(other.Draft), oldID, newID)
			if err == nil && rewrittenDraft != nil {
				other.Draft = string(rewrittenDraft)
				draftChanged = true
			}
		}

		specChanged := false
		if record.Spec != "" && other.Spec != "" {
			rewritten, err := rewriteWorkflowReferences([]byte(other.Spec), oldID, newID)
			if err != nil {
				return nil, ez.Wrap(err)
			}
			if rewritten != nil {
				_, err = r.saveHead(ctx, other, rewritten, other.Version+1)
				if err != nil {
					return nil, ez.Wrap(err)
				}

				specChanged = true
				updatedRefs = append(updatedRefs, other.Slug)
			}
		}

		if draftChanged && !specChanged {
			err = other.Update(ctx, r.db)
			if err != nil {
				return nil, ez.Wrap(err)
			}
		}
	}

	return &RenameResult{
		WorkflowSlug: newID,
		UpdatedRefs:  updatedRefs,
	}, nil
}

// SetHeader rewrites workflow.name and/or workflow.description
// wherever the workflow lives — the installed spec (as a new version)
// and/or the pending draft. An empty name is unchanged. A nil
// description is unchanged, and "" clears it.
func (r *Registry) SetHeader(ctx context.Context, workflowID, name string, description *string) error {
	trimmedID := strings.TrimSpace(workflowID)
	name = strings.TrimSpace(name)
	if name == "" && description == nil {
		return ez.New(ez.EINVALID, "Nothing to update", nil)
	}

	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, trimmedID)
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return ez.New(ez.ENOTFOUND, "Workflow "+trimmedID+" was not found", nil)
		}

		return ez.Wrap(err)
	}

	if record.Draft != "" {
		renamedDraft, err := rewriteWorkflowHeader([]byte(record.Draft), "", name, description)
		if err != nil {
			return ez.Wrap(err)
		}
		record.Draft = string(renamedDraft)
	}

	if record.Spec != "" {
		renamed, err := rewriteWorkflowHeader([]byte(record.Spec), "", name, description)
		if err != nil {
			return ez.Wrap(err)
		}

		_, err = r.saveHead(ctx, record, renamed, record.Version+1)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	}

	err = record.Update(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
