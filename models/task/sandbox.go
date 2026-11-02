package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// Sandbox is the compute a Task is allocated: one microsandbox VM. Its
// status is read live from the runtime once booted, never stored
// (ADR 0008).
type Sandbox struct {
	bun.BaseModel `bun:"table:sandboxes"`

	ID     uuid.UUID `bun:",pk,type:uuid" json:"id"`
	TaskID uuid.UUID `bun:"task_id,type:uuid,notnull" json:"task_id"`
	// Name is the runtime's sandbox name and the socket file.
	Name string `bun:"name,notnull,unique" json:"name"`
	// Spec is the environment block the sandbox booted from.
	Spec json.RawMessage `bun:"type:jsonb" json:"spec"`
	// Command is started as the first terminal once booted. Empty means none.
	Command   string     `bun:"command,notnull,default:''" json:"command"`
	BootError *string    `bun:"boot_error" json:"boot_error"`
	BootedAt  *time.Time `bun:"booted_at" json:"booted_at"`
	StoppedAt *time.Time `bun:"stopped_at" json:"stopped_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *Sandbox) Validate() error {
	if s == nil {
		return ez.New(ez.EINVALID, "sandbox is nil", nil)
	}

	if s.TaskID == uuid.Nil {
		return ez.New(ez.EINVALID, "sandbox task id is required", nil)
	}

	err := ValidateName(s.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	if len(s.Spec) == 0 {
		return ez.New(ez.EINVALID, "sandbox spec is required", nil)
	}

	return nil
}

func (s *Sandbox) Insert(ctx context.Context, db bun.IDB) error {
	if s.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		s.ID = id
	}

	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now

	err := s.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(s).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Update writes columns and updated_at.
func (s *Sandbox) Update(ctx context.Context, db bun.IDB, columns ...string) error {
	s.UpdatedAt = time.Now().UTC()

	_, err := db.NewUpdate().Model(s).Column(append(columns, "updated_at")...).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (s *Sandbox) Delete(ctx context.Context, db bun.IDB) error {
	if s.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(s).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func GetSandboxByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Sandbox, error) {
	record := new(Sandbox)

	err := db.NewSelect().Model(record).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("sandbox with ID %s not found", id), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

func GetSandboxByName(ctx context.Context, db bun.IDB, name string) (*Sandbox, error) {
	record := new(Sandbox)

	err := db.NewSelect().Model(record).Where("name = ?", name).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("sandbox %s not found", name), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// SandboxNameTaken reports whether a sandbox called name exists.
func SandboxNameTaken(ctx context.Context, db bun.IDB, name string) (bool, error) {
	exists, err := db.NewSelect().Model((*Sandbox)(nil)).Where("name = ?", name).Exists(ctx)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return exists, nil
}

// ListSandboxes returns every sandbox, newest first.
func ListSandboxes(ctx context.Context, db bun.IDB) ([]Sandbox, error) {
	sandboxes := []Sandbox{}

	err := db.NewSelect().Model(&sandboxes).Order("created_at DESC", "id DESC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sandboxes, nil
}

// ListTaskSandboxes returns the sandboxes of a task, newest first.
func ListTaskSandboxes(ctx context.Context, db bun.IDB, taskID uuid.UUID) ([]Sandbox, error) {
	sandboxes := []Sandbox{}

	err := db.NewSelect().Model(&sandboxes).Where("task_id = ?", taskID).Order("created_at DESC", "id DESC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sandboxes, nil
}
