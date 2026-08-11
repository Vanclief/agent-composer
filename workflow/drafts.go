package workflow

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	workflowmodels "github.com/vanclief/agent-composer/models/workflow"
	"github.com/vanclief/ez"
	yaml "gopkg.in/yaml.v3"
)

// Drafts are proposed specs awaiting an explicit save. They live
// on the workflow's registry row next to the installed spec, so a
// draft is never executable until promoted.

// ReadDraft returns the draft spec for a workflow, or "" when
// none exists.
func (r *Registry) ReadDraft(ctx context.Context, workflowID string) (string, error) {
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, strings.TrimSpace(workflowID))
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return "", nil
		}

		return "", ez.Wrap(err)
	}

	return record.Draft, nil
}

// WriteDraft stores a proposed spec. The content must already be
// compile-checked by the caller. A workflow that only exists as a
// draft gets its registry row (and permanent identity) here.
func (r *Registry) WriteDraft(ctx context.Context, workflowID string, raw []byte) error {
	trimmedID := strings.TrimSpace(workflowID)
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, trimmedID)
	if err != nil {
		if ez.ErrorCode(err) != ez.ENOTFOUND {
			return ez.Wrap(err)
		}

		record, err = r.createRow(ctx, trimmedID, "")
		if err != nil {
			return ez.Wrap(err)
		}
	}

	record.Draft = string(raw)

	err = record.Update(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// DeleteDraft discards a draft — a missing draft is not an error. A
// draft-only workflow disappears entirely — there is nothing else to
// keep.
func (r *Registry) DeleteDraft(ctx context.Context, workflowID string) error {
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, strings.TrimSpace(workflowID))
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return nil
		}

		return ez.Wrap(err)
	}

	if record.Spec == "" {
		err = record.Delete(ctx, r.db)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	}

	if record.Draft == "" {
		return nil
	}

	record.Draft = ""

	err = record.Update(ctx, r.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// slugifyWorkflowID turns a display name into a registry-style id:
// lowercase snake_case, matching the installed workflows' convention.
func slugifyWorkflowID(name string) string {
	var builder strings.Builder
	pendingSeparator := false
	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		isWordChar := (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9')
		if !isWordChar {
			pendingSeparator = builder.Len() > 0
			continue
		}
		if pendingSeparator {
			builder.WriteByte('_')
			pendingSeparator = false
		}
		builder.WriteRune(char)
	}

	return builder.String()
}

type CreatedDraft struct {
	WorkflowSlug string
	Spec         string
}

// CreateDraft scaffolds a new named workflow as a draft: just the
// workflow header, no nodes — the composer and inspector fill in the
// rest. The id derives from the name unless the caller picks one;
// collisions with installed workflows or existing drafts are rejected.
func (r *Registry) CreateDraft(ctx context.Context, name, description, explicitID string) (*CreatedDraft, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ez.New(ez.EINVALID, "A workflow name is required", nil)
	}

	workflowID := strings.TrimSpace(explicitID)
	if workflowID == "" {
		workflowID = slugifyWorkflowID(trimmedName)
		if workflowID == "" {
			return nil, ez.New(ez.EINVALID, "The name must contain letters or digits", nil)
		}
	} else {
		err := ValidateWorkflowID(workflowID)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	existing, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, workflowID)
	if err != nil && ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}
	if err == nil {
		if existing.Spec != "" {
			return nil, ez.New(ez.EINVALID, "Workflow "+workflowID+" already exists", nil)
		}

		return nil, ez.New(ez.EINVALID, "A draft for "+workflowID+" already exists", nil)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	var scaffold struct {
		Workflow struct {
			Slug        string `yaml:"slug"`
			ID          string `yaml:"id"`
			Name        string `yaml:"name"`
			Version     string `yaml:"version"`
			Description string `yaml:"description,omitempty"`
		} `yaml:"workflow"`
	}
	scaffold.Workflow.Slug = workflowID
	scaffold.Workflow.ID = id.String()
	scaffold.Workflow.Name = trimmedName
	scaffold.Workflow.Version = "1"
	scaffold.Workflow.Description = strings.TrimSpace(description)

	raw, err := yaml.Marshal(&scaffold)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record := &workflowmodels.Workflow{
		ID:    id,
		Slug:  workflowID,
		Draft: string(raw),
	}

	err = record.Insert(ctx, r.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &CreatedDraft{
		WorkflowSlug: workflowID,
		Spec:         string(raw),
	}, nil
}

type SavedDraft struct {
	WorkflowSlug string `json:"workflow_slug"`
	Version      string `json:"version"`
	Spec         string `json:"spec"`
}

// SaveDraft promotes a draft: it must compile, the next version is
// stamped, the head is replaced, the outgoing version stays in the
// history, and the draft is cleared.
func (r *Registry) SaveDraft(ctx context.Context, workflowID string) (*SavedDraft, error) {
	trimmedID := strings.TrimSpace(workflowID)
	record, err := workflowmodels.GetWorkflowBySlug(ctx, r.db, trimmedID)
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return nil, ez.New(ez.ENOTFOUND, "There is no draft for "+trimmedID, nil)
		}

		return nil, ez.Wrap(err)
	}
	if record.Draft == "" {
		return nil, ez.New(ez.ENOTFOUND, "There is no draft for "+trimmedID, nil)
	}

	spec, err := ParseSpec([]byte(record.Draft), "")
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if strings.TrimSpace(spec.Workflow.Slug) != trimmedID {
		return nil, ez.New(ez.EINVALID, "The draft's workflow.slug does not match "+trimmedID, nil)
	}

	version := seedVersion(spec.Workflow.Version)
	if record.Spec != "" {
		version = record.Version + 1
	}

	draft := record.Draft
	record.Draft = ""

	_, err = r.saveHead(ctx, record, []byte(draft), version)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &SavedDraft{
		WorkflowSlug: record.Slug,
		Version:      strconv.Itoa(record.Version),
		Spec:         record.Spec,
	}, nil
}

// stampWorkflowHeader rewrites workflow.version and workflow.id in
// place, preserving the rest of the document byte-for-byte where
// possible. Empty values leave their field untouched.
func stampWorkflowHeader(raw []byte, version, workflowID string) ([]byte, error) {
	var doc yaml.Node
	err := yaml.Unmarshal(raw, &doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if len(doc.Content) == 0 {
		return nil, ez.New(ez.EINVALID, "the draft is empty", nil)
	}

	workflowMap := findMapValue(doc.Content[0], "workflow")
	if workflowMap == nil {
		return nil, ez.New(ez.EINVALID, "the draft has no workflow section", nil)
	}

	if version != "" {
		setScalarValue(workflowMap, "version", version)
	}
	if workflowID != "" {
		setScalarValue(workflowMap, "id", workflowID)
	}

	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	err = encoder.Encode(&doc)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = encoder.Close()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return buffer.Bytes(), nil
}

// StampWorkflowID forces workflow.id in a spec's bytes — used to
// carry a workflow's permanent identity into a proposal that dropped
// or fabricated it.
func StampWorkflowID(raw []byte, workflowID string) ([]byte, error) {
	return stampWorkflowHeader(raw, "", workflowID)
}
