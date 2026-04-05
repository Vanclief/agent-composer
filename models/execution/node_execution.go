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
	_ relational.PaginableModel = (*NodeExecution)(nil)
	_ relational.DBModel        = (*NodeExecution)(nil)
)

type NodeExecution struct {
	bun.BaseModel `bun:"table:node_executions"`

	ID                    uuid.UUID           `bun:",pk,type:uuid" json:"id"`
	WorkflowExecutionID   uuid.UUID           `bun:"type:uuid" json:"workflow_execution_id"`
	ParentNodeExecutionID uuid.UUID           `bun:"type:uuid,nullzero" json:"parent_node_execution_id,omitempty"`
	NodeID                string              `json:"node_id"`
	Kind                  string              `json:"kind"`
	Status                NodeExecutionStatus `json:"status"`
	NodeSnapshot          json.RawMessage     `bun:"type:jsonb" json:"node_snapshot"`
	InputSnapshot         map[string]any      `bun:"type:jsonb,nullzero" json:"input_snapshot,omitempty"`
	OutputSnapshot        map[string]any      `bun:"type:jsonb,nullzero" json:"output_snapshot,omitempty"`
	Trace                 map[string]any      `bun:"type:jsonb,nullzero" json:"trace,omitempty"`
	IterationIndex        *int                `bun:",nullzero" json:"iteration_index,omitempty"`
	BranchName            string              `json:"branch_name,omitempty"`
	StartedAt             *time.Time          `bun:",nullzero" json:"started_at,omitempty"`
	FinishedAt            *time.Time          `bun:",nullzero" json:"finished_at,omitempty"`
	Metadata              map[string]any      `bun:"type:jsonb,nullzero" json:"metadata,omitempty"`
	CreatedAt             time.Time           `json:"created_at"`
}

func (n *NodeExecution) Validate() error {
	const op = "execution.NodeExecution.Validate"

	if n == nil {
		return ez.New(op, ez.EINVALID, "node execution is nil", nil)
	}

	if n.WorkflowExecutionID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "workflow_execution_id is required", nil)
	}

	if n.NodeID == "" {
		return ez.New(op, ez.EINVALID, "node_id is required", nil)
	}

	if n.Kind == "" {
		return ez.New(op, ez.EINVALID, "kind is required", nil)
	}

	if len(n.NodeSnapshot) == 0 {
		return ez.New(op, ez.EINVALID, "node_snapshot is required", nil)
	}

	err := n.Status.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (n *NodeExecution) Insert(ctx context.Context, db bun.IDB) error {
	const op = "execution.NodeExecution.Insert"

	if n.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}

		n.ID = id
	}

	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}

	err := n.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(n).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (n *NodeExecution) Update(ctx context.Context, db bun.IDB) error {
	const op = "execution.NodeExecution.Update"

	if n.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := n.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(n).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (n *NodeExecution) Delete(ctx context.Context, db bun.IDB) error {
	const op = "execution.NodeExecution.Delete"

	if n.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(n).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func GetNodeExecutionByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*NodeExecution, error) {
	const op = "execution.GetNodeExecutionByID"

	record := new(NodeExecution)
	err := db.NewSelect().
		Model(record).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ez.New(op, ez.ENOTFOUND, fmt.Sprintf("node execution with ID %s not found", id), err)
		}

		return nil, ez.Wrap(op, err)
	}

	return record, nil
}

func (n NodeExecution) GetCursor() string {
	return n.ID.String()
}

func (n NodeExecution) GetSortField() string {
	return "node_execution.id"
}

func (n NodeExecution) GetSortValue() interface{} {
	return n.ID
}

func (n NodeExecution) GetUniqueField() string {
	return "node_execution.id"
}

func (n NodeExecution) GetUniqueValue() interface{} {
	return n.ID
}
