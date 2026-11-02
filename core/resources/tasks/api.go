// Package tasks manages Tasks: orders for work, in words, that allocate
// compute as sandboxes (docs/decisions/0008-tasks-allocate-compute.md).
package tasks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// A task's status is computed, never stored (ADR 0008 item 2).
const (
	StatusOpen    = "open"
	StatusRunning = "running"
	StatusDone    = "done"
)

type API struct {
	db bun.IDB
	// Sandboxes is the compute tasks allocate.
	Sandboxes   *sandboxes.API
	msg         *messaging.Service
	defaults    controller.DefaultsSettings
	secretsPath string
	specs       task.Store
}

// NewAPI returns the tasks API. msg carries the task channels, the
// controller's defaults fill in assignee, model and owner, secretsPath is
// the secrets file (secrets.DefaultPath()), specs keeps the saved specs.
func NewAPI(ctrl *controller.Controller, sandboxesAPI *sandboxes.API, msg *messaging.Service, secretsPath string, specs task.Store) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(ctrl.DB, sandboxesAPI, msg, ctrl.Defaults(), secretsPath, specs)
}

// New is NewAPI over a bare database, for tests. It hooks /done into msg.
func New(db bun.IDB, sandboxesAPI *sandboxes.API, msg *messaging.Service, defaults controller.DefaultsSettings, secretsPath string, specs task.Store) *API {
	if sandboxesAPI == nil {
		panic("Sandboxes reference is nil")
	}
	if msg == nil {
		panic("Messaging reference is nil")
	}

	api := &API{
		db:          db,
		Sandboxes:   sandboxesAPI,
		msg:         msg,
		defaults:    defaults,
		secretsPath: secretsPath,
		specs:       specs,
	}

	msg.OnPost(api.closeOnDone)

	return api
}

// Defaults are the configured defaults Create fills in.
func (api *API) Defaults() controller.DefaultsSettings {
	return api.defaults
}

// TaskView is a Task with its computed status and its sandboxes, newest
// first.
type TaskView struct {
	ID        uuid.UUID               `json:"id"`
	Title     string                  `json:"title"`
	Name      string                  `json:"name"`
	Brief     string                  `json:"brief"`
	Assignee  string                  `json:"assignee"`
	Model     string                  `json:"model"`
	Manager   string                  `json:"manager"`
	Owner     string                  `json:"owner"`
	For       string                  `json:"for"`
	Status    string                  `json:"status"`
	DoneAt    *time.Time              `json:"done_at"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
	Sandboxes []sandboxes.SandboxView `json:"sandboxes"`
}

// validateRef checks a request names a task by exactly one of id or name.
func validateRef(id uuid.UUID, name string) error {
	if id == uuid.Nil && name == "" {
		return ez.New(ez.EINVALID, "id or name is required", nil)
	}

	if id != uuid.Nil && name != "" {
		return ez.New(ez.EINVALID, "pass only one of id or name", nil)
	}

	return nil
}

func (api *API) find(ctx context.Context, id uuid.UUID, name string) (*taskmodels.Task, error) {
	if id != uuid.Nil {
		return taskmodels.GetTaskByID(ctx, api.db, id)
	}

	return taskmodels.GetTaskByName(ctx, api.db, name)
}

// view embeds the sandboxes and computes the status: done when done_at is
// set, otherwise running when a sandbox is running, otherwise open.
func (api *API) view(ctx context.Context, record *taskmodels.Task) (*TaskView, error) {
	views, err := api.Sandboxes.TaskSandboxes(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return newView(record, views), nil
}

func newView(record *taskmodels.Task, views []sandboxes.SandboxView) *TaskView {
	status := StatusOpen
	for _, view := range views {
		if view.Status == sandboxes.StatusRunning {
			status = StatusRunning
		}
	}

	if record.DoneAt != nil {
		status = StatusDone
	}

	return &TaskView{
		ID:        record.ID,
		Title:     record.Title,
		Name:      record.Name,
		Brief:     record.Brief,
		Assignee:  record.Assignee,
		Model:     record.Model,
		Manager:   record.Manager,
		Owner:     record.Owner,
		For:       record.For,
		Status:    status,
		DoneAt:    record.DoneAt,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Sandboxes: views,
	}
}
