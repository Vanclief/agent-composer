package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

const maxTitleLength = 120

var (
	nameRegex    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	nonSlugRegex = regexp.MustCompile(`[^a-z0-9]+`)
)

// Task is an order: work to be done, in words. It is allocated compute as
// Sandboxes. Its status is computed, never stored (ADR 0008).
type Task struct {
	bun.BaseModel `bun:"table:tasks"`

	ID uuid.UUID `bun:",pk,type:uuid" json:"id"`
	// Title is what a human calls the Task.
	Title string `bun:"title,notnull,default:''" json:"title"`
	// Name is a slug, also the channel and the first sandbox's name.
	Name string `bun:"name,notnull,unique" json:"name"`
	// Brief says what to do and what done looks like, in markdown.
	Brief string `bun:"brief,notnull,default:''" json:"brief"`
	// Assignee is a harness id or a person's name. Empty is unassigned.
	Assignee string `bun:"assignee,notnull,default:''" json:"assignee"`
	// For is the id of the knowledge action or decision the Task serves.
	For       string     `bun:"for_id,notnull,default:''" json:"for"`
	DoneAt    *time.Time `bun:"done_at" json:"done_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		msg := fmt.Sprintf("task name %q is invalid: use 1-63 lowercase letters, digits or -, starting with a letter", name)
		return ez.New(ez.EINVALID, msg, nil)
	}

	// REST paths take an id or a name, so a name must never read as an id.
	_, err := uuid.Parse(name)
	if err == nil {
		return ez.New(ez.EINVALID, fmt.Sprintf("task name %q looks like a UUID: pick another name", name), nil)
	}

	return nil
}

// Slug turns a title into a task name: "Review facebook ads" becomes
// "review-facebook-ads".
func Slug(title string) (string, error) {
	slug := nonSlugRegex.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")

	if slug != "" && slug[0] >= '0' && slug[0] <= '9' {
		slug = "task-" + slug
	}

	if len(slug) > 63 {
		slug = strings.TrimRight(slug[:63], "-")
	}

	if slug == "" {
		return "", ez.New(ez.EINVALID, fmt.Sprintf("title %q has no letters or digits to make a name from", title), nil)
	}

	return slug, nil
}

func ValidateTitle(title string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ez.New(ez.EINVALID, "task title is required", nil)
	}

	if len([]rune(trimmed)) > maxTitleLength {
		return ez.New(ez.EINVALID, fmt.Sprintf("task title is longer than %d characters", maxTitleLength), nil)
	}

	return nil
}

func (t *Task) Validate() error {
	if t == nil {
		return ez.New(ez.EINVALID, "task is nil", nil)
	}

	err := ValidateName(t.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	err = ValidateTitle(t.Title)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (t *Task) Insert(ctx context.Context, db bun.IDB) error {
	if t.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		t.ID = id
	}

	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	err := t.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(t).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// MarkDone sets DoneAt to now. A Task already done keeps its first DoneAt.
func (t *Task) MarkDone(ctx context.Context, db bun.IDB) error {
	if t.DoneAt != nil {
		return nil
	}

	now := time.Now().UTC()
	t.DoneAt = &now
	t.UpdatedAt = now

	_, err := db.NewUpdate().Model(t).Column("done_at", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (t *Task) Delete(ctx context.Context, db bun.IDB) error {
	if t.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	_, err := db.NewDelete().Model(t).WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func GetTaskByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Task, error) {
	record := new(Task)

	err := db.NewSelect().Model(record).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("task with ID %s not found", id), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

func GetTaskByName(ctx context.Context, db bun.IDB, name string) (*Task, error) {
	record := new(Task)

	err := db.NewSelect().Model(record).Where("name = ?", name).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("task %s not found", name), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// ListTasks returns every task, newest first.
func ListTasks(ctx context.Context, db bun.IDB) ([]Task, error) {
	tasks := []Task{}

	err := db.NewSelect().Model(&tasks).Order("created_at DESC", "id DESC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return tasks, nil
}
