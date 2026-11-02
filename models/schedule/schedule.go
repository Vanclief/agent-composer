// Package schedule holds Schedules: a task template run on an interval,
// optionally once per item a poll command finds, and the items each
// schedule has already seen (docs/decisions/0012-scheduled-tasks.md).
package schedule

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

	"github.com/vanclief/agent-composer/task"
)

// MinEvery is the shortest interval a schedule may run on.
const MinEvery = time.Minute

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Template is the task a schedule creates: the fields of a task.yaml and
// of POST /api/tasks. Title, Name, Brief and each Accept entry are Go
// text/templates. Name is optional.
type Template struct {
	Title    string        `yaml:"title" json:"title"`
	Name     string        `yaml:"name" json:"name"`
	Brief    string        `yaml:"brief" json:"brief"`
	Accept   []string      `yaml:"accept" json:"accept"`
	Assignee string        `yaml:"assignee" json:"assignee"`
	Model    string        `yaml:"model" json:"model"`
	Manager  string        `yaml:"manager" json:"manager"`
	Owner    string        `yaml:"owner" json:"owner"`
	For      string        `yaml:"for" json:"for"`
	Sandbox  *task.Sandbox `yaml:"sandbox" json:"sandbox"`
}

// Poll is the command a schedule runs on the host to find items, and the
// secrets set in its environment by name.
type Poll struct {
	Command string   `yaml:"command" json:"command"`
	Secrets []string `yaml:"secrets" json:"secrets"`
}

// Schedule runs Task every Every. Its next run is computed from LastRunAt,
// never stored.
type Schedule struct {
	bun.BaseModel `bun:"table:schedules"`

	ID   uuid.UUID `bun:",pk,type:uuid" json:"id"`
	Name string    `bun:"name,notnull,unique" json:"name"`
	// Every is the interval as written, a Go duration such as "2m".
	Every   string   `bun:"every,notnull" json:"every"`
	Enabled bool     `bun:"enabled,notnull" json:"enabled"`
	Task    Template `bun:"task,type:jsonb,notnull" json:"task"`
	Poll    *Poll    `bun:"poll,type:jsonb" json:"poll"`
	// LastRunAt is when the last tick started, failed ones included.
	LastRunAt *time.Time `bun:"last_run_at" json:"last_run_at"`
	// LastError is the last tick's error, empty when it succeeded.
	LastError string    `bun:"last_error,notnull,default:''" json:"last_error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Item is one key a schedule's poll found, and the task made for it.
// Without a poll, the key is the task's name.
type Item struct {
	bun.BaseModel `bun:"table:schedule_items"`

	ID         uuid.UUID `bun:",pk,type:uuid" json:"id"`
	ScheduleID uuid.UUID `bun:"schedule_id,type:uuid,notnull,unique:schedule_items_schedule_key" json:"schedule_id"`
	Key        string    `bun:"key,notnull,unique:schedule_items_schedule_key" json:"key"`
	TaskID     uuid.UUID `bun:"task_id,type:uuid,notnull" json:"task_id"`
	// Task is the created task's name, kept after the task is deleted.
	Task   string    `bun:"task,notnull" json:"task"`
	SeenAt time.Time `bun:"seen_at,notnull" json:"seen_at"`
}

// ValidateName applies the task name rules to a schedule name.
func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		msg := fmt.Sprintf("schedule name %q is invalid: use 1-63 lowercase letters, digits or -, starting with a letter", name)
		return ez.New(ez.EINVALID, msg, nil)
	}

	_, err := uuid.Parse(name)
	if err == nil {
		return ez.New(ez.EINVALID, fmt.Sprintf("schedule name %q looks like a UUID: pick another name", name), nil)
	}

	return nil
}

// ParseEvery reads an interval, at least MinEvery.
func ParseEvery(every string) (time.Duration, error) {
	duration, err := time.ParseDuration(every)
	if err != nil {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("every %q is not a duration such as 2m or 1h", every), err)
	}

	if duration < MinEvery {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("every %q is shorter than the minimum, %s", every, "1m"), nil)
	}

	return duration, nil
}

// NextRunAt is LastRunAt plus Every; a schedule that never ran is due
// since it was created.
func (s *Schedule) NextRunAt() time.Time {
	if s.LastRunAt == nil {
		return s.CreatedAt
	}

	every, err := ParseEvery(s.Every)
	if err != nil {
		every = MinEvery
	}

	return s.LastRunAt.Add(every)
}

func (s *Schedule) Validate() error {
	if s == nil {
		return ez.New(ez.EINVALID, "schedule is nil", nil)
	}

	err := ValidateName(s.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = ParseEvery(s.Every)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (s *Schedule) Insert(ctx context.Context, db bun.IDB) error {
	if s.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		s.ID = id
	}

	at := now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = at
	}
	s.UpdatedAt = at

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

// Update writes what PUT replaces: every, enabled, task and poll.
func (s *Schedule) Update(ctx context.Context, db bun.IDB) error {
	err := s.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	s.UpdatedAt = now()

	_, err = db.NewUpdate().Model(s).Column("every", "enabled", "task", "poll", "updated_at").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// MarkRun records a tick that started at and ended with lastError.
func (s *Schedule) MarkRun(ctx context.Context, db bun.IDB, at time.Time, lastError string) error {
	started := at.UTC().Truncate(time.Microsecond)
	s.LastRunAt = &started
	s.LastError = lastError

	_, err := db.NewUpdate().Model(s).Column("last_run_at", "last_error").WherePK().Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Delete removes the schedule and its items; the tasks it created stay.
func (s *Schedule) Delete(ctx context.Context, db bun.IDB) error {
	if s.ID == uuid.Nil {
		return ez.New(ez.EINVALID, "id is required", errors.New("nil uuid"))
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewDelete().Model((*Item)(nil)).Where("schedule_id = ?", s.ID).Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}

		_, err = tx.NewDelete().Model(s).WherePK().Exec(ctx)
		if err != nil {
			return ez.Wrap(err)
		}

		return nil
	})
}

func GetByID(ctx context.Context, db bun.IDB, id uuid.UUID) (*Schedule, error) {
	record := new(Schedule)

	err := db.NewSelect().Model(record).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("schedule with ID %s not found", id), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

func GetByName(ctx context.Context, db bun.IDB, name string) (*Schedule, error) {
	record := new(Schedule)

	err := db.NewSelect().Model(record).Where("name = ?", name).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("schedule %s not found", name), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return record, nil
}

// List returns every schedule by name.
func List(ctx context.Context, db bun.IDB) ([]Schedule, error) {
	records := []Schedule{}

	err := db.NewSelect().Model(&records).Order("name ASC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return records, nil
}

// SeenKeys returns which of keys the schedule has already seen.
func SeenKeys(ctx context.Context, db bun.IDB, scheduleID uuid.UUID, keys []string) (map[string]bool, error) {
	seen := map[string]bool{}
	if len(keys) == 0 {
		return seen, nil
	}

	found := []string{}
	err := db.NewSelect().Model((*Item)(nil)).Column("key").
		Where("schedule_id = ?", scheduleID).Where("key IN (?)", bun.In(keys)).Scan(ctx, &found)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for _, key := range found {
		seen[key] = true
	}

	return seen, nil
}

// Insert records the item, seen now.
func (i *Item) Insert(ctx context.Context, db bun.IDB) error {
	if i.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return ez.Wrap(err)
		}

		i.ID = id
	}

	if i.SeenAt.IsZero() {
		i.SeenAt = now()
	}

	_, err := db.NewInsert().Model(i).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// ListItems returns a schedule's items, newest first.
func ListItems(ctx context.Context, db bun.IDB, scheduleID uuid.UUID) ([]Item, error) {
	items := []Item{}

	err := db.NewSelect().Model(&items).Where("schedule_id = ?", scheduleID).Order("seen_at DESC", "id DESC").Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return items, nil
}

// now is the current time as the database keeps it, to the microsecond.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
