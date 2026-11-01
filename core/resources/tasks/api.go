// Package tasks manages Tasks: one microsandbox VM each, created from a
// task.Spec, with status read live from the runtime
// (docs/decisions/0003-tasks-run-in-microsandbox-vms.md).
package tasks

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

const (
	// msgVsockPort is the guest port agc msg dials, the default of
	// `agc msg` in interfaces/cli/msg.go.
	msgVsockPort = 5100
	// statusRunning and statusStopped are the runtime's own words.
	statusRunning = "running"
	statusStopped = "stopped"
)

// Sandbox is a live handle on a Task's VM. Detach releases it and leaves
// the VM running.
type Sandbox interface {
	executor.TerminalExecutor
	Detach(ctx context.Context) error
}

// Runtime is the seam over package sandbox, so tests can fake it.
type Runtime interface {
	Create(ctx context.Context, spec sandbox.Spec) (Sandbox, error)
	Connect(ctx context.Context, name string) (Sandbox, error)
	Start(ctx context.Context, name string, secrets []sandbox.Secret) error
	Stop(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (string, error)
}

type API struct {
	db          bun.IDB
	msg         *messaging.Service
	rt          Runtime
	secretsPath string
	socketDir   string

	mu        sync.Mutex
	listeners map[string]*listener
}

// listener is one Task's messaging server and the socket it serves.
type listener struct {
	srv *http.Server
	ln  net.Listener
}

// NewAPI returns the tasks API. secretsPath is the secrets file
// (secrets.DefaultPath()), socketDir holds one messaging socket per Task.
func NewAPI(ctrl *controller.Controller, msg *messaging.Service, rt Runtime, secretsPath string, socketDir string) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return newAPI(ctrl.DB, msg, rt, secretsPath, socketDir)
}

func newAPI(db bun.IDB, msg *messaging.Service, rt Runtime, secretsPath string, socketDir string) *API {
	if msg == nil {
		panic("Messaging reference is nil")
	}
	if rt == nil {
		panic("Runtime reference is nil")
	}

	return &API{
		db:          db,
		msg:         msg,
		rt:          rt,
		secretsPath: secretsPath,
		socketDir:   socketDir,
		listeners:   map[string]*listener{},
	}
}

// TaskView is a Task with its live status. Status is the runtime's word,
// verbatim, or "unknown" with StatusError when the runtime cannot say.
type TaskView struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Spec        task.Spec `json:"spec"`
	Status      string    `json:"status"`
	StatusError string    `json:"status_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

func decodeSpec(record *taskmodels.Task) (task.Spec, error) {
	var spec task.Spec

	err := json.Unmarshal(record.Spec, &spec)
	if err != nil {
		return spec, ez.New(ez.EINTERNAL, "stored spec of task "+record.Name+" is unreadable", err)
	}

	return spec, nil
}

// view reads the live status. A running task gets its messaging listener
// if no agc process serves it yet, which covers tasks created or started
// by a CLI process that has since exited.
func (api *API) view(ctx context.Context, record *taskmodels.Task) (*TaskView, error) {
	spec, err := decodeSpec(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view := &TaskView{
		ID:        record.ID,
		Name:      record.Name,
		Spec:      spec,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}

	status, err := api.rt.Status(ctx, record.Name)
	if err != nil {
		view.Status = "unknown"
		view.StatusError = ez.ErrorMessage(err)
		return view, nil
	}
	view.Status = status

	if status == statusRunning {
		err = api.startTaskListener(record.Name, spec)
		if err != nil {
			log.Warn().Err(err).Str("task", record.Name).Msg("cannot start task messaging listener")
		}
	}

	return view, nil
}

// resolveSecrets reads the secrets file only when the spec asks for
// secrets. Its errors name secrets, never values, and pass through as is.
func (api *API) resolveSecrets(spec task.Spec) ([]secrets.Binding, error) {
	requests, err := spec.SecretRequests()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(requests) == 0 {
		return nil, nil
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	bindings, err := store.Resolve(requests)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return bindings, nil
}

// Executor connects to a running Task, for terminals and workflow steps.
func (api *API) Executor(ctx context.Context, name string) (Sandbox, error) {
	sb, err := api.rt.Connect(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sb, nil
}
