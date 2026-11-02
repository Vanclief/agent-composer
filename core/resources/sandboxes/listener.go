package sandboxes

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// Each sandbox has its own unix socket, named after it. The VM connects
// guest vsock port msgVsockPort to it, so every request on it comes from
// that sandbox, speaking for its task: the caller's Task is the task name,
// so the sandbox posts in #<task> (ADR 0005, ADR 0008). An agent's home
// speaks for its agent instead (home.go, ADR 0016).

func (api *API) socketPath(name string) string {
	return filepath.Join(api.socketDir, name+".sock")
}

// startSandboxListener serves the sandbox's socket with socketHandler. A
// sandbox on a peer gets no listener here: the peer serves its socket and
// forwards every request to ServeForwarded (ADR 0018).
func (api *API) startSandboxListener(record *taskmodels.Sandbox, owner *taskmodels.Task, spec task.Spec) error {
	if record.Server != "" {
		return nil
	}

	handler, err := api.socketHandler(record, owner, spec)
	if err != nil {
		return ez.Wrap(err)
	}

	return api.startListener(record.Name, handler)
}

// socketHandler is what the sandbox's socket answers: messaging as its
// task plus agc task deliver, or, for an agent's home, as that agent
// (homeHandler).
func (api *API) socketHandler(record *taskmodels.Sandbox, owner *taskmodels.Task, spec task.Spec) (http.Handler, error) {
	if record.Agent != "" {
		return api.homeHandler(record.Name), nil
	}

	grants, err := Grants(owner, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	caller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: owner.Name, Grants: grants}, nil
	}

	mux := http.NewServeMux()
	mux.Handle("/", httpapi.New(api.msg, caller))
	mux.HandleFunc("POST "+httpapi.DeliverPath, api.serveDeliver(owner.Name))

	return mux, nil
}

// Grants are the channels a sandbox of owner may use besides #<task>: the
// spec's, and the task owner's inbox #<owner> read-write when the task has
// an owner (ADR 0009 item 5).
func Grants(owner *taskmodels.Task, spec task.Spec) ([]messaging.Grant, error) {
	grants, err := spec.Grants()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if owner.Owner == "" {
		return grants, nil
	}

	inbox := "#" + owner.Owner
	for i := range grants {
		if grants[i].Channel == inbox {
			grants[i].ReadOnly = false
			return grants, nil
		}
	}

	return append(grants, messaging.Grant{Channel: inbox}), nil
}

// startListener serves handler on the socket of the sandbox called name,
// unless this process already does, or another agc process does (the
// socket answers). A socket nobody answers is stale and replaced.
func (api *API) startListener(name string, handler http.Handler) error {
	api.mu.Lock()
	defer api.mu.Unlock()

	_, ok := api.listeners[name]
	if ok {
		return nil
	}

	path := api.socketPath(name)

	conn, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		_ = conn.Close()
		return nil
	}

	err = os.MkdirAll(api.socketDir, 0o700)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot create socket directory "+api.socketDir, err)
	}

	err = os.Remove(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ez.New(ez.EINTERNAL, "cannot remove stale socket "+path, err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot listen on "+path, err)
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn().Err(err).Str("sandbox", name).Msg("sandbox messaging listener stopped")
		}
	}()

	api.listeners[name] = &listener{srv: srv, ln: ln}

	return nil
}

// DeliverFunc delivers the task called taskName with summary and report,
// the request's report JSON as sent (nil when absent), for agc task
// deliver inside one of its sandboxes (ADR 0010 item 2, "Delivery
// report"), and returns the task's view.
type DeliverFunc func(ctx context.Context, taskName string, summary string, report json.RawMessage) (any, error)

// OnDeliver sets what agc task deliver calls on every sandbox's socket.
// The socket speaks for its own task only, so a sandbox delivers no other.
func (api *API) OnDeliver(deliver DeliverFunc) {
	api.mu.Lock()
	api.deliver = deliver
	api.mu.Unlock()
}

func (api *API) serveDeliver(taskName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		deliver := api.deliver
		api.mu.Unlock()

		if deliver == nil {
			httpapi.WriteError(w, ez.New(ez.EUNAVAILABLE, "agc task deliver is not served here", nil))
			return
		}

		var req httpapi.DeliverRequest

		err := httpapi.DecodeBody(w, r, &req)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		view, err := deliver(r.Context(), taskName, req.Summary, req.Report)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, view)
	}
}

// stopListener stops this process's listener for the sandbox, if any.
// Closing the listener removes its socket file. It is closed here as well
// as by Serve, so the file is gone when this returns even if Serve has not
// started yet, and a later listener on the same path is never unlinked.
func (api *API) stopListener(name string) {
	api.mu.Lock()
	l := api.listeners[name]
	delete(api.listeners, name)
	api.mu.Unlock()

	if l == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := l.srv.Shutdown(ctx)
	if err != nil {
		_ = l.srv.Close()
	}

	_ = l.ln.Close()
}
