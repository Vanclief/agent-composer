package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	msgclient "github.com/vanclief/agent-composer/messaging/client"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// fakeRuntime records the calls that change a sandbox, in order.
type fakeRuntime struct {
	mu        sync.Mutex
	calls     []string
	statuses  map[string]string
	failing   map[string]bool // Status fails for these names
	created   []sandbox.Spec
	started   [][]sandbox.Secret
	runFailOn string // a Run whose first arg is this exits 128
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{statuses: map[string]string{}, failing: map[string]bool{}}
}

func (f *fakeRuntime) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeRuntime) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeRuntime) Create(ctx context.Context, spec sandbox.Spec) (Sandbox, error) {
	f.record("create " + spec.Name)
	f.mu.Lock()
	f.created = append(f.created, spec)
	f.statuses[spec.Name] = "running"
	f.mu.Unlock()
	return &fakeSandbox{rt: f}, nil
}

func (f *fakeRuntime) Connect(ctx context.Context, name string) (Sandbox, error) {
	return &fakeSandbox{rt: f}, nil
}

func (f *fakeRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	f.record("start " + name)
	f.mu.Lock()
	f.started = append(f.started, secrets)
	f.statuses[name] = "running"
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntime) Stop(ctx context.Context, name string) error {
	f.record("stop " + name)
	f.mu.Lock()
	f.statuses[name] = "stopped"
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntime) Remove(ctx context.Context, name string) error {
	f.record("remove " + name)
	f.mu.Lock()
	delete(f.statuses, name)
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntime) Status(ctx context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failing[name] {
		return "", ez.New(ez.EUNAVAILABLE, "msb is not installed", nil)
	}
	status, ok := f.statuses[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}
	return status, nil
}

// fakeSandbox implements Run and Detach. Any other method panics on the
// nil embedded interface, which no test reaches.
type fakeSandbox struct {
	executor.TerminalExecutor
	rt *fakeRuntime
}

func (s *fakeSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	s.rt.record("run " + cmd.Name + " " + strings.Join(cmd.Args, " ") + " in " + cmd.Dir)
	if len(cmd.Args) > 0 && cmd.Args[0] == s.rt.runFailOn {
		return &executor.Result{ExitCode: 128, Stderr: []byte("fatal: repository not found\n")}, nil
	}
	return &executor.Result{}, nil
}

func (s *fakeSandbox) Detach(ctx context.Context) error {
	s.rt.record("detach")
	return nil
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Every pooled connection would get its own :memory: database.
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	tables := []interface{}{
		(*taskmodels.Task)(nil),
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	return db
}

type testEnv struct {
	api *API
	rt  *fakeRuntime
	db  *bun.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	err := os.WriteFile(secretsPath, []byte("GITHUB_TOKEN=ghp_value\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Unix socket paths are capped near 104 bytes, too short for
	// t.TempDir() on macOS.
	socketDir, err := os.MkdirTemp("", "agc")
	if err != nil {
		t.Fatal(err)
	}

	db := newTestDB(t)
	rt := newFakeRuntime()
	api := newAPI(db, messaging.NewService(db), rt, secretsPath, socketDir)
	t.Cleanup(func() {
		for _, name := range api.listenerNames() {
			api.stopListener(name)
		}
		_ = os.RemoveAll(socketDir)
	})

	return &testEnv{api: api, rt: rt, db: db}
}

func (api *API) listenerNames() []string {
	api.mu.Lock()
	defer api.mu.Unlock()

	names := []string{}
	for name := range api.listeners {
		names = append(names, name)
	}

	return names
}

func socketExists(t *testing.T, path string) bool {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}

	return info.Mode()&os.ModeSocket != 0
}

func testSpec() task.Spec {
	return task.Spec{
		Repo:     "github.com/vanclief/agent-composer",
		Setup:    "go mod download",
		Secrets:  []string{"GITHUB_TOKEN"},
		Env:      map[string]string{"GOFLAGS": "-mod=mod"},
		Channels: []string{"#releases"},
	}
}

func TestCreate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	view, err := env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: testSpec()})
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "review-42" || view.Status != "running" || view.Spec.Repo != testSpec().Repo {
		t.Fatalf("unexpected view %+v", view)
	}

	socket := env.api.socketPath("review-42")
	if !socketExists(t, socket) {
		t.Fatalf("no listener socket at %s", socket)
	}

	wantCalls := []string{
		"create review-42",
		"run git clone https://github.com/vanclief/agent-composer.git /home/agent/agent-composer in ",
		"run bash -lc go mod download in /home/agent/agent-composer",
		"detach",
	}
	if !reflect.DeepEqual(env.rt.Calls(), wantCalls) {
		t.Fatalf("calls = %q", env.rt.Calls())
	}

	created := env.rt.created[0]
	if created.Labels["agc.task"] != "review-42" {
		t.Fatalf("labels = %v", created.Labels)
	}
	if created.Env["AGC_TASK"] != "review-42" || created.Env["AGC_MSG_VSOCK_PORT"] != "5100" || created.Env["GOFLAGS"] != "-mod=mod" {
		t.Fatalf("env = %v", created.Env)
	}
	wantSecrets := []sandbox.Secret{{Name: "GITHUB_TOKEN", Value: "ghp_value", Hosts: []string{"api.github.com", "github.com"}}}
	if !reflect.DeepEqual(created.Secrets, wantSecrets) {
		t.Fatalf("secrets = %+v", created.Secrets)
	}
	if created.VsockPath != socket || created.VsockPort != 5100 {
		t.Fatalf("vsock = %s:%d", created.VsockPath, created.VsockPort)
	}

	record, err := taskmodels.GetTaskByName(ctx, env.db, "review-42")
	if err != nil {
		t.Fatal(err)
	}

	var stored task.Spec
	err = json.Unmarshal(record.Spec, &stored)
	if err != nil || !reflect.DeepEqual(stored, testSpec()) {
		t.Fatalf("stored spec = %+v, err %v", stored, err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: task.Spec{}})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("expected ECONFLICT for a duplicate name, got %v", err)
	}
}

func TestCreateWithRef(t *testing.T) {
	env := newTestEnv(t)

	spec := task.Spec{Repo: "github.com/o/n", Ref: "v1.2.0"}
	_, err := env.api.Create(context.Background(), nil, &CreateRequest{Name: "ref-task", Spec: spec})
	if err != nil {
		t.Fatal(err)
	}

	calls := env.rt.Calls()
	if calls[2] != "run git -C /home/agent/n checkout v1.2.0 in " {
		t.Fatalf("calls = %q", calls)
	}
}

func TestCreateMissingSecret(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	spec := task.Spec{Secrets: []string{"OPENAI_API_KEY"}}
	_, err := env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: spec})
	if ez.ErrorCode(err) != ez.ENOTFOUND || !strings.Contains(ez.ErrorMessage(err), "OPENAI_API_KEY") {
		t.Fatalf("expected ENOTFOUND naming the secret, got %v", err)
	}

	if len(env.rt.Calls()) != 0 {
		t.Fatalf("runtime was called: %q", env.rt.Calls())
	}

	if socketExists(t, env.api.socketPath("review-42")) {
		t.Fatal("listener socket exists")
	}

	_, err = taskmodels.GetTaskByName(ctx, env.db, "review-42")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected no row, got %v", err)
	}
}

func TestCreateCloneFails(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.rt.runFailOn = "clone"

	_, err := env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: testSpec()})
	if ez.ErrorCode(err) != ez.EINTERNAL {
		t.Fatalf("expected EINTERNAL, got %v", err)
	}

	msg := ez.ErrorMessage(err)
	if !strings.Contains(msg, "fatal: repository not found") || !strings.Contains(msg, "sandbox review-42 was removed") {
		t.Fatalf("message = %q", msg)
	}

	wantCalls := []string{
		"create review-42",
		"run git clone https://github.com/vanclief/agent-composer.git /home/agent/agent-composer in ",
		"detach",
		"stop review-42",
		"remove review-42",
	}
	if !reflect.DeepEqual(env.rt.Calls(), wantCalls) {
		t.Fatalf("calls = %q", env.rt.Calls())
	}

	if socketExists(t, env.api.socketPath("review-42")) {
		t.Fatal("listener socket still exists")
	}

	_, err = taskmodels.GetTaskByName(ctx, env.db, "review-42")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected no row, got %v", err)
	}
}

func insertTask(t *testing.T, db bun.IDB, name string, createdAt time.Time) {
	t.Helper()

	record := &taskmodels.Task{Name: name, Spec: json.RawMessage(`{"channels":["#releases:read"]}`), CreatedAt: createdAt}
	err := record.Insert(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
}

func TestList(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now().UTC()

	insertTask(t, env.db, "old", now.Add(-2*time.Hour))
	insertTask(t, env.db, "mid", now.Add(-time.Hour))
	insertTask(t, env.db, "new", now)
	env.rt.statuses["old"] = "stopped"
	env.rt.statuses["mid"] = "crashed"
	env.rt.failing["new"] = true

	list, err := env.api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	views := list.Tasks

	got := []string{}
	for _, view := range views {
		got = append(got, view.Name+"="+view.Status+"/"+view.StatusError)
	}

	want := []string{"new=unknown/msb is not installed", "mid=crashed/", "old=stopped/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}

	if views[2].Spec.Channels[0] != "#releases:read" {
		t.Fatalf("spec not decoded: %+v", views[2].Spec)
	}
}

func TestStopStartDelete(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created, err := env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: testSpec()})
	if err != nil {
		t.Fatal(err)
	}
	socket := env.api.socketPath("review-42")
	before := len(env.rt.Calls())

	view, err := env.api.Stop(ctx, nil, &StopRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "stopped" || socketExists(t, socket) {
		t.Fatalf("after stop: status %s, socket exists %v", view.Status, socketExists(t, socket))
	}

	err = os.WriteFile(env.api.secretsPath, []byte("GITHUB_TOKEN=rotated\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	view, err = env.api.Start(ctx, nil, &StartRequest{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" || !socketExists(t, socket) {
		t.Fatalf("after start: status %s, socket exists %v", view.Status, socketExists(t, socket))
	}
	if env.rt.started[0][0].Value != "rotated" {
		t.Fatalf("start did not re-read secrets: %+v", env.rt.started[0])
	}

	deleted, err := env.api.Delete(ctx, nil, &DeleteRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Deleted || deleted.Name != "review-42" {
		t.Fatalf("delete response %+v", deleted)
	}

	wantCalls := []string{"stop review-42", "start review-42", "stop review-42", "remove review-42"}
	if !reflect.DeepEqual(env.rt.Calls()[before:], wantCalls) {
		t.Fatalf("calls = %q", env.rt.Calls()[before:])
	}

	if socketExists(t, socket) {
		t.Fatal("socket still exists after delete")
	}

	_, err = env.api.Get(ctx, nil, &GetRequest{Name: "review-42"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected ENOTFOUND after delete, got %v", err)
	}
}

func TestDeleteStoppedDoesNotStop(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "saved", time.Now())
	env.rt.statuses["saved"] = "stopped"

	_, err := env.api.Delete(context.Background(), nil, &DeleteRequest{Name: "saved"})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(env.rt.Calls(), []string{"remove saved"}) {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestDeleteGoneSandbox(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "gone", time.Now())

	_, err := env.api.Delete(context.Background(), nil, &DeleteRequest{Name: "gone"})
	if err != nil {
		t.Fatal(err)
	}

	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestResume(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()

	insertTask(t, env.db, "up", now)
	insertTask(t, env.db, "down", now)
	insertTask(t, env.db, "broken", now)
	env.rt.statuses["up"] = "running"
	env.rt.statuses["down"] = "stopped"
	env.rt.failing["broken"] = true

	err := env.api.Resume(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(env.api.listenerNames(), []string{"up"}) {
		t.Fatalf("listeners = %v", env.api.listenerNames())
	}

	if !socketExists(t, env.api.socketPath("up")) || socketExists(t, env.api.socketPath("down")) {
		t.Fatal("unexpected sockets")
	}
}

func socketClient(path string) *msgclient.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", path)
		},
	}

	return &msgclient.Client{BaseURL: "http://task", HTTP: &http.Client{Transport: transport}}
}

func TestListenerServesMessagingAsTheTask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.api.Create(ctx, nil, &CreateRequest{Name: "review-42", Spec: task.Spec{Channels: []string{"#alerts:read"}}})
	if err != nil {
		t.Fatal(err)
	}

	client := socketClient(env.api.socketPath("review-42"))

	p, err := client.Join(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}

	m, err := client.Post(ctx, p.ID, "#review-42", "hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Task != "review-42" || m.From != "reviewer" {
		t.Fatalf("message = %+v", m)
	}

	_, err = client.Post(ctx, p.ID, "#alerts", "no", nil)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("expected ENOTAUTHORIZED posting to a read-only grant, got %v", err)
	}
}

func TestStartListenerLeavesASocketServedElsewhere(t *testing.T) {
	env := newTestEnv(t)
	other := newAPI(env.db, env.api.msg, env.rt, env.api.secretsPath, env.api.socketDir)

	err := env.api.startListener("shared", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = other.startListener("shared", nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(other.listenerNames()) != 0 {
		t.Fatal("second process took over a socket that is being served")
	}

	_, err = socketClient(env.api.socketPath("shared")).Join(context.Background(), "still-served")
	if err != nil {
		t.Fatalf("socket no longer served: %v", err)
	}
}
