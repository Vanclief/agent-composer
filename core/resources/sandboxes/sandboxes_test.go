package sandboxes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	accountmodels "github.com/vanclief/agent-composer/models/account"
	agentmodels "github.com/vanclief/agent-composer/models/agent"
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
	files     map[string]string
	terminals map[string][]executor.Terminal // jobs per sandbox, cleared by Stop
	runFailOn string                         // a Run whose first arg is this exits 128
	// blockCreate makes Create wait until its context ends.
	blockCreate bool
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{statuses: map[string]string{}, failing: map[string]bool{}, files: map[string]string{}, terminals: map[string][]executor.Terminal{}}
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
	if f.blockCreate {
		<-ctx.Done()
		return nil, ez.New(ez.EINTERNAL, "create cancelled", ctx.Err())
	}
	f.mu.Lock()
	f.created = append(f.created, spec)
	f.statuses[spec.Name] = "running"
	f.mu.Unlock()
	return &fakeSandbox{rt: f, name: spec.Name}, nil
}

func (f *fakeRuntime) Connect(ctx context.Context, name string) (Sandbox, error) {
	return &fakeSandbox{rt: f, name: name}, nil
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
	delete(f.terminals, name)
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

// fakeSandbox implements Run, WriteFile, StartTerminal, ListTerminals and
// Detach. Any
// other method panics on the nil embedded interface, which no test
// reaches.
type fakeSandbox struct {
	executor.TerminalExecutor
	rt   *fakeRuntime
	name string
}

func (s *fakeSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	call := "run " + cmd.Name + " " + strings.Join(cmd.Args, " ") + " in " + cmd.Dir
	if cmd.User != "" {
		call += " as " + cmd.User
	}
	s.rt.record(call)
	if len(cmd.Args) > 0 && cmd.Args[0] == s.rt.runFailOn {
		return &executor.Result{ExitCode: 128, Stderr: []byte("fatal: repository not found\n")}, nil
	}
	return &executor.Result{}, nil
}

func (s *fakeSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	s.rt.record("write " + path)
	s.rt.mu.Lock()
	s.rt.files[path] = string(data)
	s.rt.mu.Unlock()
	return nil
}

func (s *fakeSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	s.rt.record("terminal " + cmd.Name + " " + strings.Join(cmd.Args, " ") + " in " + cmd.Dir)
	terminal := executor.Terminal{ID: "job_1", Command: append([]string{cmd.Name}, cmd.Args...), Running: true}
	s.rt.mu.Lock()
	s.rt.terminals[s.name] = append(s.rt.terminals[s.name], terminal)
	s.rt.mu.Unlock()
	return &terminal, nil
}

func (s *fakeSandbox) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	s.rt.mu.Lock()
	defer s.rt.mu.Unlock()
	return append([]executor.Terminal(nil), s.rt.terminals[s.name]...), nil
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
		(*taskmodels.Sandbox)(nil),
		(*msgmodels.Participant)(nil),
		(*msgmodels.Message)(nil),
		(*msgmodels.ChannelArchive)(nil),
		(*accountmodels.Account)(nil),
		(*agentmodels.Agent)(nil),
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

	// No test reads the host's msb volumes.
	t.Setenv(EnvVolumeRoot, t.TempDir())

	db := newTestDB(t)
	rt := newFakeRuntime()
	api := New(context.Background(), db, messaging.NewService(db), rt, secretsPath, socketDir)
	t.Cleanup(func() {
		api.Wait()
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

func testSandbox() task.Sandbox {
	return task.Sandbox{
		Spec: task.Spec{
			Repo:     "github.com/vanclief/agent-composer",
			Setup:    "go mod download",
			Secrets:  []string{"GITHUB_TOKEN"},
			Env:      map[string]string{"GOFLAGS": "-mod=mod"},
			Channels: []string{"#releases"},
		},
		Command: "claude --dangerously-skip-permissions",
	}
}

func insertTask(t *testing.T, db bun.IDB, name string) *taskmodels.Task {
	t.Helper()

	record := &taskmodels.Task{Title: "Title of " + name, Name: name, Brief: "Do " + name + "."}
	err := record.Insert(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}

	return record
}

// insertBooted inserts a sandbox that booted at createdAt.
func insertBooted(t *testing.T, db bun.IDB, owner *taskmodels.Task, name string, createdAt time.Time) *taskmodels.Sandbox {
	t.Helper()

	record := &taskmodels.Sandbox{TaskID: owner.ID, Name: name, Spec: []byte(`{"channels":["#releases:read"]}`), BootedAt: &createdAt, CreatedAt: createdAt}
	err := record.Insert(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}

	return record
}

func TestAllocate(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	owner := insertTask(t, env.db, "review-42")

	view, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: testSandbox()})
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "review-42" || view.Task != "review-42" || view.TaskID != owner.ID || view.Status != "starting" || view.BootedAt != nil {
		t.Fatalf("unexpected view %+v", view)
	}
	if view.Spec.Repo != testSandbox().Repo || view.Command != testSandbox().Command {
		t.Fatalf("unexpected spec or command %+v", view)
	}

	env.api.Wait()
	// Get then checks the clone for work, which is TestGetWork's concern.
	bootCalls := env.rt.Calls()

	got, err := env.api.Get(ctx, nil, &GetRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "running" || got.BootedAt == nil || got.StatusError != "" {
		t.Fatalf("after boot: %+v", got)
	}

	socket := env.api.socketPath("review-42")
	if !socketExists(t, socket) {
		t.Fatalf("no listener socket at %s", socket)
	}

	script, err := agentConfigScript("/home/agent/agent-composer")
	if err != nil {
		t.Fatal(err)
	}

	wantCalls := []string{
		"create review-42",
		"run chown agent:agent /home/agent in  as root",
		"run bash -c " + homeScript + " in ",
		"run git clone https://github.com/vanclief/agent-composer.git /home/agent/agent-composer in ",
		"run bash -c " + script + " in ",
		"write /home/agent/TASK.md",
		"run bash -lc go mod download in /home/agent/agent-composer",
		"terminal claude --dangerously-skip-permissions in /home/agent/agent-composer",
		"detach",
	}
	if !reflect.DeepEqual(bootCalls, wantCalls) {
		t.Fatalf("calls = %q", bootCalls)
	}

	brief := env.rt.files["/home/agent/TASK.md"]
	for _, want := range []string{"# Title of review-42\n", "Do review-42.", "`agc/review-42`", "`#review-42`"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("TASK.md lacks %q:\n%s", want, brief)
		}
	}

	created := env.rt.created[0]
	if !reflect.DeepEqual(created.Labels, map[string]string{"agc.task": "review-42", "agc.sandbox": "review-42"}) {
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

	// /home/agent is a private disk of the default size; the harness homes
	// are on the named volume agc-<sandbox>, recorded on the row.
	if created.DiskDest != "/home/agent" || created.DiskMiB != 0 {
		t.Fatalf("disk = %s %d", created.DiskDest, created.DiskMiB)
	}
	harness := sandbox.Volume{Source: "agc-review-42", Dest: "/home/agent/.harness", ReadWrite: true, QuotaMiB: 4096}
	if len(created.Volumes) != 1 || created.Volumes[0] != harness {
		t.Fatalf("volumes = %+v", created.Volumes)
	}
	if view.Volume != "agc-review-42" || got.Volume != "agc-review-42" {
		t.Fatalf("volume = %q, %q", view.Volume, got.Volume)
	}
}

// A deleted sandbox's volume stays on the host with its transcripts, so its
// name is not reused: the next sandbox must not inherit them.
func TestAllocateSkipsNamesWithAVolumeLeft(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "fix")

	err := os.MkdirAll(filepath.Join(env.api.volumeRoot, "agc-fix", ".claude"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	view, err := env.api.Allocate(context.Background(), nil, &AllocateRequest{TaskName: "fix", Sandbox: task.Sandbox{Spec: task.Spec{Disk: "8G"}}})
	if err != nil {
		t.Fatal(err)
	}
	if view.Name != "fix-2" || view.Volume != "agc-fix-2" || view.Spec.Disk != "8G" {
		t.Fatalf("view = %+v", view)
	}

	env.api.Wait()
	if env.rt.created[0].DiskMiB != 8192 {
		t.Fatalf("disk = %d MiB", env.rt.created[0].DiskMiB)
	}
}

func TestAllocateRejectsSmallDisk(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "fix")

	_, err := env.api.Allocate(context.Background(), nil, &AllocateRequest{TaskName: "fix", Sandbox: task.Sandbox{Spec: task.Spec{Disk: "1G"}}})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "minimum is 2G") {
		t.Fatalf("err = %v", err)
	}
	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestAllocateNamesLaterSandboxesAfterTheTask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "fix")

	names := []string{}
	for i := 0; i < 3; i++ {
		view, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "fix", Sandbox: task.Sandbox{}})
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, view.Name)
	}

	if !reflect.DeepEqual(names, []string{"fix", "fix-2", "fix-3"}) {
		t.Fatalf("names = %q", names)
	}

	env.api.Wait()

	// Boots run concurrently, so created is in no particular order.
	var created sandbox.Spec
	for _, spec := range env.rt.created {
		if spec.Name == "fix-3" {
			created = spec
		}
	}
	if created.Labels["agc.task"] != "fix" || created.Labels["agc.sandbox"] != "fix-3" || created.Env["AGC_TASK"] != "fix" {
		t.Fatalf("third sandbox = %+v", created)
	}
	if created.VsockPath != env.api.socketPath("fix-3") {
		t.Fatalf("vsock path = %s", created.VsockPath)
	}
}

func TestAllocateWithRef(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "ref-task")

	sb := task.Sandbox{Spec: task.Spec{Repo: "github.com/o/n", Ref: "v1.2.0"}}
	_, err := env.api.Allocate(context.Background(), nil, &AllocateRequest{TaskName: "ref-task", Sandbox: sb})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	calls := env.rt.Calls()
	if calls[4] != "run git -C /home/agent/n checkout v1.2.0 in " {
		t.Fatalf("calls = %q", calls)
	}
}

func TestAllocateWithoutRepoTrustsHome(t *testing.T) {
	env := newTestEnv(t)
	insertTask(t, env.db, "scratch")

	_, err := env.api.Allocate(context.Background(), nil, &AllocateRequest{TaskName: "scratch", Sandbox: task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	calls := env.rt.Calls()
	if !strings.HasPrefix(calls[3], "run bash -c ") || !strings.Contains(calls[3], "dir='/home/agent'\n") {
		t.Fatalf("calls = %q", calls)
	}
}

func TestAgentConfigScript(t *testing.T) {
	cases := []struct {
		dir     string
		wantErr bool
	}{
		{"/home/agent/agent-composer", false},
		{"/home/agent", false},
		{"/home/agent/my repo.v2", false},
		{`/home/agent/a"b`, true},
		{"/home/agent/a'b", true},
		{`/home/agent/a\b`, true},
		{"/home/agent/a\nb", true},
		{"relative/repo", true},
	}

	for _, tc := range cases {
		script, err := agentConfigScript(tc.dir)
		if tc.wantErr {
			if ez.ErrorCode(err) != ez.EINVALID {
				t.Errorf("%q: expected EINVALID, got %v", tc.dir, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.dir, err)
			continue
		}

		wants := []string{
			"dir='" + tc.dir + "'\n",
			".hasCompletedOnboarding = true",
			`.theme = "dark"`,
			".bypassPermissionsModeAccepted = true",
			".projects[$dir].hasTrustDialogAccepted = true",
			".skipDangerousModePermissionPrompt = true",
			`echo "[projects.\"$dir\"]"`,
			`echo 'trust_level = "trusted"'`,
		}
		for _, want := range wants {
			if !strings.Contains(script, want) {
				t.Errorf("%q: script lacks %q", tc.dir, want)
			}
		}
		if strings.Contains(script, "{{dir}}") {
			t.Errorf("%q: placeholder left in script", tc.dir)
		}
	}
}

func TestAllocateMissingSecret(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	sb := task.Sandbox{Spec: task.Spec{Secrets: []string{"OPENAI_API_KEY"}}}
	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: sb})
	if ez.ErrorCode(err) != ez.ENOTFOUND || !strings.Contains(ez.ErrorMessage(err), "OPENAI_API_KEY") {
		t.Fatalf("expected ENOTFOUND naming the secret, got %v", err)
	}

	if len(env.rt.Calls()) != 0 {
		t.Fatalf("runtime was called: %q", env.rt.Calls())
	}

	_, err = taskmodels.GetSandboxByName(ctx, env.db, "review-42")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected no row, got %v", err)
	}
}

func TestAllocateCloneFails(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.rt.runFailOn = "clone"
	insertTask(t, env.db, "review-42")

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: testSandbox()})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	view, err := env.api.Get(ctx, nil, &GetRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}

	if view.Status != "failed" || view.BootedAt != nil {
		t.Fatalf("view = %+v", view)
	}
	if !strings.Contains(view.StatusError, "fatal: repository not found") || !strings.Contains(view.StatusError, "sandbox review-42 was removed") {
		t.Fatalf("status_error = %q", view.StatusError)
	}

	wantCalls := []string{
		"create review-42",
		"run chown agent:agent /home/agent in  as root",
		"run bash -c " + homeScript + " in ",
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

	_, err = env.api.Stop(ctx, nil, &StopRequest{Name: "review-42"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("expected ECONFLICT stopping a failed sandbox, got %v", err)
	}
}

func TestStaleStartingSandboxFails(t *testing.T) {
	env := newTestEnv(t)
	owner := insertTask(t, env.db, "stale")

	record := &taskmodels.Sandbox{TaskID: owner.ID, Name: "stale", Spec: []byte(`{}`), CreatedAt: time.Now().Add(-2 * time.Hour)}
	err := record.Insert(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}

	view, err := env.api.Get(context.Background(), nil, &GetRequest{ID: record.ID})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "failed" || !strings.Contains(view.StatusError, "did not finish") {
		t.Fatalf("view = %+v", view)
	}
}

func TestList(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now().UTC()
	owner := insertTask(t, env.db, "owner")

	insertBooted(t, env.db, owner, "old", now.Add(-2*time.Hour))
	insertBooted(t, env.db, owner, "mid", now.Add(-time.Hour))
	insertBooted(t, env.db, owner, "new", now)
	env.rt.statuses["old"] = "stopped"
	env.rt.statuses["mid"] = "crashed"
	env.rt.failing["new"] = true

	list, err := env.api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	views := list.Sandboxes

	got := []string{}
	for _, view := range views {
		got = append(got, view.Task+"/"+view.Name+"="+view.Status+"/"+view.StatusError)
	}

	want := []string{"owner/new=unknown/msb is not installed", "owner/mid=crashed/", "owner/old=stopped/"}
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
	insertTask(t, env.db, "review-42")

	created, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: testSandbox()})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	socket := env.api.socketPath("review-42")
	before := len(env.rt.Calls())

	view, err := env.api.Stop(ctx, nil, &StopRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "stopped" || view.StoppedAt == nil || socketExists(t, socket) {
		t.Fatalf("after stop: %+v, socket exists %v", view, socketExists(t, socket))
	}

	err = os.WriteFile(env.api.secretsPath, []byte("GITHUB_TOKEN=rotated\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	view, err = env.api.Start(ctx, nil, &StartRequest{ID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" || view.StoppedAt != nil || !socketExists(t, socket) {
		t.Fatalf("after start: %+v, socket exists %v", view, socketExists(t, socket))
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

	wantCalls := []string{"stop review-42", "start review-42", "terminal claude --dangerously-skip-permissions in /home/agent/agent-composer", "detach", "stop review-42", "remove review-42"}
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
	owner := insertTask(t, env.db, "owner")
	insertBooted(t, env.db, owner, "saved", time.Now())
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
	owner := insertTask(t, env.db, "owner")
	insertBooted(t, env.db, owner, "gone", time.Now())

	_, err := env.api.Delete(context.Background(), nil, &DeleteRequest{Name: "gone"})
	if err != nil {
		t.Fatal(err)
	}

	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestDeleteCancelsBoot(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.rt.blockCreate = true
	insertTask(t, env.db, "slow")

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "slow", Sandbox: task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for len(env.rt.Calls()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	_, err = env.api.Delete(ctx, nil, &DeleteRequest{Name: "slow"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = taskmodels.GetSandboxByName(ctx, env.db, "slow")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected the row gone, got %v", err)
	}

	if socketExists(t, env.api.socketPath("slow")) {
		t.Fatal("listener socket still exists")
	}
}

func TestResume(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	owner := insertTask(t, env.db, "owner")

	insertBooted(t, env.db, owner, "up", now)
	insertBooted(t, env.db, owner, "down", now)
	insertBooted(t, env.db, owner, "broken", now)
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
	insertTask(t, env.db, "review-42")

	sb := task.Sandbox{Spec: task.Spec{Channels: []string{"#alerts:read"}}}
	for i := 0; i < 2; i++ {
		_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42", Sandbox: sb})
		if err != nil {
			t.Fatal(err)
		}
	}
	env.api.Wait()

	// The second sandbox has its own socket and speaks for the task.
	client := socketClient(env.api.socketPath("review-42-2"))

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

func TestListenerServesDeliverForItsOwnTask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	insertTask(t, env.db, "review-42")

	_, err := env.api.Allocate(ctx, nil, &AllocateRequest{TaskName: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()

	client := socketClient(env.api.socketPath("review-42"))

	out := map[string]string{}
	err = client.Deliver(ctx, "posted the findings", nil, nil, &out)
	if ez.ErrorCode(err) != ez.EUNAVAILABLE {
		t.Fatalf("expected EUNAVAILABLE before OnDeliver, got %v", err)
	}

	env.api.OnDeliver(func(ctx context.Context, taskName string, summary string, report json.RawMessage, files map[string][]byte) (any, error) {
		if summary == "" {
			return nil, ez.New(ez.EINVALID, "summary is required", nil)
		}
		return map[string]string{"task": taskName, "summary": summary, "report": string(report), "after": fmt.Sprint(len(files["after"]))}, nil
	})

	err = client.Deliver(ctx, "posted the findings", nil, nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out["task"] != "review-42" || out["summary"] != "posted the findings" || out["report"] != "" {
		t.Fatalf("delivered = %+v", out)
	}

	// The report reaches the tasks API as it was sent.
	report := `{"criteria":[{"met":"partly","note":"x"}],"extra":{"kept":true}}`
	err = client.Deliver(ctx, "with a report", json.RawMessage(report), nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out["report"] != report {
		t.Fatalf("report = %q, want %q", out["report"], report)
	}

	// Shots come with it, larger than a messaging body may be.
	err = client.Deliver(ctx, "with shots", json.RawMessage(report), map[string][]byte{"after": make([]byte, 3<<20)}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out["after"] != fmt.Sprint(3<<20) {
		t.Fatalf("after = %q bytes", out["after"])
	}

	err = client.Deliver(ctx, "", nil, nil, &out)
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("expected EINVALID, got %v", err)
	}

	// The messaging routes still answer on the same socket.
	_, err = client.Join(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
}

func TestStartListenerLeavesASocketServedElsewhere(t *testing.T) {
	env := newTestEnv(t)
	other := New(context.Background(), env.db, env.api.msg, env.rt, env.api.secretsPath, env.api.socketDir)

	caller := func(*http.Request) (messaging.Caller, error) {
		return messaging.Caller{Task: "shared"}, nil
	}
	routes := func(*http.ServeMux) {}

	err := env.api.startListener("shared", caller, routes)
	if err != nil {
		t.Fatal(err)
	}

	err = other.startListener("shared", caller, routes)
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

func TestStartRelaunchesTheJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	owner := insertTask(t, env.db, "review-42")

	withCommand := insertBooted(t, env.db, owner, "review-42", time.Now().UTC())
	withCommand.Command = "agc agent run"
	err := withCommand.Update(ctx, env.db, "command")
	if err != nil {
		t.Fatal(err)
	}
	insertBooted(t, env.db, owner, "review-42-2", time.Now().UTC())

	for _, name := range []string{"review-42", "review-42-2"} {
		_, err = env.api.Stop(ctx, nil, &StopRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
	}

	terminals := func() []string {
		calls := []string{}
		for _, call := range env.rt.Calls() {
			if strings.HasPrefix(call, "terminal ") {
				calls = append(calls, call)
			}
		}
		return calls
	}

	_, err = env.api.Start(ctx, nil, &StartRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"terminal agc agent run in /home/agent"}
	if !reflect.DeepEqual(terminals(), want) {
		t.Fatalf("terminals after start = %q, want %q", terminals(), want)
	}

	// A start on a VM whose job still runs does not launch it again.
	_, err = env.api.Start(ctx, nil, &StartRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(terminals(), want) {
		t.Fatalf("terminals after a second start = %q, want %q", terminals(), want)
	}

	_, err = env.api.Start(ctx, nil, &StartRequest{Name: "review-42-2"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(terminals(), want) {
		t.Fatalf("a sandbox with no command launched a job: %q", terminals())
	}
}

// A persistent agent's home keeps the agent's volume agc-<agent> across
// homes (ADR 0016 item 5): after the first home is deleted, its volume is
// still on the host, so the next home gets a free sandbox name and the
// same volume, mounted as the harness homes.
func TestAllocateHomeAfterDeleteKeepsTheAgentsVolume(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	request := HomeRequest{Agent: "reviewer-1", Command: "agc agent run --agent reviewer-1"}

	first, err := env.api.AllocateHome(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()
	if first.Name != "reviewer-1" || first.Volume != "agc-reviewer-1" {
		t.Fatalf("first = %s %s", first.Name, first.Volume)
	}

	_, err = env.api.Delete(ctx, nil, &DeleteRequest{Name: first.Name})
	if err != nil {
		t.Fatal(err)
	}

	// msb keeps the deleted home's volume on the host.
	err = os.MkdirAll(filepath.Join(env.api.volumeRoot, "agc-reviewer-1", ".claude"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	second, err := env.api.AllocateHome(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	env.api.Wait()
	if second.Name != "reviewer-1-2" || second.Volume != "agc-reviewer-1" {
		t.Fatalf("second = %s %s", second.Name, second.Volume)
	}

	got, err := taskmodels.GetSandboxByName(ctx, env.db, second.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Volume != "agc-reviewer-1" {
		t.Fatalf("row volume = %q", got.Volume)
	}

	created := env.rt.created[len(env.rt.created)-1]
	harness := sandbox.Volume{Source: "agc-reviewer-1", Dest: "/home/agent/.harness", ReadWrite: true, QuotaMiB: 4096}
	if created.Name != "reviewer-1-2" || len(created.Volumes) != 1 || created.Volumes[0] != harness {
		t.Fatalf("created = %s %+v", created.Name, created.Volumes)
	}
}
