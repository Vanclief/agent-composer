package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// SystemName is the participant agc itself posts as (ADR 0009). It is
// neither human nor in a task, and no one else may join with it.
const SystemName = "agc"

// Participant is a name agc handed out on join. The id is the identity,
// the name is only unique while the participant is active.
type Participant struct {
	bun.BaseModel `bun:"table:messaging_participants"`

	ID       string     `bun:"id,pk" json:"id"`
	Name     string     `bun:"name,notnull" json:"name"`
	Task     string     `bun:"task,notnull" json:"task"` // empty for humans
	Human    bool       `bun:"human,notnull" json:"human"`
	JoinedAt time.Time  `bun:"joined_at,notnull" json:"joined_at"`
	LeftAt   *time.Time `bun:"left_at,nullzero" json:"left_at,omitempty"`
	// ActiveName equals Name while active and NULL after leave. The
	// unique index on it rejects a name in use anywhere, even under
	// concurrent joins, while letting any number of past holders exist.
	ActiveName *string `bun:"active_name,unique" json:"-"`
}

func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		msg := fmt.Sprintf("name %q is invalid: use 1-32 lowercase letters, digits, _ or -, starting with a letter", name)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

func (p *Participant) Validate() error {
	if p == nil {
		return ez.New(ez.EINVALID, "participant is nil", nil)
	}

	err := ValidateName(p.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if !p.Human && p.Task == "" && p.Name != SystemName {
		return ez.New(ez.EINVALID, "participant must belong to a task or be human", nil)
	}

	return nil
}

// Insert creates an active participant with a fresh random id.
func (p *Participant) Insert(ctx context.Context, db bun.IDB) error {
	if p.ID == "" {
		buf := make([]byte, 16)
		_, err := rand.Read(buf)
		if err != nil {
			return ez.Wrap(err)
		}

		p.ID = hex.EncodeToString(buf)
	}

	if p.JoinedAt.IsZero() {
		p.JoinedAt = time.Now().UTC()
	}

	name := p.Name
	p.ActiveName = &name

	err := p.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(p).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Leave releases the name. The row stays so history keeps its sender.
func (p *Participant) Leave(ctx context.Context, db bun.IDB) error {
	now := time.Now().UTC()
	p.LeftAt = &now
	p.ActiveName = nil

	_, err := db.NewUpdate().Model(p).Column("left_at", "active_name").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
