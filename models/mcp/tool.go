package mcp

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

var (
	_ relational.PaginableModel = (*ToolPolicy)(nil)
	_ relational.DBModel        = (*ToolPolicy)(nil)
)

// ToolPolicy defines which tools from a given MCP are allowed.
type ToolPolicy struct {
	bun.BaseModel `bun:"table:tool_policies"`

	ID      uuid.UUID `bun:",pk,type:uuid" json:"id"`
	MCPName string    `json:"mcp_name"`
	Tools   []string  `bun:",array" json:"tools"` // Allowlist of tool names
}

// ---- Constructor ----

func NewToolPolicy(mcpName string, tools []string) (*ToolPolicy, error) {
	const op = "parrot.NewToolPolicy"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	tp := &ToolPolicy{
		ID:      id,
		MCPName: strings.TrimSpace(mcpName),
		Tools:   tools,
	}

	err = tp.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return tp, nil
}

// ---- Validation ----

func (tp *ToolPolicy) Validate() error {
	const op = "ToolPolicy.Validate"

	if tp.MCPName == "" {
		return ez.New(op, ez.EINVALID, "mcp_name is required", nil)
	}

	if len(tp.Tools) == 0 {
		return ez.New(op, ez.EINVALID, "at least one tool is required", nil)
	}

	for i, tool := range tp.Tools {
		tp.Tools[i] = strings.TrimSpace(tool)
		if tp.Tools[i] == "" {
			return ez.New(op, ez.EINVALID, "tool name cannot be empty", nil)
		}
	}

	return nil
}

// ---- CRUD ----

func (tp *ToolPolicy) Insert(ctx context.Context, db bun.IDB) error {
	const op = "ToolPolicy.Insert"

	if tp.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}
		tp.ID = id
	}

	err := tp.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(tp).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (tp *ToolPolicy) Update(ctx context.Context, db bun.IDB) error {
	const op = "ToolPolicy.Update"

	if tp.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := tp.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(tp).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (tp *ToolPolicy) Delete(ctx context.Context, db bun.IDB) error {
	const op = "ToolPolicy.Delete"

	if tp.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(tp).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

// ---- Queries ----

func GetToolPolicyByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*ToolPolicy, error) {
	const op = "parrot.GetToolPolicyByID"

	tp := new(ToolPolicy)
	err := db.NewSelect().
		Model(tp).
		Where("tool_policy.id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tp, ez.New(op, ez.ENOTFOUND, "tool policy not found", err)
		}
		return tp, ez.Wrap(op, err)
	}
	return tp, nil
}

func GetToolPoliciesByMCP(ctx context.Context, db bun.IDB, mcpName string) ([]*ToolPolicy, error) {
	const op = "parrot.GetToolPoliciesByMCP"

	var policies []*ToolPolicy
	err := db.NewSelect().
		Model(&policies).
		Where("tool_policy.mcp_name = ?", strings.TrimSpace(mcpName)).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return policies, nil
}

// ---- Pagination helpers (UUIDv7-ordered) ----

func (tp ToolPolicy) GetCursor() string {
	return tp.ID.String()
}

func (tp ToolPolicy) GetSortField() string {
	return "tool_policy.id"
}

func (tp ToolPolicy) GetSortValue() interface{} {
	return tp.ID
}

func (tp ToolPolicy) GetUniqueField() string {
	return "tool_policy.id"
}

func (tp ToolPolicy) GetUniqueValue() interface{} {
	return tp.ID
}
