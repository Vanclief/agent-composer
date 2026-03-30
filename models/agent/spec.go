package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/core/helpers/jsonutil"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

var (
	_ relational.PaginableModel = (*Spec)(nil)
	_ relational.DBModel        = (*Spec)(nil)
)

type Spec struct {
	bun.BaseModel `bun:"table:agent_specs"`

	ID                     uuid.UUID                    `bun:",pk,type:uuid" json:"id"`
	Name                   string                       `json:"name"`
	Harness                Harness                      `json:"harness"`
	Model                  string                       `bun:"model" json:"model"`
	HarnessConfig          json.RawMessage              `bun:"type:jsonb,nullzero" json:"harness_config,omitempty"`
	ReasoningEffort        runtimetypes.ReasoningEffort `json:"reasoning_effort"`
	Instructions           string                       `json:"instructions"`
	AutoCompact            bool                         `json:"auto_compact"`
	CompactAtPercent       int                          `json:"compact_at_percent"`
	CompactionPrompt       string                       `json:"compaction_prompt"`
	ShellAccess            bool                         `json:"shell_access"`
	WebSearch              bool                         `json:"web_search"`
	StructuredOutput       bool                         `json:"structured_output"`
	StructuredOutputSchema json.RawMessage              `bun:"type:json,nullzero" json:"structured_output_schema"`
	Version                int                          `json:"version"`
}

// ---- Constructor ----

func (pt *Spec) AfterScanRow(ctx context.Context) error {
	normalized, err := jsonutil.NormalizeJSONSchemaPropertiesOrder(pt.StructuredOutputSchema)
	if err == nil {
		pt.StructuredOutputSchema = normalized
	}
	if pt.ReasoningEffort == "" {
		pt.ReasoningEffort = runtimetypes.ReasoningEffortMedium
	}
	return nil
}

func NewAgentSpec(name string, harness Harness, model string, harnessConfig json.RawMessage, instructions string, reasoningEffort runtimetypes.ReasoningEffort, version int) (*Spec, error) {
	const op = "agent.NewAgentSpec"

	id, err := uuid.NewV7()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	pt := &Spec{
		ID:                     id,
		Name:                   strings.TrimSpace(name),
		Harness:                harness,
		Model:                  strings.TrimSpace(model),
		HarnessConfig:          CopyRawJSON(harnessConfig),
		Instructions:           strings.TrimSpace(instructions),
		AutoCompact:            false,
		CompactAtPercent:       90,
		CompactionPrompt:       "",
		ShellAccess:            true,
		WebSearch:              false,
		StructuredOutput:       false,
		StructuredOutputSchema: nil,
		ReasoningEffort:        normalizeReasoningEffort(reasoningEffort),
		Version:                version,
	}

	err = pt.Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return pt, nil
}

// ---- Validation ----

func (pt *Spec) Validate() error {
	const op = "Spec.Validate"

	if pt.Name == "" {
		return ez.New(op, ez.EINVALID, "name is required", nil)
	}

	if pt.Instructions == "" {
		return ez.New(op, ez.EINVALID, "instructions are required", nil)
	}

	if pt.Version <= 0 {
		return ez.New(op, ez.EINVALID, "version must be > 0", nil)
	}

	err := pt.Harness.Validate()
	if err != nil {
		return ez.Wrap(op, err)
	}

	if strings.TrimSpace(pt.Model) == "" {
		return ez.New(op, ez.EINVALID, "model is required", nil)
	}

	err = validateHarnessConfig(pt.HarnessConfig)
	if err != nil {
		return ez.Wrap(op, err)
	}

	if pt.CompactAtPercent <= 0 || pt.CompactAtPercent > 100 {
		return ez.New(op, ez.EINVALID, "compact_at_percent must be between 1 and 100", nil)
	}

	return nil
}

// ---- CRUD ----

func (pt *Spec) Insert(ctx context.Context, db bun.IDB) error {
	const op = "Spec.Insert"

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

func (pt *Spec) Update(ctx context.Context, db bun.IDB) error {
	const op = "Spec.Update"

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

func (pt *Spec) Delete(ctx context.Context, db bun.IDB) error {
	const op = "Spec.Delete"

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

func GetAgentSpecByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Spec, error) {
	const op = "agent.GetAgentSpecByID"

	pt := new(Spec)
	err := db.NewSelect().
		Model(pt).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errMsg := fmt.Sprintf("agent spec with ID %s not found", id)
			return nil, ez.New(op, ez.ENOTFOUND, errMsg, err)
		}
		return nil, ez.Wrap(op, err)
	}
	return pt, nil
}

// ---- Pagination helpers ----

func (pt Spec) GetCursor() string {
	return pt.ID.String()
}

func (pt Spec) GetSortField() string {
	return "spec.id"
}

func (pt Spec) GetSortValue() interface{} {
	return pt.ID
}

func (pt Spec) GetUniqueField() string {
	return "spec.id"
}

func (pt Spec) GetUniqueValue() interface{} {
	return pt.ID
}

func validateHarnessConfig(raw json.RawMessage) error {
	const op = "agent.validateHarnessConfig"

	if len(raw) == 0 {
		return nil
	}

	var payload map[string]any
	err := json.Unmarshal(raw, &payload)
	if err != nil {
		return ez.New(op, ez.EINVALID, "harness_config must be a valid JSON object", err)
	}

	return nil
}

func CopyRawJSON(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	dst := make(json.RawMessage, len(src))
	copy(dst, src)

	return dst
}

func normalizeReasoningEffort(value runtimetypes.ReasoningEffort) runtimetypes.ReasoningEffort {
	if value == "" {
		return runtimetypes.ReasoningEffortMedium
	}

	return value
}
