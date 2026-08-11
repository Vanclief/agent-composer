package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

var (
	_ relational.PaginableModel = (*WorkflowExecution)(nil)
	_ relational.DBModel        = (*WorkflowExecution)(nil)
)

type WorkflowExecution struct {
	bun.BaseModel `bun:"table:workflow_executions"`

	ID           uuid.UUID `bun:",pk,type:uuid" json:"id"`
	WorkflowSlug string    `json:"workflow_slug"`
	// WorkflowID is the workflow's permanent identity — the slug can
	// be renamed, this cannot.
	WorkflowID       uuid.UUID               `bun:"type:uuid,nullzero" json:"workflow_id,omitempty"`
	WorkflowVersion  string                  `json:"workflow_version"`
	WorkflowSnapshot json.RawMessage         `bun:"type:jsonb" json:"workflow_snapshot"`
	InputSnapshot    map[string]any          `bun:"type:jsonb,nullzero" json:"input_snapshot,omitempty"`
	OutputSnapshot   map[string]any          `bun:"type:jsonb,nullzero" json:"output_snapshot,omitempty"`
	Status           WorkflowExecutionStatus `json:"status"`
	ProjectDir       string                  `json:"project_dir,omitempty"`
	StartedAt        *time.Time              `bun:",nullzero" json:"started_at,omitempty"`
	FinishedAt       *time.Time              `bun:",nullzero" json:"finished_at,omitempty"`
	Metadata         map[string]any          `bun:"type:jsonb,nullzero" json:"metadata,omitempty"`
	CreatedAt        time.Time               `json:"created_at"`
}

func (w *WorkflowExecution) Validate() error {
	if w == nil {
		return ez.New(ez.EINVALID, "workflow execution is nil", nil)
	}

	if w.WorkflowSlug == "" {
		return ez.New(ez.EINVALID, "workflow_id is required", nil)
	}

	if w.WorkflowVersion == "" {
		return ez.New(ez.EINVALID, "workflow_version is required", nil)
	}

	if len(w.WorkflowSnapshot) == 0 {
		return ez.New(ez.EINVALID, "workflow_snapshot is required", nil)
	}

	err := w.Status.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (w *WorkflowExecution) Insert(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		w.ID = id
	}

	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now().UTC()
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

func (w *WorkflowExecution) Update(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", nil)
	}

	err := w.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewUpdate().Model(w).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (w *WorkflowExecution) Delete(ctx context.Context, db bun.IDB) error {
	if w.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(w).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func GetWorkflowExecutionByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*WorkflowExecution, error) {
	record := new(WorkflowExecution)
	err := db.NewSelect().
		Model(record).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("workflow execution with ID %s not found", id), err)
		}

		return nil, ez.Wrap(err)
	}

	return record, nil
}

func (w WorkflowExecution) GetCursor() string {
	return w.ID.String()
}

func (w WorkflowExecution) GetSortField() string {
	return "workflow_execution.id"
}

func (w WorkflowExecution) GetSortValue() interface{} {
	return w.ID
}

func (w WorkflowExecution) GetUniqueField() string {
	return "workflow_execution.id"
}

func (w WorkflowExecution) GetUniqueValue() interface{} {
	return w.ID
}
