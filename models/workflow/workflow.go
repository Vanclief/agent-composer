package workflow

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Workflow is a registry row: the workflow's permanent identity plus
// its current spec. Spec holds the installed YAML head and Draft
// holds a proposed spec awaiting an explicit save. A workflow that was
// composed but never saved has an empty Spec.
type Workflow struct {
	bun.BaseModel `bun:"table:workflows"`

	ID uuid.UUID `bun:",pk,type:uuid" json:"id"`
	// Slug is the human-facing workflow id — renameable, unlike ID.
	Slug string `bun:",notnull,unique" json:"slug"`
	// Version counts installed heads; it is 0 while only a draft exists.
	Version   int       `json:"version"`
	Spec      string    `json:"spec"`
	Draft     string    `json:"draft,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w *Workflow) Validate() error {
	if w == nil {
		return ez.New(ez.EINVALID, "workflow is nil", nil)
	}

	if w.Slug == "" {
		return ez.New(ez.EINVALID, "slug is required", nil)
	}

	return nil
}

func (w *Workflow) Insert(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		w.ID = id
	}

	now := time.Now().UTC()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = now
	}

	err := w.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(w).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (w *Workflow) Update(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", nil)
	}

	err := w.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	w.UpdatedAt = time.Now().UTC()

	_, err = db.NewUpdate().Model(w).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (w *Workflow) Delete(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", nil)
	}

	_, err := db.NewDelete().Model(w).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func GetWorkflowBySlug(ctx context.Context, db bun.IDB, slug string) (*Workflow, error) {
	record := new(Workflow)
	err := db.NewSelect().
		Model(record).
		Where("slug = ?", slug).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ez.New(ez.ENOTFOUND, "Workflow "+slug+" was not found", err)
		}

		return nil, ez.Wrap(err)
	}

	return record, nil
}

func ListWorkflows(ctx context.Context, db bun.IDB) ([]Workflow, error) {
	records := []Workflow{}
	err := db.NewSelect().
		Model(&records).
		Order("slug ASC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return records, nil
}

// WorkflowVersion is one immutable entry in a workflow's history.
// Every installed head gets a row, so any past version can be
// inspected or restored. Rows outlive their workflow — deleting a
// workflow does not rewrite the past.
type WorkflowVersion struct {
	bun.BaseModel `bun:"table:workflow_versions"`

	ID         uuid.UUID `bun:",pk,type:uuid" json:"id"`
	WorkflowID uuid.UUID `bun:"type:uuid,notnull" json:"workflow_id"`
	Version    int       `bun:",notnull" json:"version"`
	Spec       string    `bun:",notnull" json:"spec"`
	CreatedAt  time.Time `json:"created_at"`
}

func (v *WorkflowVersion) Validate() error {
	if v == nil {
		return ez.New(ez.EINVALID, "workflow version is nil", nil)
	}

	if v.WorkflowID == uuid.Nil {
		return ez.New(ez.EINVALID, "workflow_id is required", nil)
	}

	if v.Version < 1 {
		return ez.New(ez.EINVALID, "version must be at least 1", nil)
	}

	if v.Spec == "" {
		return ez.New(ez.EINVALID, "spec is required", nil)
	}

	return nil
}

func (v *WorkflowVersion) Insert(ctx context.Context, db bun.IDB) error {
	if v.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		v.ID = id
	}

	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}

	err := v.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(v).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func GetWorkflowVersion(ctx context.Context, db bun.IDB, workflowID uuid.UUID, version int) (*WorkflowVersion, error) {
	record := new(WorkflowVersion)
	err := db.NewSelect().
		Model(record).
		Where("workflow_id = ? AND version = ?", workflowID, version).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ez.New(ez.ENOTFOUND, "The requested version was not found", err)
		}

		return nil, ez.Wrap(err)
	}

	return record, nil
}

func ListWorkflowVersions(ctx context.Context, db bun.IDB, workflowID uuid.UUID) ([]WorkflowVersion, error) {
	records := []WorkflowVersion{}
	err := db.NewSelect().
		Model(&records).
		Where("workflow_id = ?", workflowID).
		Order("version DESC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return records, nil
}
