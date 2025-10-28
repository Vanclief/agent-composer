package hook

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

type Hook struct {
	bun.BaseModel `bun:"table:hooks"`

	ID        uuid.UUID `bun:",pk,type:uuid" json:"id"`
	EventType EventType `bun:"event_type,notnull" json:"event_type"`
	AgentName string    `bun:"agent_name" json:"agent_name"` // empty = wildcard
	Command   string    `bun:"command,notnull" json:"command"`
	Args      []string  `bun:"args,array" json:"args"`
	Enabled   bool      `bun:"enabled" json:"enabled"`
}

func NewHook(eventType EventType, agentName, command string, args []string, enabled bool) (*Hook, error) {
	const op = "hook.NewHook"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	h := &Hook{
		ID:        id,
		EventType: eventType,
		AgentName: agentName,
		Command:   command,
		Args:      args,
		Enabled:   enabled,
	}

	err = h.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return h, nil
}

func (h Hook) IsWildcard() bool {
	return h.AgentName == ""
}

// Paginatable-style helpers
func (h Hook) GetCursor() string {
	return fmt.Sprintf("%s:%s", h.GetSortValue(), h.GetUniqueValue())
}

func (h Hook) GetSortField() string {
	return `"hook".agent_name`
}

func (h Hook) GetSortValue() interface{} {
	return h.AgentName
}

func (h Hook) GetUniqueField() string {
	return `"hook".id`
}

func (h Hook) GetUniqueValue() interface{} {
	return h.ID.String()
}

// Validation
func (h *Hook) Validate() error {
	if strings.TrimSpace(h.Command) == "" {
		return fmt.Errorf("command must not be empty")
	}
	return nil
}

// CRUD
func (h *Hook) Insert(ctx context.Context, db bun.IDB) error {
	const op = "hook.Insert"

	err := h.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(h).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (h *Hook) Update(ctx context.Context, db bun.IDB) error {
	const op = "hook.Update"

	err := h.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(h).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (h *Hook) Delete(ctx context.Context, db bun.IDB) error {
	const op = "hook.Delete"

	_, err := db.NewDelete().Model(h).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}
