package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

var (
	_ relational.PaginableModel = (*Conversation)(nil)
	_ relational.DBModel        = (*Conversation)(nil)
)

type Conversation struct {
	bun.BaseModel `bun:"table:conversations"`

	ID                     uuid.UUID              `bun:",pk,type:uuid" json:"id"`
	AgentSpecID            uuid.UUID              `bun:"type:uuid" json:"agent_spec_id"`
	AgentName              string                 `json:"agent_name"`
	Provider               LLMProvider            `json:"provider"`
	Model                  string                 `json:"model"`
	ReasoningEffort        types.ReasoningEffort  `json:"reasoning_effort"`
	Instructions           string                 `json:"instructions"`
	Tools                  []types.ToolDefinition `bun:"type:jsonb,nullzero" json:"-"`
	Messages               []types.Message        `bun:"type:jsonb,nullzero" json:"messages"`
	Status                 ConversationStatus     `json:"status"`
	InputTokens            int64                  `json:"input_tokens"`
	OutputTokens           int64                  `json:"output_tokens"`
	CachedTokens           int64                  `json:"cached_tokens"`
	Cost                   int64                  `json:"cost"`
	CreatedAt              time.Time              `json:"created_at"`
	AutoCompact            bool                   `json:"auto_compact"`
	CompactAtPercent       int                    `json:"compact_at_percent"`
	CompactionPrompt       string                 `json:"compaction_prompt"`
	CompactCount           int                    `json:"compact_count"`
	ShellAccess            bool                   `json:"shell_access"`
	WebSearch              bool                   `json:"web_search"`
	StructuredOutput       bool                   `json:"structured_output"`
	StructuredOutputSchema map[string]any         `bun:"type:jsonb,nullzero" json:"structured_output_schema"`
}

// ---- Constructor ----

func NewConversation(agentSpec *Spec, messages []types.Message) (*Conversation, error) {
	const op = "agent.NewConversation"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	conversation := &Conversation{
		ID:                     id,
		AgentSpecID:            agentSpec.ID,
		AgentName:              agentSpec.Name,
		Provider:               agentSpec.Provider,
		Model:                  agentSpec.Model,
		ReasoningEffort:        agentSpec.ReasoningEffort,
		Instructions:           agentSpec.Instructions,
		Messages:               messages,
		Status:                 ConversationStatusQueued,
		CreatedAt:              time.Now().UTC(),
		AutoCompact:            agentSpec.AutoCompact,
		CompactAtPercent:       agentSpec.CompactAtPercent,
		CompactionPrompt:       agentSpec.CompactionPrompt,
		CompactCount:           0,
		ShellAccess:            agentSpec.ShellAccess,
		WebSearch:              agentSpec.WebSearch,
		StructuredOutput:       agentSpec.StructuredOutput,
		StructuredOutputSchema: agentSpec.StructuredOutputSchema,
	}

	err = conversation.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return conversation, nil
}

// ---- Validation ----

func (c *Conversation) Validate() error {
	const op = "Conversation.Validate"

	if c.AgentSpecID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "agent_spec_id is required", nil)
	}

	if c.AgentName == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	if c.Instructions == "" {
		return ez.New(op, ez.EINVALID, "instructions are required", nil)
	}

	if err := c.Provider.Validate(); err != nil {
		return ez.Wrap(op, err)
	}

	if c.CompactAtPercent <= 0 || c.CompactAtPercent > 100 {
		return ez.New(op, ez.EINVALID, "compact_at_percent must be between 1 and 100", nil)
	}

	return nil
}

func (c *Conversation) Insert(ctx context.Context, db bun.IDB) error {
	const op = "Conversation.Insert"

	if c.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(op, err)
		}
		c.ID = id
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}

	err := c.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewInsert().Model(c).Exec(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to insert conversation")
		return ez.Wrap(op, err)
	}

	return nil
}

func (c *Conversation) Update(ctx context.Context, db bun.IDB) error {
	const op = "Conversation.Update"

	if c.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", nil)
	}

	err := c.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	_, err = db.NewUpdate().Model(c).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}

	return nil
}

func (c *Conversation) Delete(ctx context.Context, db bun.IDB) error {
	const op = "Conversation.Delete"

	if c.ID == uuid.Nil {
		return ez.New(op, ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(c).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(op, err)
	}
	return nil
}

func (c *Conversation) Clone(ctx context.Context, db bun.IDB, discardMessages bool) (*Conversation, error) {
	const op = "Conversation.Clone"

	if c == nil {
		return nil, ez.New(op, ez.EINVALID, "conversation is nil", nil)
	}

	// Create a value copy so mutations don't affect the original object.
	clone := *c

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	clone.ID = id
	clone.CreatedAt = time.Now().UTC()
	clone.InputTokens = 0
	clone.OutputTokens = 0
	clone.CachedTokens = 0

	if discardMessages {
		clone.Messages = []types.Message{*types.NewSystemMessage(clone.Instructions)}
	} else if len(c.Messages) > 0 {
		clone.Messages = append([]types.Message(nil), c.Messages...)
	}

	_, err = db.NewInsert().Model(&clone).Exec(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return &clone, nil
}

// ---- Queries ----

func GetConversationByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Conversation, error) {
	const op = "agent.GetConversationByID"

	conversation := new(Conversation)
	err := db.NewSelect().
		Model(conversation).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errMsg := fmt.Sprintf("conversation with ID %s not found", id)
			return nil, ez.New(op, ez.ENOTFOUND, errMsg, err)
		}
		return nil, ez.Wrap(op, err)
	}
	return conversation, nil
}

func GetConversationsBySpecID(ctx context.Context, db bun.IDB, agentSpecID uuid.UUID) ([]*Conversation, error) {
	const op = "agent.GetConversationsBySpecID"

	var conversations []*Conversation
	err := db.NewSelect().
		Model(&conversations).
		Where("conversation.agent_spec_id = ?", agentSpecID).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return conversations, nil
}

// ---- Pagination helpers ----

func (c Conversation) GetCursor() string {
	return c.ID.String()
}

func (c Conversation) GetSortField() string {
	return "conversation.id"
}

func (c Conversation) GetSortValue() interface{} {
	return c.ID
}

func (c Conversation) GetUniqueField() string {
	return "conversation.id"
}

func (c Conversation) GetUniqueValue() interface{} {
	return c.ID
}
