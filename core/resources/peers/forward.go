package peers

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	"github.com/vanclief/agent-composer/task"
)

// SocketsPath prefixes the primary's route for forwarded sockets:
// SocketsPath/<sandbox>/<socket path>.
const SocketsPath = "/api/peer/sockets/"

// A VM a peer runs for its primary has its vsock socket on the peer, like
// any sandbox. The peer serves nothing on it itself: every request, of
// every route (messaging, agc task deliver, the agent routes of a home,
// and whatever a later branch adds), is passed to the primary as is, at
// SocketsPath/<sandbox>/..., with the peer token. The path names the
// sandbox, which is how the peer vouches for who spoke; the primary works
// out who that sandbox speaks as from its own rows (ADR 0018).

// forward is one VM's socket and the server forwarding it.
type forward struct {
	srv *http.Server
	ln  net.Listener
}

func (api *API) socketPath(name string) string {
	return filepath.Join(api.socketDir, name+".sock")
}

// forwardHandler passes every request to the primary as the sandbox
// called name.
func (api *API) forwardHandler(name string) http.Handler {
	target, _ := url.Parse(api.primary)

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			pr.Out.URL.Path = target.Path + SocketsPath + url.PathEscape(name) + pr.In.URL.Path
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
		},
		Transport: api.proxy,
		// GET /wait answers when a message arrives; pass it on at once.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			httpapi.WriteError(w, ez.New(ez.EUNAVAILABLE, "cannot reach the primary agc at "+api.primary+": "+err.Error(), err))
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := loadToken(api.secretsPath)
		if err == nil && token == "" {
			err = ez.New(ez.EUNAVAILABLE, "the peer token is not set on this server, so it cannot speak to the primary", nil)
		}
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		// Whatever the guest sent in its place, the header is the peer's.
		r.Header.Set("Authorization", TokenScheme+token)

		proxy.ServeHTTP(w, r)
	})
}

// startForward serves the VM's socket, unless this process already does
// or the socket answers. A socket nobody answers is stale and replaced.
func (api *API) startForward(name string) error {
	err := api.requirePrimary()
	if err != nil {
		return ez.Wrap(err)
	}

	api.mu.Lock()
	defer api.mu.Unlock()

	_, ok := api.forwards[name]
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

	srv := &http.Server{Handler: api.forwardHandler(name), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn().Err(err).Str("sandbox", name).Msg("peer socket forwarder stopped")
		}
	}()

	api.forwards[name] = &forward{srv: srv, ln: ln}

	return nil
}

// stopForward stops the VM's forwarder, if this process runs one.
func (api *API) stopForward(name string) {
	api.mu.Lock()
	f := api.forwards[name]
	delete(api.forwards, name)
	api.mu.Unlock()

	if f == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := f.srv.Shutdown(ctx)
	if err != nil {
		_ = f.srv.Close()
	}

	_ = f.ln.Close()
}

func (api *API) removeSocket(name string) {
	err := os.Remove(api.socketPath(name))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Warn().Err(err).Str("sandbox", name).Msg("cannot remove the peer socket")
	}
}

// Resume brings back the forwarder of every running VM this server runs
// for a primary, found by the agc.server label, for a peer restarting
// while its VMs kept running. Nothing happens on a server that is no
// peer.
func (api *API) Resume(ctx context.Context) error {
	if api.primary == "" {
		return nil
	}

	infos, err := api.list(ctx, task.LabelServer)
	if err != nil {
		return ez.Wrap(err)
	}

	for _, info := range infos {
		if info.Status != sandboxes.StatusRunning {
			continue
		}

		err = api.startForward(info.Name)
		if err != nil {
			log.Warn().Err(err).Str("sandbox", info.Name).Msg("cannot resume the peer socket forwarder")
		}
	}

	return nil
}

// ForwardNames lists the VMs whose sockets this process forwards.
func (api *API) ForwardNames() []string {
	api.mu.Lock()
	defer api.mu.Unlock()

	names := []string{}
	for name := range api.forwards {
		names = append(names, name)
	}

	return names
}
