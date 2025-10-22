package parrot

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/models/provider"
	"github.com/vanclief/agent-composer/runtime/llm"
)

var (
	_ relational.PaginableModel = (*Template)(nil)
	_ relational.DBModel        = (*Template)(nil)
)

// NOTE: I could keep the provider as separate so you can
// run the same template with different providers. Depends
// on if it makes sense to have the same instructions with different
// providers

type Template struct {
	bun.BaseModel `bun:"table:parrot_templates"`

	ID              uuid.UUID            `bun:",pk,type:uuid" json:"id"`
	Name            string               `json:"name"`
	Provider        provider.LLMProvider `json:"provider"`
	Model           string               `json:"model"`
	ReasoningEffort llm.ReasoningEffort  `json:"reasoning_effort"`
	Instructions    string               `json:"instructions"`
	AllowedTools    []string             `bun:"allowed_tools,type:jsonb,nullzero" json:"allowed_tools"`
	Version         int                  `json:"version"`
}

// ---- Constructor ----

func NewParrotTemplate(name string, prov provider.LLMProvider, model, instructions string, reasoningEffort llm.ReasoningEffort, version int, allowedTools []string) (*Template, error) {
	const op = "parrot.NewParrotTemplate"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	pt := &Template{
		ID:              id,
		Name:            strings.TrimSpace(name),
		Provider:        prov,
		Model:           strings.TrimSpace(model),
		Instructions:    strings.TrimSpace(instructions),
		ReasoningEffort: reasoningEffort,
		AllowedTools:    allowedTools,
		Version:         version,
	}

	err = pt.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}

// ---- Validation ----

func (pt *Template) Validate() error {
	const op = "Template.Validate"

	if pt.Name == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	if pt.Instructions == "" {
		return ez.New(op, ez.EINVALID, "instructions are required", nil)
	}

	if pt.Version <= 0 {
		return ez.New(op, ez.EINVALID, "version must be > 0", nil)
	}

	if err := pt.Provider.Validate(); err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

// ---- CRUD ----

func (pt *Template) Insert(ctx context.Context, db bun.IDB) error {
	const op = "Template.Insert"

	if pt.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}
		pt.ID = id
	}

	err := pt.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(pt).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (pt *Template) Update(ctx context.Context, db bun.IDB) error {
	const op = "Template.Update"

	if pt.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := pt.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(pt).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (pt *Template) Delete(ctx context.Context, db bun.IDB) error {
	const op = "Template.Delete"

	if pt.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(pt).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

// ---- Queries ----

func GetParrotTemplateByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Template, error) {
	const op = "parrot.GetParrotTemplateByID"

	pt := new(Template)
	err := db.NewSelect().
		Model(pt).
		Where("template.id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pt, ez.New(op, ez.ENOTFOUND, "parrot template not found", err)
		}
		return pt, ez.Wrap(op, err)
	}
	return pt, nil
}

// ---- Pagination helpers ----

func (pt Template) GetCursor() string {
	return pt.ID.String()
}

func (pt Template) GetSortField() string {
	return "template.id"
}

func (pt Template) GetSortValue() interface{} {
	return pt.ID
}

func (pt Template) GetUniqueField() string {
	return "template.id"
}

func (pt Template) GetUniqueValue() interface{} {
	return pt.ID
}
