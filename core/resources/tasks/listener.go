package tasks

import (
	"context"
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
	"github.com/vanclief/agent-composer/task"
)

// Each Task has its own unix socket. The sandbox connects guest vsock port
// msgVsockPort to it, so every request on it comes from that Task
// (docs/decisions/0005-messaging-transport-and-identity.md).

func (api *API) socketPath(name string) string {
	return filepath.Join(api.socketDir, name+".sock")
}

func (api *API) startTaskListener(name string, spec task.Spec) error {
	grants, err := spec.Grants()
	if err != nil {
		return ez.Wrap(err)
	}

	return api.startListener(name, grants)
}

// startListener serves messaging for the task on its socket, unless this
// process already does, or another agc process does (the socket answers).
// A socket nobody answers is stale and replaced.
func (api *API) startListener(name string, grants []messaging.Grant) error {
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

	caller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: name, Grants: grants}, nil
	}
	srv := &http.Server{Handler: httpapi.New(api.msg, caller), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn().Err(err).Str("task", name).Msg("task messaging listener stopped")
		}
	}()

	api.listeners[name] = &listener{srv: srv, ln: ln}

	return nil
}

// stopListener stops this process's listener for the task, if any.
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
