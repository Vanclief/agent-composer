package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/peers"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	msgclient "github.com/vanclief/agent-composer/messaging/client"
	"github.com/vanclief/agent-composer/task"
)

// Two agc servers in one process (ADR 0018): a primary with a peer called
// mini, each with its own secrets file and its own fake msb, talking over
// real HTTP on loopback. Nothing reaches another host.

// testPeerToken is a fixture, not a secret.
const testPeerToken = "not-a-real-peer-token"

// vmRuntime is a fake msb: VMs by name with a status, every call logged.
type vmRuntime struct {
	mu      sync.Mutex
	status  map[string]string
	specs   map[string]sandbox.Spec
	started map[string][]sandbox.Secret
	calls   []string
	stream  *fakeStream
	ran     []executor.Command
}

func newVMRuntime() *vmRuntime {
	return &vmRuntime{status: map[string]string{}, specs: map[string]sandbox.Spec{}, started: map[string][]sandbox.Secret{}}
}

func (r *vmRuntime) log(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, call)
}

func (r *vmRuntime) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.calls...)
}

func (r *vmRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	r.log("create " + spec.Name)

	r.mu.Lock()
	r.status[spec.Name] = "running"
	r.specs[spec.Name] = spec
	r.mu.Unlock()

	return &vmSandbox{rt: r}, nil
}

func (r *vmRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	r.log("connect " + name)

	return &vmSandbox{rt: r}, nil
}

func (r *vmRuntime) Start(ctx context.Context, name string, vmSecrets []sandbox.Secret) error {
	r.log("start " + name)

	r.mu.Lock()
	r.status[name] = "running"
	r.started[name] = vmSecrets
	r.mu.Unlock()

	return nil
}

func (r *vmRuntime) Stop(ctx context.Context, name string) error {
	r.log("stop " + name)

	r.mu.Lock()
	r.status[name] = "stopped"
	r.mu.Unlock()

	return nil
}

func (r *vmRuntime) Remove(ctx context.Context, name string) error {
	r.log("remove " + name)

	r.mu.Lock()
	delete(r.status, name)
	r.mu.Unlock()

	return nil
}

func (r *vmRuntime) Status(ctx context.Context, name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.status[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}

	return status, nil
}

// vmSandbox runs every command with exit 0 and serves one terminal.
type vmSandbox struct {
	executor.TerminalExecutor
	rt *vmRuntime
}

func (s *vmSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	s.rt.mu.Lock()
	s.rt.ran = append(s.rt.ran, cmd)
	s.rt.mu.Unlock()

	return &executor.Result{Stdout: []byte("ran " + cmd.Name)}, nil
}

func (s *vmSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	s.rt.log("write " + path)
	return nil
}

func (s *vmSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	s.rt.log("terminal " + cmd.Name)
	return &executor.Terminal{ID: "job_1", Command: append([]string{cmd.Name}, cmd.Args...), Running: true}, nil
}

func (s *vmSandbox) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	return []executor.Terminal{{ID: "job_1", Command: []string{"bash"}, Running: true}}, nil
}

func (s *vmSandbox) AttachTerminal(ctx context.Context, id string, replayBytes int) (executor.TerminalStream, error) {
	if id != "job_1" {
		return nil, ez.New(ez.ENOTFOUND, "cannot find job "+id, nil)
	}

	return s.rt.stream, nil
}

func (s *vmSandbox) Detach(ctx context.Context) error {
	return nil
}

// swapHandler lets an httptest server start before its handler exists,
// since each of the two servers needs the other's URL.
type swapHandler struct {
	mu sync.Mutex
	h  http.Handler
}

func (s *swapHandler) set(h http.Handler) {
	s.mu.Lock()
	s.h = h
	s.mu.Unlock()
}

func (s *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	h := s.h
	s.mu.Unlock()

	h.ServeHTTP(w, r)
}

func writeSecrets(t *testing.T, lines ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "secrets.env")

	err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

type peerPair struct {
	primary    *taskRoutes
	primaryURL string
	primaryRT  *vmRuntime
	peer       *taskRoutes
	peerURL    string
	peerRT     *vmRuntime
	peerAPI    *peers.API
	socketDir  string
}

// servePeerPair starts a primary whose servers: has mini, and mini, whose
// primary: is the primary. The primary's secrets file has no GITHUB_TOKEN;
// mini's has one.
func servePeerPair(t *testing.T) *peerPair {
	t.Helper()

	primaryHTTP := &swapHandler{}
	primaryServer := httptest.NewServer(primaryHTTP)
	t.Cleanup(primaryServer.Close)

	peerHTTP := &swapHandler{}
	peerServer := httptest.NewServer(peerHTTP)
	t.Cleanup(peerServer.Close)

	primarySecrets := writeSecrets(t, "AGC_PEER_TOKEN="+testPeerToken)
	peerSecrets := writeSecrets(t, "AGC_PEER_TOKEN="+testPeerToken, "GITHUB_TOKEN=peer-only-fixture")

	pair := &peerPair{primaryURL: primaryServer.URL, peerURL: peerServer.URL, primaryRT: newVMRuntime(), peerRT: newVMRuntime()}

	pair.primary = newTaskRoutes(t, pair.primaryRT, primarySecrets)
	primaryPeers, err := peers.New([]controller.ServerSettings{{Name: "mini", URL: peerServer.URL + "/"}}, "", pair.primary.server.SandboxesAPI, pair.primaryRT, primarySecrets, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pair.primary.server.PeersAPI = primaryPeers
	pair.primary.server.SandboxesAPI.SetServers(primaryPeers.Runtimes())

	// The peer's socket dir must be short: unix socket paths are capped
	// near 104 bytes.
	socketDir, err := os.MkdirTemp("", "agcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(socketDir)
	})
	pair.socketDir = socketDir

	pair.peer = newTaskRoutes(t, pair.peerRT, peerSecrets)
	pair.peerAPI, err = peers.New(nil, primaryServer.URL, pair.peer.server.SandboxesAPI, pair.peerRT, peerSecrets, socketDir)
	if err != nil {
		t.Fatal(err)
	}
	pair.peer.server.PeersAPI = pair.peerAPI
	t.Cleanup(func() {
		for _, name := range pair.peerAPI.ForwardNames() {
			_, _ = pair.peerAPI.StopVM(context.Background(), nil, &peers.StopVMRequest{Name: name})
		}
	})

	primaryHTTP.set(pair.primary.e)
	peerHTTP.set(pair.peer.e)

	return pair
}

// waitStatus polls the primary until the sandbox has status.
func (p *peerPair) waitStatus(t *testing.T, name string, status string) sandboxes.SandboxView {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		code, body := p.primary.do(http.MethodGet, "/api/sandboxes/"+name, "")
		if code != http.StatusOK {
			t.Fatalf("GET sandbox %s: %d %s", name, code, body)
		}

		view := sandboxes.SandboxView{}
		err := json.Unmarshal([]byte(body), &view)
		if err != nil {
			t.Fatal(err)
		}

		if view.Status == status {
			return view
		}

		if time.Now().After(deadline) {
			t.Fatalf("sandbox %s is %s (%s), want %s", name, view.Status, view.StatusError, status)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func peerSocketClient(path string) *msgclient.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", path)
		},
	}

	return &msgclient.Client{BaseURL: "http://task", HTTP: &http.Client{Transport: transport}}
}

const peerTaskBody = `{"title":"Remote work","name":"remote","brief":"b","assignee":"franco","manager":"franco",
"sandbox":{"server":"mini","secrets":["GITHUB_TOKEN"],"command":"bash"}}`

func TestPeerAllocatesOnThePeer(t *testing.T) {
	pair := servePeerPair(t)

	code, body := pair.primary.do(http.MethodPost, "/api/tasks", peerTaskBody)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}

	view := pair.waitStatus(t, "remote", "running")
	if view.Server != "mini" {
		t.Fatalf("server = %q, want mini", view.Server)
	}

	// The primary's msb was never used.
	if len(pair.primaryRT.Calls()) != 0 {
		t.Fatalf("primary runtime was called: %q", pair.primaryRT.Calls())
	}

	pair.peerRT.mu.Lock()
	spec := pair.peerRT.specs["remote"]
	pair.peerRT.mu.Unlock()

	// The secret's value is the peer's, read from the peer's own file.
	if len(spec.Secrets) != 1 || spec.Secrets[0].Name != "GITHUB_TOKEN" || spec.Secrets[0].Value != "peer-only-fixture" {
		t.Fatalf("peer VM secrets = %+v, want GITHUB_TOKEN from the peer's file", spec.Secrets)
	}
	if strings.Join(spec.Secrets[0].Hosts, ",") != "api.github.com,github.com" {
		t.Fatalf("hosts = %v", spec.Secrets[0].Hosts)
	}
	if spec.VsockPath != filepath.Join(pair.socketDir, "remote.sock") {
		t.Fatalf("vsock path = %q, want the peer's socket", spec.VsockPath)
	}
	if spec.Labels[task.LabelServer] != "mini" || spec.Labels[task.LabelKey] != "remote" {
		t.Fatalf("labels = %v", spec.Labels)
	}

	// The boot ran through the peer: TASK.md written, command started.
	calls := strings.Join(pair.peerRT.Calls(), "|")
	if !strings.Contains(calls, "write "+sandboxes.BriefPath) || !strings.Contains(calls, "terminal bash") {
		t.Fatalf("peer calls = %s", calls)
	}

	// Terminals are read through the primary.
	code, body = pair.primary.do(http.MethodGet, "/api/sandboxes/remote/terminals", "")
	if code != http.StatusOK || !strings.Contains(body, `"job_1"`) {
		t.Fatalf("terminals: %d %s", code, body)
	}

	// Stop, start and delete reach the peer.
	code, body = pair.primary.do(http.MethodPost, "/api/sandboxes/remote/stop", "")
	if code != http.StatusOK || !strings.Contains(body, `"status":"stopped"`) {
		t.Fatalf("stop: %d %s", code, body)
	}
	if len(pair.peerAPI.ForwardNames()) != 0 {
		t.Fatal("the peer still forwards a stopped VM's socket")
	}

	code, body = pair.primary.do(http.MethodPost, "/api/sandboxes/remote/start", "")
	if code != http.StatusOK || !strings.Contains(body, `"status":"running"`) {
		t.Fatalf("start: %d %s", code, body)
	}
	pair.peerRT.mu.Lock()
	restarted := pair.peerRT.started["remote"]
	pair.peerRT.mu.Unlock()
	if len(restarted) != 1 || restarted[0].Value != "peer-only-fixture" {
		t.Fatalf("start secrets = %+v, want the peer's value", restarted)
	}

	code, body = pair.primary.do(http.MethodDelete, "/api/sandboxes/remote", "")
	if code != http.StatusOK {
		t.Fatalf("delete: %d %s", code, body)
	}
	calls = strings.Join(pair.peerRT.Calls(), "|")
	if !strings.HasSuffix(calls, "stop remote|remove remote") {
		t.Fatalf("peer calls after delete = %s", calls)
	}
	if len(pair.primaryRT.Calls()) != 0 {
		t.Fatalf("primary runtime was called: %q", pair.primaryRT.Calls())
	}
}

func TestPeerSocketForwardsToThePrimary(t *testing.T) {
	pair := servePeerPair(t)
	ctx := context.Background()

	code, body := pair.primary.do(http.MethodPost, "/api/tasks", peerTaskBody)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}
	pair.waitStatus(t, "remote", "running")

	client := peerSocketClient(filepath.Join(pair.socketDir, "remote.sock"))

	participant, err := client.Join(ctx, "agent")
	if err != nil {
		t.Fatal(err)
	}

	// The sandbox posts in #remote as its task, decided by the primary.
	_, err = client.Post(ctx, participant.ID, "#remote", "hello from mini", nil)
	if err != nil {
		t.Fatal(err)
	}

	// The primary's access rule still applies: no grant, no post.
	_, err = client.Post(ctx, participant.ID, "#elsewhere", "no", nil)
	if err == nil || ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("post to an ungranted channel: %v, want not authorized", err)
	}

	code, body = pair.primary.do(http.MethodGet, "/api/msg/read?channel=%23remote", "")
	if code != http.StatusOK || !strings.Contains(body, "hello from mini") {
		t.Fatalf("read on the primary: %d %s", code, body)
	}

	// A wait blocks on the peer until the primary has a message.
	seen, err := client.Read(ctx, "#remote", 0, 100)
	if err != nil {
		t.Fatal(err)
	}

	got := make(chan error, 1)
	go func() {
		messages, err := client.Wait(ctx, "#remote", seen.Cursor, 5*time.Second)
		if err == nil && len(messages.Messages) == 0 {
			err = errors.New("wait answered with no message")
		}
		got <- err
	}()

	code, body = pair.primary.do(http.MethodPost, "/api/msg/join", `{"name":"franco"}`)
	if code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("join on the primary: %d %s", code, body)
	}
	franco := struct {
		ID string `json:"id"`
	}{}
	err = json.Unmarshal([]byte(body), &franco)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	code, body = pair.primary.do(http.MethodPost, "/api/msg/post", `{"participant_id":"`+franco.ID+`","channel":"#remote","body":"answer"}`)
	if code != http.StatusOK && code != http.StatusCreated {
		t.Fatalf("post on the primary: %d %s", code, body)
	}

	select {
	case err = <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wait through the peer never answered")
	}

	// agc task deliver from the peer's VM delivers the task on the primary.
	err = client.Deliver(ctx, "done on mini", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	code, body = pair.primary.do(http.MethodGet, "/api/tasks/remote", "")
	if code != http.StatusOK || !strings.Contains(body, `"status":"review"`) {
		t.Fatalf("task after deliver: %d %s", code, body)
	}

	// The peer has no rows and answers nothing itself: its own messaging
	// never saw the post.
	code, body = pair.peer.do(http.MethodGet, "/api/msg/read?channel=%23remote", "")
	if code != http.StatusOK || strings.Contains(body, "hello from mini") {
		t.Fatalf("read on the peer: %d %s", code, body)
	}
}

func TestPeerTerminalIsRelayedThroughThePrimary(t *testing.T) {
	pair := servePeerPair(t)

	pair.peerRT.stream = &fakeStream{
		chunks: [][]byte{[]byte("hello "), []byte("mini")},
		input:  make(chan []byte, 1),
		sizes:  make(chan [2]uint16, 1),
		exited: make(chan struct{}),
		closed: make(chan struct{}),
	}

	code, body := pair.primary.do(http.MethodPost, "/api/tasks", peerTaskBody)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}
	pair.waitStatus(t, "remote", "running")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The panel dials the primary only.
	target := "ws" + strings.TrimPrefix(pair.primaryURL, "http") + "/api/sandboxes/remote/terminals/job_1/ws"
	conn, _, err := websocket.Dial(ctx, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	for _, want := range []string{"hello ", "mini"} {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if typ != websocket.MessageBinary || string(data) != want {
			t.Fatalf("got %v %q, want binary %q", typ, data, want)
		}
	}

	err = conn.Write(ctx, websocket.MessageBinary, []byte("ls\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","rows":40,"cols":120}`))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = conn.Read(ctx)
	var closeErr websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.StatusNormalClosure {
		t.Fatalf("read after exit: %v, want normal closure", err)
	}

	if string(<-pair.peerRT.stream.input) != "ls\n" {
		t.Fatal("input did not reach the peer's terminal")
	}
	if <-pair.peerRT.stream.sizes != [2]uint16{40, 120} {
		t.Fatal("resize did not reach the peer's terminal")
	}
}

func TestPeerAllocationFailsFast(t *testing.T) {
	pair := servePeerPair(t)

	cases := []routeCase{
		// An unknown server names the known ones.
		{http.MethodPost, "/api/tasks", `{"title":"x","name":"x1","brief":"b","assignee":"franco","manager":"franco","sandbox":{"server":"attic"}}`,
			http.StatusBadRequest, "server attic is not configured here; the servers are local, mini"},
		// A secret the peer lacks names the secret and the server; the
		// primary's own file is never read for a peer's sandbox.
		{http.MethodPost, "/api/tasks", `{"title":"y","name":"y1","brief":"b","assignee":"franco","manager":"franco","sandbox":{"server":"mini","secrets":["OPENAI_API_KEY"]}}`,
			http.StatusNotFound, "secret OPENAI_API_KEY is not set on server mini"},
		// The peer token is agc's own and no sandbox gets it.
		{http.MethodPost, "/api/tasks", `{"title":"z","name":"z1","brief":"b","assignee":"franco","manager":"franco","sandbox":{"server":"mini","secrets":["AGC_PEER_TOKEN@example.com"]}}`,
			http.StatusBadRequest, "secret AGC_PEER_TOKEN is agc's own"},
	}
	pair.primary.check(t, cases)

	// Nothing was inserted or booted.
	code, body := pair.primary.do(http.MethodGet, "/api/sandboxes", "")
	if code != http.StatusOK || strings.Contains(body, `"name":"`) {
		t.Fatalf("sandboxes after failed allocations: %d %s", code, body)
	}
	if len(pair.peerRT.Calls()) != 0 {
		t.Fatalf("peer runtime was called: %q", pair.peerRT.Calls())
	}
}

func TestListServers(t *testing.T) {
	pair := servePeerPair(t)

	code, body := pair.primary.do(http.MethodPost, "/api/tasks", peerTaskBody)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}
	pair.waitStatus(t, "remote", "running")

	code, body = pair.primary.do(http.MethodGet, "/api/servers", "")
	if code != http.StatusOK {
		t.Fatalf("servers: %d %s", code, body)
	}

	out := peers.ListServersResponse{}
	err := json.Unmarshal([]byte(body), &out)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.Servers) != 2 || out.Servers[0].Name != "local" || out.Servers[1].Name != "mini" {
		t.Fatalf("servers = %s", body)
	}

	mini := out.Servers[1]
	if !mini.Reachable || mini.Host == nil || mini.Host.CPUs == 0 || mini.URL != pair.peerURL {
		t.Fatalf("mini = %+v", mini)
	}
	// One running sandbox with msb's defaults.
	if mini.Sandboxes != 1 || mini.InUse.CPUs != sandbox.DefaultCPUs || mini.InUse.MemoryMiB != sandbox.DefaultMemoryMiB {
		t.Fatalf("mini in use = %+v", mini)
	}
	if out.Servers[0].Sandboxes != 0 || !out.Servers[0].Reachable {
		t.Fatalf("local = %+v", out.Servers[0])
	}
}

func TestListServersMarksAPeerDown(t *testing.T) {
	secretsPath := writeSecrets(t, "AGC_PEER_TOKEN="+testPeerToken)
	routes := newTaskRoutes(t, newVMRuntime(), secretsPath)

	// A port nothing listens on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	down := "http://" + ln.Addr().String()
	_ = ln.Close()

	api, err := peers.New([]controller.ServerSettings{{Name: "gone", URL: down}}, "", routes.server.SandboxesAPI, newVMRuntime(), secretsPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	routes.server.PeersAPI = api

	routes.check(t, []routeCase{
		{http.MethodGet, "/api/servers", "", http.StatusOK, `"name":"gone","url":"` + down + `","reachable":false,"error":"cannot reach server gone at ` + down},
	})
}

func TestPeerRoutesNeedTheToken(t *testing.T) {
	pair := servePeerPair(t)

	cases := []struct {
		header string
		status int
		want   string
	}{
		{"", http.StatusUnauthorized, "the peer token is missing or wrong"},
		{"Bearer wrong", http.StatusUnauthorized, "the peer token is missing or wrong"},
		{"Bearer " + testPeerToken, http.StatusOK, `"cpus"`},
	}

	for _, tc := range cases {
		request := httptest.NewRequest(http.MethodGet, "/api/peer/host", nil)
		if tc.header != "" {
			request.Header.Set("Authorization", tc.header)
		}
		recorder := httptest.NewRecorder()
		pair.peer.e.ServeHTTP(recorder, request)

		if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("%q: %d %s", tc.header, recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), testPeerToken) {
			t.Fatal("an answer echoed the token")
		}
	}

	// The forwarded socket route checks the token too.
	request := httptest.NewRequest(http.MethodPost, "/api/peer/sockets/remote/post", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	pair.primary.e.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("socket without token: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPeerRoutesAreOffWithoutAToken(t *testing.T) {
	secretsPath := writeSecrets(t, "GITHUB_TOKEN=fixture")
	routes := newTaskRoutes(t, newVMRuntime(), secretsPath)
	api, err := peers.New(nil, "", routes.server.SandboxesAPI, newVMRuntime(), secretsPath, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	routes.server.PeersAPI = api

	request := httptest.NewRequest(http.MethodGet, "/api/peer/host", nil)
	request.Header.Set("Authorization", "Bearer anything")
	recorder := httptest.NewRecorder()
	routes.e.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "AGC_PEER_TOKEN is not set") {
		t.Fatalf("%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestForwardedSocketRefusesALocalSandbox(t *testing.T) {
	pair := servePeerPair(t)

	// A sandbox on the primary itself.
	code, body := pair.primary.do(http.MethodPost, "/api/tasks", `{"title":"Here","name":"here","brief":"b","assignee":"franco","manager":"franco","sandbox":{}}`)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, body)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/peer/sockets/here/join", strings.NewReader(`{"name":"x"}`))
	request.Header.Set("Authorization", "Bearer "+testPeerToken)
	recorder := httptest.NewRecorder()
	pair.primary.e.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "runs on this server; no peer speaks for it") {
		t.Fatalf("%d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPeerResumesForwardersOfRunningVMs(t *testing.T) {
	pair := servePeerPair(t)

	pair.peerAPI.SetList(func(ctx context.Context, labelKey string) ([]sandbox.Info, error) {
		if labelKey != task.LabelServer {
			t.Fatalf("listed by %q, want %q", labelKey, task.LabelServer)
		}

		return []sandbox.Info{{Name: "up", Status: "running"}, {Name: "down", Status: "stopped"}}, nil
	})

	err := pair.peerAPI.Resume(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	names := pair.peerAPI.ForwardNames()
	if len(names) != 1 || names[0] != "up" {
		t.Fatalf("forwarders = %v, want [up]", names)
	}
}
