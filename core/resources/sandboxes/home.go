package sandboxes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	"github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// AgentBriefPath is where agc writes a persistent agent's instructions
// (AGENT.md) in its home, the way a task's sandbox gets TASK.md (ADR 0016
// item 2).
const AgentBriefPath = task.HomeDir + "/AGENT.md"

// EnvAgent names the persistent agent in its home's environment. It is the
// agent's name at its first wake; the agent's current name is what GET
// /agent on the socket answers.
const EnvAgent = "AGC_AGENT"

// Homes answers for persistent agents on their homes' sockets. The agents
// API implements it and sets it with SetHomes; every method gets the home
// sandbox's name, which is how the socket knows its agent.
type Homes interface {
	// Caller is who the home speaks as, computed per request so the
	// agent's channels follow its tasks.
	Caller(ctx context.Context, home string) (messaging.Caller, error)
	// Agent answers GET /agent.
	Agent(ctx context.Context, home string) (any, error)
	// Sleep stops the home, for POST /agent/sleep.
	Sleep(ctx context.Context, home string) error
	// CreateTask creates a task managed by the agent from a create
	// request body.
	CreateTask(ctx context.Context, home string, body []byte) (any, error)
	// GetTask reads a task the agent manages.
	GetTask(ctx context.Context, home string, taskName string) (any, error)
	// AcceptTask accepts a task the agent manages.
	AcceptTask(ctx context.Context, home string, taskName string) (any, error)
	// RejectTask sends back a task the agent manages, with why.
	RejectTask(ctx context.Context, home string, taskName string, why string) (any, error)
}

// SetHomes sets what serves the agent routes on every home's socket.
func (api *API) SetHomes(homes Homes) {
	api.mu.Lock()
	api.homes = homes
	api.mu.Unlock()
}

func (api *API) currentHomes() (Homes, error) {
	api.mu.Lock()
	homes := api.homes
	api.mu.Unlock()

	if homes == nil {
		return nil, ez.New(ez.EUNAVAILABLE, "agents are not served here", nil)
	}

	return homes, nil
}

// HomeRequest allocates a persistent agent's home: a sandbox with no task
// (ADR 0016 item 4).
type HomeRequest struct {
	// Agent is the agent's name, which the sandbox is named after.
	Agent string
	// Harness is the agent's harness, whose provider's default account
	// is bound when the template names no credential of it (ADR 0018).
	Harness agent.Harness
	// Sandbox is the template's sandbox; its command is ignored for
	// Command.
	Sandbox task.Sandbox
	// Command is the job agent, the agent loop.
	Command string
	// Instructions are written as AGENT.md.
	Instructions string
}

// AllocateHome inserts the agent's home and boots it in the background, as
// AllocateFor does for a task. The sandbox is named after the agent (the
// first free name), has no task, and gets $AGC_AGENT. Its volume is the
// agent's, agc-<agent>, whatever the sandbox is called, and is reused when
// an earlier home left it: the harness sessions and the loop's
// agc-sessions.json and agc-cursor.json outlive every home (ADR 0016
// item 5).
func (api *API) AllocateHome(ctx context.Context, request HomeRequest) (*SandboxView, error) {
	err := request.Sandbox.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if request.Agent == "" {
		return nil, ez.New(ez.EINVALID, "a home needs its agent's name", nil)
	}

	spec := request.Sandbox.Spec
	env := map[string]string{}
	for k, v := range spec.Env {
		env[k] = v
	}
	env[EnvAgent] = request.Agent
	spec.Env = env

	spec, bindings, err := api.resolveSecrets(ctx, spec, request.Harness)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	encodedSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record := &taskmodels.Sandbox{Agent: request.Agent, Volume: task.VolumeName(request.Agent), Spec: encodedSpec, Command: strings.TrimSpace(request.Command), Instructions: request.Instructions}

	err = api.insert(ctx, request.Agent, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.startBoot(ctx, record, &taskmodels.Task{}, spec, bindings)
}

// RenameHomeAgent records the agent's new name on its home, so the
// sandbox's view links to it. The sandbox keeps its name.
func (api *API) RenameHomeAgent(ctx context.Context, home string, agent string) error {
	record, err := taskmodels.GetSandboxByName(ctx, api.db, home)
	if err != nil {
		return ez.Wrap(err)
	}

	record.Agent = agent

	err = record.Update(ctx, api.db, "agent")
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Home returns the view of the agent's home sandbox called name.
func (api *API) Home(ctx context.Context, name string) (*SandboxView, error) {
	record, err := taskmodels.GetSandboxByName(ctx, api.db, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record, &taskmodels.Task{})
}

// SocketDir is where the sandboxes' messaging sockets are, <name>.sock.
func (api *API) SocketDir() string {
	return api.socketDir
}

// briefPath is where the sandbox's instructions are written.
func briefPath(record *taskmodels.Sandbox) string {
	if record.Agent != "" {
		return AgentBriefPath
	}

	return BriefPath
}

// startHomeListener serves an agent's home: messaging as the agent, and
// the agent routes. agc task deliver has nothing to deliver in a home.
func (api *API) startHomeListener(home string) error {
	caller := func(r *http.Request) (messaging.Caller, error) {
		homes, err := api.currentHomes()
		if err != nil {
			return messaging.Caller{}, ez.Wrap(err)
		}

		return homes.Caller(r.Context(), home)
	}

	return api.startListener(home, caller, func(mux *http.ServeMux) {
		mux.HandleFunc("POST "+httpapi.DeliverPath, func(w http.ResponseWriter, r *http.Request) {
			httpapi.WriteError(w, ez.New(ez.EINVALID, "sandbox "+home+" is an agent's home and has no task to deliver; agents manage tasks, they do not deliver them", nil))
		})
		mux.HandleFunc("GET "+httpapi.AgentPath, api.serveAgent(home))
		mux.HandleFunc("POST "+httpapi.AgentSleepPath, api.serveSleep(home))
		mux.HandleFunc("POST "+httpapi.TasksPath, api.serveCreateTask(home))
		mux.HandleFunc("GET "+httpapi.TaskPath, api.serveTask(home, func(ctx context.Context, homes Homes, r *http.Request) (any, error) {
			return homes.GetTask(ctx, home, r.PathValue("name"))
		}))
		mux.HandleFunc("POST "+httpapi.TaskAcceptPath, api.serveTask(home, func(ctx context.Context, homes Homes, r *http.Request) (any, error) {
			return homes.AcceptTask(ctx, home, r.PathValue("name"))
		}))
		mux.HandleFunc("POST "+httpapi.TaskRejectPath, func(w http.ResponseWriter, r *http.Request) {
			var body httpapi.RejectBody

			err := httpapi.DecodeBody(w, r, &body)
			if err != nil {
				httpapi.WriteError(w, err)
				return
			}

			api.serveTask(home, func(ctx context.Context, homes Homes, r *http.Request) (any, error) {
				return homes.RejectTask(ctx, home, r.PathValue("name"), body.Why)
			})(w, r)
		})
	})
}

func (api *API) serveAgent(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		homes, err := api.currentHomes()
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		view, err := homes.Agent(r.Context(), home)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// serveSleep answers 204 first and stops the home after: the request comes
// from the VM being stopped, and stopping the listener waits for the
// requests it serves.
func (api *API) serveSleep(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		homes, err := api.currentHomes()
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		// Counted before answering, so a Wait after the answer waits for
		// the stop.
		api.boots.Add(1)

		w.WriteHeader(http.StatusNoContent)

		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}

		ctx := context.WithoutCancel(r.Context())

		go func() {
			defer api.boots.Done()

			err := homes.Sleep(ctx, home)
			if err != nil {
				log.Warn().Err(err).Str("sandbox", home).Msg("the agent asked to sleep and its home could not be stopped")
			}
		}()
	}
}

func (api *API) serveCreateTask(home string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		homes, err := api.currentHomes()
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			httpapi.WriteError(w, ez.New(ez.EINVALID, "request body is too large or unreadable", err))
			return
		}

		view, err := homes.CreateTask(r.Context(), home, body)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusCreated, view)
	}
}

// serveTask answers one of the task routes of a home with fn's view.
func (api *API) serveTask(home string, fn func(ctx context.Context, homes Homes, r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		homes, err := api.currentHomes()
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		view, err := fn(r.Context(), homes, r)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}
