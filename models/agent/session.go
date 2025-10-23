package agent

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/runtime/llm"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

var (
	_ relational.PaginableModel = (*Session)(nil)
	_ relational.DBModel        = (*Session)(nil)
)

type Session struct {
	bun.BaseModel `bun:"table:agent_sessions"`

	ID           uuid.UUID            `bun:",pk,type:uuid" json:"id"`
	AgentSpecID  uuid.UUID            `bun:"type:uuid" json:"agent_spec_id"`
	Name         string               `json:"name"`
	Provider     LLMProvider          `json:"provider"`
	Instructions string               `json:"instructions"`
	Tools        []llm.ToolDefinition `bun:"type:jsonb,nullzero" json:"-"`
	Messages     []llm.Message        `bun:"type:jsonb,nullzero" json:"messages"`
	Status       SessionStatus        `json:"status"`
}

// ---- Constructor ----

func NewAgentSession(agentSpec *Spec, messages []llm.Message) (*Session, error) {
	const op = "agent.NewAgentSession"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	session := &Session{
		ID:           id,
		AgentSpecID:  agentSpec.ID,
		Name:         agentSpec.Name,
		Provider:     agentSpec.Provider,
		Instructions: agentSpec.Instructions,
		Messages:     messages,
		Status:       SessionStatusQueued,
	}

	err = session.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return session, nil
}

// ---- Validation ----

func (s *Session) Validate() error {
	const op = "Session.Validate"

	if s.AgentSpecID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "agent_spec_id is required", nil)
	}

	if s.Name == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	if s.Instructions == "" {
		return ez.New(op, ez.EINVALID, "instructions are required", nil)
	}

	if err := s.Provider.Validate(); err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

// ---- CRUD ----

func (s *Session) Insert(ctx context.Context, db bun.IDB) error {
	const op = "Session.Insert"

	if s.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}
		s.ID = id
	}

	err := s.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(s).Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (s *Session) Update(ctx context.Context, db bun.IDB) error {
	const op = "Session.Update"

	if s.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := s.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(s).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (s *Session) Delete(ctx context.Context, db bun.IDB) error {
	const op = "Session.Delete"

	if s.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(s).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

// ---- Queries ----

func GetAgentSessionByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Session, error) {
	const op = "agent.GetAgentSessionByID"

	session := new(Session)
	err := db.NewSelect().
		Model(session).
		Where("session.id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session, ez.New(op, ez.ENOTFOUND, "agent session not found", err)
		}
		return session, ez.Wrap(op, err)
	}
	return session, nil
}

func GetAgentSessionsBySpecID(ctx context.Context, db bun.IDB, agentSpecID uuid.UUID) ([]*Session, error) {
	const op = "agent.GetAgentSessionsBySpecID"

	var sessions []*Session
	err := db.NewSelect().
		Model(&sessions).
		Where("session.agent_spec_id = ?", agentSpecID).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return sessions, nil
}

// ---- Pagination helpers ----

func (s Session) GetCursor() string {
	return s.ID.String()
}

func (s Session) GetSortField() string {
	return "session.id"
}

func (s Session) GetSortValue() interface{} {
	return s.ID
}

func (s Session) GetUniqueField() string {
	return "session.id"
}

func (s Session) GetUniqueValue() interface{} {
	return s.ID
}
