package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// nameRegex is an agent's name: lowercase letters, digits and dashes, the
// way the panel checks it, and short enough to be a messaging name.
var nameRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// MaxNameBytes keeps an agent's name a valid messaging participant and
// sandbox name.
const MaxNameBytes = 32

// Agent is a persistent agent (ADR 0016): a row, not a process. It is
// born numbered from a role (reviewer-3) and renamed when kept. Its state
// is computed from the home sandbox, never stored.
type Agent struct {
	bun.BaseModel `bun:"table:agents"`

	ID uuid.UUID `bun:",pk,type:uuid" json:"id"`
	// Name is unique: <role>-<number> until kept, then whatever it was
	// named.
	Name string `bun:"name,notnull,unique" json:"name"`
	// Role is the template it runs, agents/<role>.yaml, as typed.
	Role string `bun:"role,notnull" json:"role"`
	// Number is the n of <series>-<n> it was born as, kept after a
	// rename.
	Number int `bun:"number,notnull" json:"number"`
	// Series is the lowercased role the number was drawn from. It never
	// changes, so a number is never handed out twice even when a kept
	// agent changes role.
	Series  string  `bun:"series,notnull" json:"series"`
	Kept    bool    `bun:"kept,notnull,default:false" json:"kept"`
	Harness Harness `bun:"harness,notnull" json:"harness"`
	Model   string  `bun:"model,notnull,default:''" json:"model"`
	// Effort is the harness's reasoning effort, empty for its default.
	Effort string `bun:"effort,notnull,default:''" json:"effort"`
	Owner  string `bun:"owner,notnull,default:''" json:"owner"`
	// Home is the home sandbox's name, empty until the first wake. It
	// keeps its name when the agent is renamed.
	Home string `bun:"home,notnull,default:''" json:"home"`
	// Idle is how long the agent waits for a message before it sleeps, a
	// Go duration from the template.
	Idle         string     `bun:"idle,notnull,default:''" json:"idle"`
	CreatedAt    time.Time  `bun:"created_at,notnull" json:"created_at"`
	UpdatedAt    time.Time  `bun:"updated_at,notnull" json:"updated_at"`
	LastActiveAt *time.Time `bun:"last_active_at" json:"last_active_at"`
}

// ValidateName checks an agent's name.
func ValidateName(name string) error {
	if len(name) > MaxNameBytes || !nameRegex.MatchString(name) {
		msg := fmt.Sprintf("name %q is invalid: use up to %d lowercase letters, digits and dashes", name, MaxNameBytes)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

func (a *Agent) Validate() error {
	if a == nil {
		return ez.New(ez.EINVALID, "agent is nil", nil)
	}

	err := ValidateName(a.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if a.Role == "" {
		return ez.New(ez.EINVALID, "agent role is required", nil)
	}

	err = a.Harness.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (a *Agent) Insert(ctx context.Context, db bun.IDB) error {
	if a.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		a.ID = id
	}

	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	err := a.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(a).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Update writes columns and updated_at.
func (a *Agent) Update(ctx context.Context, db bun.IDB, columns ...string) error {
	a.UpdatedAt = time.Now().UTC()

	_, err := db.NewUpdate().Model(a).Column(append(columns, "updated_at")...).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (a *Agent) Delete(ctx context.Context, db bun.IDB) error {
	if a.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(a).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// GetAgentByName returns the agent called name.
func GetAgentByName(ctx context.Context, db bun.IDB, name string) (*Agent, error) {
	return getAgent(ctx, db, "name = ?", name, "agent "+name+" not found")
}

// GetAgentByHome returns the agent whose home sandbox is called home.
func GetAgentByHome(ctx context.Context, db bun.IDB, home string) (*Agent, error) {
	if home == "" {
		return nil, ez.New(ez.ENOTFOUND, "no agent has an empty home", nil)
	}

	return getAgent(ctx, db, "home = ?", home, "no agent lives in sandbox "+home)
}

func getAgent(ctx context.Context, db bun.IDB, where string, value string, notFound string) (*Agent, error) {
	record := new(Agent)

	err := db.NewSelect().Model(record).Where(where, value).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, notFound, err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// ListAgents returns every agent, oldest first.
func ListAgents(ctx context.Context, db bun.IDB) ([]Agent, error) {
	agents := []Agent{}

	err := db.NewSelect().Model(&agents).Order("created_at ASC", "id ASC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return agents, nil
}

// MaxNumber is the highest number drawn from series, 0 when there is none.
func MaxNumber(ctx context.Context, db bun.IDB, series string) (int, error) {
	var max sql.NullInt64

	err := db.NewSelect().Model((*Agent)(nil)).ColumnExpr("MAX(number)").Where("series = ?", series).Scan(ctx, &max)
	if err != nil {
		return 0, ez.Wrap(err)
	}

	return int(max.Int64), nil
}
