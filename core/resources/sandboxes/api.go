// Package sandboxes manages Sandboxes: the compute a Task is allocated,
// one microsandbox VM each, booted from a task.Spec, with status read live
// from the runtime (docs/decisions/0003-tasks-run-in-microsandbox-vms.md,
// docs/decisions/0008-tasks-allocate-compute.md).
package sandboxes

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
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

const (
	// msgVsockPort is the guest port agc msg dials, the default of
	// `agc msg` in interfaces/cli/msg.go.
	msgVsockPort = 5100
	// bootTimeout bounds creating, cloning and setting up a sandbox.
	bootTimeout = 30 * time.Minute

	// StatusStarting and StatusFailed are agc's words for a sandbox that
	// has not booted yet or failed to. Every other status is the runtime's
	// word, verbatim.
	StatusStarting = "starting"
	StatusFailed   = "failed"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusUnknown  = "unknown"
	// StatusDeleted is only ever sent in sandbox.status, for a sandbox
	// that is gone.
	StatusDeleted = "deleted"
)

// Sandbox is a live handle on a VM. Detach releases it and leaves the VM
// running.
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
	// volumeRoot is where msb keeps named volumes on this host
	// (VolumeRoot), read for transcripts.
	volumeRoot string
	// rootCtx is what boots derive from, so they outlive the request that
	// allocated them and end with the process.
	rootCtx context.Context

	mu        sync.Mutex
	listeners map[string]*listener
	booting   map[string]*boot
	// deliver serves agc task deliver on the sandbox sockets; nil until
	// the tasks API sets it with OnDeliver.
	deliver DeliverFunc
	// homes serves the agent routes on agent homes' sockets; nil until
	// the agents API sets it with SetHomes.
	homes Homes
	// onStatus runs after every sandbox.status publication.
	onStatus func(view SandboxView)

	// stopping serializes stopOnce.
	stopping sync.Mutex
	// naming serializes picking a sandbox name and inserting it.
	naming sync.Mutex
	// boots counts allocations still booting.
	boots sync.WaitGroup

	// events gets sandbox.status on every status change, nil drops them.
	events *events.Bus
}

// StatusEvent is the data of sandbox.status.
type StatusEvent struct {
	Sandbox SandboxView `json:"sandbox"`
}

// SetEvents publishes sandbox.status on bus from now on. Call it before
// the API is used.
func (api *API) SetEvents(bus *events.Bus) {
	api.events = bus
}

// publish sends view as a sandbox.status event.
func (api *API) publish(view *SandboxView) {
	if view == nil {
		return
	}

	api.events.Publish(events.Event{Type: events.SandboxStatus, Data: StatusEvent{Sandbox: *view}})

	api.mu.Lock()
	onStatus := api.onStatus
	api.mu.Unlock()

	if onStatus != nil {
		onStatus(*view)
	}
}

// OnStatus sets what runs after every sandbox.status publication, with the
// view sent; the agents API uses it to publish agent.state for homes.
func (api *API) OnStatus(fn func(view SandboxView)) {
	api.mu.Lock()
	api.onStatus = fn
	api.mu.Unlock()
}

// publishCurrent reads the sandbox's view and sends it, for status changes
// that happen outside a request (a boot finishing or failing).
func (api *API) publishCurrent(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) {
	if api.events == nil {
		return
	}

	view, err := api.view(ctx, record, owner)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot read the sandbox for sandbox.status")
		return
	}

	api.publish(view)
}

// listener is one sandbox's messaging server and the socket it serves.
type listener struct {
	srv *http.Server
	ln  net.Listener
}

// NewAPI returns the sandboxes API. rootCtx bounds background boots,
// secretsPath is the secrets file (secrets.DefaultPath()), socketDir holds
// one messaging socket per sandbox.
func NewAPI(rootCtx context.Context, ctrl *controller.Controller, msg *messaging.Service, rt Runtime, secretsPath string, socketDir string) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	return New(rootCtx, ctrl.DB, msg, rt, secretsPath, socketDir)
}

// New is NewAPI over a bare database, for tests.
func New(rootCtx context.Context, db bun.IDB, msg *messaging.Service, rt Runtime, secretsPath string, socketDir string) *API {
	if msg == nil {
		panic("Messaging reference is nil")
	}
	if rt == nil {
		panic("Runtime reference is nil")
	}
	if rootCtx == nil {
		rootCtx = context.Background()
	}

	return &API{
		db:          db,
		msg:         msg,
		rt:          rt,
		secretsPath: secretsPath,
		socketDir:   socketDir,
		volumeRoot:  VolumeRoot(),
		rootCtx:     rootCtx,
		listeners:   map[string]*listener{},
		booting:     map[string]*boot{},
	}
}

// SandboxView is a Sandbox with its status. Status is starting or failed
// from the row, otherwise the runtime's word verbatim, or unknown with
// StatusError when the runtime cannot say.
type SandboxView struct {
	ID     uuid.UUID `json:"id"`
	TaskID uuid.UUID `json:"task_id"`
	// Task is the task's name, empty for an agent's home.
	Task string `json:"task"`
	// Agent is the name of the persistent agent whose home this is
	// (ADR 0016), empty for a task's sandbox.
	Agent   string    `json:"agent"`
	Name    string    `json:"name"`
	Spec    task.Spec `json:"spec"`
	Command string    `json:"command"`
	// Instructions is the TASK.md written into the sandbox, verbatim.
	Instructions string `json:"instructions"`
	// Volume is the named volume holding the harness homes and their
	// transcripts, kept after the sandbox is deleted. Empty for sandboxes
	// booted before volumes.
	Volume      string     `json:"volume"`
	Status      string     `json:"status"`
	StatusError string     `json:"status_error,omitempty"`
	BootedAt    *time.Time `json:"booted_at"`
	StoppedAt   *time.Time `json:"stopped_at"`
	CreatedAt   time.Time  `json:"created_at"`
	// Work is the clone's unpushed work, set only by Get and by List with
	// Work, when the sandbox is running and has a repo; null otherwise
	// and when the check fails.
	Work *Work `json:"work"`
}

// validateRef checks a request names a sandbox by exactly one of id or
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

func (api *API) find(ctx context.Context, id uuid.UUID, name string) (*taskmodels.Sandbox, error) {
	if id != uuid.Nil {
		return taskmodels.GetSandboxByID(ctx, api.db, id)
	}

	return taskmodels.GetSandboxByName(ctx, api.db, name)
}

// findWithTask returns the sandbox and the task it is allocated to.
func (api *API) findWithTask(ctx context.Context, id uuid.UUID, name string) (*taskmodels.Sandbox, *taskmodels.Task, error) {
	record, err := api.find(ctx, id, name)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	owner, err := api.ownerOf(ctx, record)
	if err != nil {
		return nil, nil, ez.Wrap(err)
	}

	return record, owner, nil
}

// ownerOf returns the task the sandbox is allocated to, or an empty task
// for an agent's home, which has none (ADR 0016).
func (api *API) ownerOf(ctx context.Context, record *taskmodels.Sandbox) (*taskmodels.Task, error) {
	if record.TaskID == uuid.Nil {
		return &taskmodels.Task{}, nil
	}

	owner, err := taskmodels.GetTaskByID(ctx, api.db, record.TaskID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return owner, nil
}

func decodeSpec(record *taskmodels.Sandbox) (task.Spec, error) {
	var spec task.Spec

	err := json.Unmarshal(record.Spec, &spec)
	if err != nil {
		return spec, ez.New(ez.EINTERNAL, "stored spec of sandbox "+record.Name+" is unreadable", err)
	}

	return spec, nil
}

// baseView is the sandbox's row as a view, without a status.
func baseView(record *taskmodels.Sandbox, owner *taskmodels.Task) (*SandboxView, error) {
	spec, err := decodeSpec(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	view := &SandboxView{
		ID:           record.ID,
		TaskID:       record.TaskID,
		Task:         owner.Name,
		Agent:        record.Agent,
		Name:         record.Name,
		Spec:         spec,
		Command:      record.Command,
		Instructions: record.Instructions,
		Volume:       record.Volume,
		BootedAt:     record.BootedAt,
		StoppedAt:    record.StoppedAt,
		CreatedAt:    record.CreatedAt,
	}

	return view, nil
}

// view reads the status. A running sandbox gets its messaging listener if
// no agc process serves it yet, which covers sandboxes booted or started by
// a CLI process that has since exited.
func (api *API) view(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task) (*SandboxView, error) {
	view, err := baseView(record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	spec := view.Spec

	if record.BootError != nil {
		view.Status = StatusFailed
		view.StatusError = *record.BootError
		return view, nil
	}

	if record.BootedAt == nil && time.Since(record.CreatedAt) > bootTimeout+time.Minute {
		// No boot outlives bootTimeout, so the process booting it is gone.
		msg := "boot did not finish within " + bootTimeout.String() + "; agc stopped while it was booting"
		record.BootError = &msg

		err = record.Update(ctx, api.db, "boot_error")
		if err != nil {
			return nil, ez.Wrap(err)
		}

		view.Status = StatusFailed
		view.StatusError = msg
		api.publish(view)
		return view, nil
	}

	if record.BootedAt == nil {
		view.Status = StatusStarting
		return view, nil
	}

	status, err := api.rt.Status(ctx, record.Name)
	if err != nil {
		view.Status = StatusUnknown
		view.StatusError = ez.ErrorMessage(err)
		return view, nil
	}
	view.Status = status

	if status == StatusRunning {
		err = api.startSandboxListener(record, owner, spec)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot start sandbox messaging listener")
		}
	}

	return view, nil
}

// TaskSandboxes returns the views of a task's sandboxes, newest first.
func (api *API) TaskSandboxes(ctx context.Context, owner *taskmodels.Task) ([]SandboxView, error) {
	records, err := taskmodels.ListTaskSandboxes(ctx, api.db, owner.ID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	views := []SandboxView{}
	for i := range records {
		view, err := api.view(ctx, &records[i], owner)
		if err != nil {
			return nil, ez.Wrap(err)
		}
		views = append(views, *view)
	}

	return views, nil
}

// resolveSecrets binds the provider default account for harness when the
// spec names no credential of it (withDefaultAccount), then resolves the
// spec's secrets, reading the secrets file only when there are any. It
// returns the spec with the default bound, to store and boot from. Its
// errors name secrets, never values, and pass through as is.
func (api *API) resolveSecrets(ctx context.Context, spec task.Spec, harness agent.Harness) (task.Spec, []secrets.Binding, error) {
	spec, err := api.withDefaultAccount(ctx, spec, harness)
	if err != nil {
		return spec, nil, ez.Wrap(err)
	}

	requests, err := spec.SecretRequests()
	if err != nil {
		return spec, nil, ez.Wrap(err)
	}

	if len(requests) == 0 {
		return spec, nil, nil
	}

	store, err := secrets.Load(api.secretsPath)
	if err != nil {
		return spec, nil, ez.Wrap(err)
	}

	bindings, err := store.Resolve(requests)
	if err != nil {
		return spec, nil, ez.Wrap(err)
	}

	return spec, bindings, nil
}

// Executor connects to a running sandbox, for terminals and workflow steps.
func (api *API) Executor(ctx context.Context, name string) (Sandbox, error) {
	sb, err := api.rt.Connect(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sb, nil
}

// Wait blocks until every boot this API started has finished, for a CLI
// process that must not exit mid-boot.
func (api *API) Wait() {
	api.boots.Wait()
}
