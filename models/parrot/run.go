package parrot

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/provider"
	"github.com/vanclief/agent-composer/runtime/llm"
)

var (
	_ relational.PaginableModel = (*Run)(nil)
	_ relational.DBModel        = (*Run)(nil)
)

type Run struct {
	bun.BaseModel `bun:"table:parrot_runs"`

	ID           uuid.UUID            `bun:",pk,type:uuid" json:"id"`
	TemplateID   uuid.UUID            `bun:"type:uuid" json:"template_id"`
	Name         string               `json:"name"`
	Provider     provider.LLMProvider `json:"provider"`
	Instructions string               `json:"instructions"`
	Tools        []llm.ToolDefinition `bun:"type:jsonb,nullzero" json:"-"`
	Messages     []llm.Message        `bun:"type:jsonb,nullzero" json:"messages"`
	Status       RunStatus            `json:"status"`
}

// ---- Constructor ----

func NewParrotRun(template *Template, messages []llm.Message) (*Run, error) {
	const op = "parrot.NewParrotRun"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	pr := &Run{
		ID:           id,
		TemplateID:   template.ID,
		Name:         template.Name,
		Provider:     template.Provider,
		Instructions: template.Instructions,
		Messages:     messages,
		Status:       RunStatusQueued,
	}

	err = pr.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pr, nil
}

// ---- Validation ----

func (pr *Run) Validate() error {
	const op = "Run.Validate"

	if pr.TemplateID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "template_id is required", nil)
	}

	if pr.Name == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	if pr.Instructions == "" {
		return ez.New(op, ez.EINVALID, "instructions are required", nil)
	}

	if err := pr.Provider.Validate(); err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

// ---- CRUD ----

func (pr *Run) Insert(ctx context.Context, db bun.IDB) error {
	const op = "Run.Insert"

	if pr.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}
		pr.ID = id
	}

	err := pr.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(pr).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (pr *Run) Update(ctx context.Context, db bun.IDB) error {
	const op = "Run.Update"

	if pr.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := pr.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(pr).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (pr *Run) Delete(ctx context.Context, db bun.IDB) error {
	const op = "Run.Delete"

	if pr.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(pr).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

// ---- Queries ----

func GetParrotRunByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Run, error) {
	const op = "parrot.GetParrotRunByID"

	pr := new(Run)
	err := db.NewSelect().
		Model(pr).
		Where("run.id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pr, ez.New(op, ez.ENOTFOUND, "parrot run not found", err)
		}
		return pr, ez.Wrap(op, err)
	}
	return pr, nil
}

func GetParrotRunsByTemplateID(ctx context.Context, db bun.IDB, templateID uuid.UUID) ([]*Run, error) {
	const op = "parrot.GetParrotRunsByTemplateID"

	var runs []*Run
	err := db.NewSelect().
		Model(&runs).
		Where("run.template_id = ?", templateID).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return runs, nil
}

// ---- Pagination helpers ----

func (pr Run) GetCursor() string {
	return pr.ID.String()
}

func (pr Run) GetSortField() string {
	return "run.id"
}

func (pr Run) GetSortValue() interface{} {
	return pr.ID
}

func (pr Run) GetUniqueField() string {
	return "run.id"
}

func (pr Run) GetUniqueValue() interface{} {
	return pr.ID
}
