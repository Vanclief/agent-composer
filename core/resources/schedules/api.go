// Package schedules runs tasks on a schedule: every interval a schedule
// creates a task from its template, or, with a poll command, one task per
// new item the command lists (docs/decisions/0012-scheduled-tasks.md).
package schedules

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

const (
	// DefaultPollTimeout bounds one run of a poll command.
	DefaultPollTimeout = 60 * time.Second
	// CheckInterval is how often the scheduler looks for due schedules.
	CheckInterval = 30 * time.Second
)

// Runner runs a poll command on the host with env as its whole
// environment, and returns its stdout and stderr.
type Runner func(ctx context.Context, command string, env []string) ([]byte, []byte, error)

type API struct {
	db          bun.IDB
	tasks       *tasks.API
	secretsPath string
	events      *events.Bus

	// run, pollTimeout and now are replaced in tests.
	run         Runner
	pollTimeout time.Duration
	now         func() time.Time

	mu      sync.Mutex
	running map[uuid.UUID]bool
	ticks   sync.WaitGroup
}

// NewAPI returns the schedules API. Tasks are created through tasksAPI,
// exactly as POST /api/tasks creates them; secretsPath is the secrets file
// poll commands read their secrets from.
func NewAPI(ctrl *controller.Controller, tasksAPI *tasks.API, secretsPath string) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.DB, tasksAPI, secretsPath)
}

// New is NewAPI over a bare database, for tests.
func New(db bun.IDB, tasksAPI *tasks.API, secretsPath string) *API {
	if tasksAPI == nil {
		panic("Tasks reference is nil")
	}

	return &API{
		db:          db,
		tasks:       tasksAPI,
		secretsPath: secretsPath,
		run:         runShell,
		pollTimeout: DefaultPollTimeout,
		now:         time.Now,
		running:     map[uuid.UUID]bool{},
	}
}

// SetEvents publishes schedule.ran on bus after every tick from now on.
// Call it before the API is used.
func (api *API) SetEvents(bus *events.Bus) {
	api.events = bus
}

// RanEvent is the data of schedule.ran: the schedule after the tick and
// the tasks it created.
type RanEvent struct {
	Schedule ScheduleView     `json:"schedule"`
	Created  []tasks.TaskView `json:"created"`
}

// Wait returns once every tick started so far has ended.
func (api *API) Wait() {
	api.ticks.Wait()
}

// ScheduleView is a schedule with its computed next run.
type ScheduleView struct {
	ID      uuid.UUID               `json:"id"`
	Name    string                  `json:"name"`
	Every   string                  `json:"every"`
	Enabled bool                    `json:"enabled"`
	Task    schedulemodels.Template `json:"task"`
	Poll    *schedulemodels.Poll    `json:"poll"`
	// LastRunAt is when the last tick started, failed ones included.
	LastRunAt *time.Time `json:"last_run_at"`
	LastError string     `json:"last_error"`
	// NextRunAt is last_run_at plus every (created_at before the first
	// run), null while the schedule is disabled.
	NextRunAt *time.Time `json:"next_run_at"`
	// Running is true while a tick of this schedule runs.
	Running   bool      `json:"running"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (api *API) view(record *schedulemodels.Schedule) *ScheduleView {
	view := &ScheduleView{
		ID:        record.ID,
		Name:      record.Name,
		Every:     record.Every,
		Enabled:   record.Enabled,
		Task:      record.Task,
		Poll:      record.Poll,
		LastRunAt: record.LastRunAt,
		LastError: record.LastError,
		Running:   api.isRunning(record.ID),
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}

	if view.Task.Accept == nil {
		view.Task.Accept = []string{}
	}

	if record.Enabled {
		next := record.NextRunAt()
		view.NextRunAt = &next
	}

	return view
}

func (api *API) isRunning(id uuid.UUID) bool {
	api.mu.Lock()
	defer api.mu.Unlock()

	return api.running[id]
}

// validateRef checks a request names a schedule by exactly one of id or
// name.
func validateRef(id uuid.UUID, name string) error {
	if id == uuid.Nil && name == "" {
		return ez.New(ez.EINVALID, "id or name is required", nil)
	}

	if id != uuid.Nil && name != "" {
		return ez.New(ez.EINVALID, "pass only one of id or name", nil)
	}

	return nil
}

func (api *API) find(ctx context.Context, id uuid.UUID, name string) (*schedulemodels.Schedule, error) {
	if id != uuid.Nil {
		return schedulemodels.GetByID(ctx, api.db, id)
	}

	return schedulemodels.GetByName(ctx, api.db, name)
}
