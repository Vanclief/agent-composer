package sandboxes

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

// A sandbox runs on one agc server: the local one, or a peer named in the
// config's servers: (ADR 0018). Each server has a Runtime. api.rt is a
// router over them that picks the runtime from the sandbox row's server
// column on every call, by the sandbox's name, so every caller of the
// Runtime interface, in this package or a sibling, reaches the right
// server without knowing there is more than one.

// SecretChecker is implemented by the runtime of a peer, which reads
// secret values from its own secrets file: CheckSecrets fails, naming the
// secrets and the server, when any of bindings is not set there. The
// bindings carry names and hosts, never values.
type SecretChecker interface {
	CheckSecrets(ctx context.Context, bindings []secrets.Binding) error
}

// SetServers sets the runtimes of the peers, by server name, before the
// API is used. The local runtime is the one New got.
func (api *API) SetServers(peers map[string]Runtime) {
	api.mu.Lock()
	defer api.mu.Unlock()

	api.peers = map[string]Runtime{}
	for name, rt := range peers {
		api.peers[name] = rt
	}
}

// ServerNames are the servers a sandbox may run on: local, then the peers
// in name order.
func (api *API) ServerNames() []string {
	api.mu.Lock()
	defer api.mu.Unlock()

	names := []string{}
	for name := range api.peers {
		names = append(names, name)
	}
	sort.Strings(names)

	return append([]string{controller.LocalServer}, names...)
}

// serverKey is how a sandbox row stores server: empty for local.
func serverKey(server string) string {
	server = strings.TrimSpace(server)
	if server == controller.LocalServer {
		return ""
	}

	return server
}

// serverName is how a view shows a row's server: local for empty.
func serverName(key string) string {
	if key == "" {
		return controller.LocalServer
	}

	return key
}

// runtimeFor returns the runtime of the server stored as key, or an error
// naming the known servers.
func (api *API) runtimeFor(key string) (Runtime, error) {
	if key == "" {
		return api.local, nil
	}

	api.mu.Lock()
	rt, ok := api.peers[key]
	api.mu.Unlock()

	if !ok {
		msg := "server " + key + " is not configured here; the servers are " + strings.Join(api.ServerNames(), ", ")
		return nil, ez.New(ez.EINVALID, msg, nil)
	}

	return rt, nil
}

// resolveSecretsOn resolves the spec's secrets for the server stored as
// key. Local reads the values from this server's secrets file. A peer
// gets names and hosts only, after it says it has every one: secrets
// never travel between servers (ADR 0018).
func (api *API) resolveSecretsOn(ctx context.Context, key string, spec task.Spec) ([]secrets.Binding, error) {
	if key == "" {
		return api.resolveSecrets(spec)
	}

	rt, err := api.runtimeFor(key)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	requests, err := spec.SecretRequests()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(requests) == 0 {
		return nil, nil
	}

	bindings, err := secrets.Bind(requests)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	checker, ok := rt.(SecretChecker)
	if ok {
		err = checker.CheckSecrets(ctx, bindings)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	return bindings, nil
}

// router is api.rt: the Runtime that sends each call to the runtime of the
// server the sandbox's row names. A name with no row (a call made before
// the insert, or after the delete) goes to the local runtime.
type router struct {
	api *API
}

var _ Runtime = router{}

func (r router) pick(ctx context.Context, name string) (Runtime, error) {
	// With no peers every sandbox is local: no row to read.
	r.api.mu.Lock()
	peers := len(r.api.peers)
	r.api.mu.Unlock()

	if peers == 0 {
		return r.api.local, nil
	}

	record, err := taskmodels.GetSandboxByName(ctx, r.api.db, name)
	if err != nil {
		if ez.ErrorCode(err) == ez.ENOTFOUND {
			return r.api.local, nil
		}

		return nil, ez.Wrap(err)
	}

	return r.api.runtimeFor(record.Server)
}

func (r router) Create(ctx context.Context, spec sandbox.Spec) (Sandbox, error) {
	rt, err := r.pick(ctx, spec.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return rt.Create(ctx, spec)
}

func (r router) Connect(ctx context.Context, name string) (Sandbox, error) {
	rt, err := r.pick(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return rt.Connect(ctx, name)
}

func (r router) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	rt, err := r.pick(ctx, name)
	if err != nil {
		return ez.Wrap(err)
	}

	return rt.Start(ctx, name, secrets)
}

func (r router) Stop(ctx context.Context, name string) error {
	rt, err := r.pick(ctx, name)
	if err != nil {
		return ez.Wrap(err)
	}

	return rt.Stop(ctx, name)
}

func (r router) Remove(ctx context.Context, name string) error {
	rt, err := r.pick(ctx, name)
	if err != nil {
		return ez.Wrap(err)
	}

	return rt.Remove(ctx, name)
}

func (r router) Status(ctx context.Context, name string) (string, error) {
	rt, err := r.pick(ctx, name)
	if err != nil {
		return "", ez.Wrap(err)
	}

	return rt.Status(ctx, name)
}

// ServeForwarded answers a request a peer forwarded from the socket of
// the sandbox called name, which runs on that peer: exactly what the
// sandbox's socket would answer if the sandbox ran here, as its task or,
// for a home, as its agent (ADR 0018). The peer vouches for which sandbox
// spoke; who the sandbox speaks as comes from this server's rows, never
// from the request. r's path is the socket path, the forwarding prefix
// already stripped. A sandbox that runs on this server is refused: no
// peer speaks for it.
func (api *API) ServeForwarded(w http.ResponseWriter, r *http.Request, name string) {
	record, owner, err := api.findWithTask(r.Context(), uuid.Nil, name)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}

	if record.Server == "" {
		httpapi.WriteError(w, ez.New(ez.ENOTAUTHORIZED, "sandbox "+name+" runs on this server; no peer speaks for it", nil))
		return
	}

	spec, err := decodeSpec(record)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}

	handler, err := api.socketHandler(record, owner, spec)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}

	handler.ServeHTTP(w, r)
}

// Usage is what the running sandboxes of one server take: the sum of
// their specs' cpus and memory, with msb's defaults for a spec that sets
// none (ADR 0018).
type Usage struct {
	Sandboxes int    `json:"sandboxes"`
	CPUs      int    `json:"cpus"`
	MemoryMiB uint64 `json:"memory_mib"`
}

// Usage sums the running sandboxes of every server, by server name
// (local for this one). A server with none is absent.
func (api *API) Usage(ctx context.Context) (map[string]Usage, error) {
	records, err := taskmodels.ListSandboxes(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	usage := map[string]Usage{}
	for i := range records {
		record := &records[i]

		if !api.running(ctx, record) {
			continue
		}

		var spec task.Spec

		err = json.Unmarshal(record.Spec, &spec)
		if err != nil {
			continue
		}

		cpus := int(spec.CPUs)
		if cpus == 0 {
			cpus = sandbox.DefaultCPUs
		}

		memory := uint64(spec.Memory)
		if memory == 0 {
			memory = sandbox.DefaultMemoryMiB
		}

		name := serverName(record.Server)
		current := usage[name]
		current.Sandboxes++
		current.CPUs += cpus
		current.MemoryMiB += memory
		usage[name] = current
	}

	return usage, nil
}
