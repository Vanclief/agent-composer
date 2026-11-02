package agents

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	msgclient "github.com/vanclief/agent-composer/messaging/client"
	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// fakeRuntime boots, starts and stops every sandbox at once and records
// the calls. A non-nil gate holds every Create until it is closed.
type fakeRuntime struct {
	mu       sync.Mutex
	statuses map[string]string
	specs    map[string]sandbox.Spec
	calls    []string
	gate     chan struct{}
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

func (f *fakeRuntime) set(name string, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses[name] = status
}

func (f *fakeRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	f.record("create " + spec.Name)
	if f.gate != nil {
		<-f.gate
	}
	f.mu.Lock()
	f.statuses[spec.Name] = "running"
	f.specs[spec.Name] = spec
	f.mu.Unlock()
	return &fakeSandbox{rt: f}, nil
}

func (f *fakeRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	return &fakeSandbox{rt: f}, nil
}

func (f *fakeRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	f.record("start " + name)
	f.set(name, "running")
	return nil
}

func (f *fakeRuntime) Stop(ctx context.Context, name string) error {
	f.record("stop " + name)
	f.set(name, "stopped")
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
	status, ok := f.statuses[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}
	return status, nil
}

// fakeSandbox succeeds at every boot step and records the files written
// and the terminal started. Other methods panic on the nil embedded
// interface, which no test reaches.
type fakeSandbox struct {
	executor.TerminalExecutor
	rt *fakeRuntime
}

func (s *fakeSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	return &executor.Result{}, nil
}

func (s *fakeSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	s.rt.record("write " + path)
	return nil
}

func (s *fakeSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	s.rt.record("terminal " + strings.Join(append([]string{cmd.Name}, cmd.Args...), " "))
	return &executor.Terminal{ID: "job_1", Running: true}, nil
}

func (s *fakeSandbox) Detach(ctx context.Context) error {
	return nil
}

type testEnv struct {
	api   *API
	tasks *tasks.API
	sbx   *sandboxes.API
	rt    *fakeRuntime
	db    *bun.DB
	msg   *messaging.Service
	bus   *events.Bus
}

const testSandbox = `sandbox:
  image: agc-agent
  disk: 4G
`

// templates are the roles the tests use: reviewer reads nothing extra,
// watcher reads #ci-*, main reads every channel.
var templates = map[string]string{
	"reviewer.yaml": "harness: claude_code\nidle: 20m\nreads: []\n" + testSandbox + "instructions: |\n  Review.\n",
	"watcher.yaml":  "harness: codex_cli\nmodel: gpt-5\nreads: ['#ci-*']\n" + testSandbox + "instructions: Watch CI.\n",
	"main.yaml":     "harness: claude_code\nreads: ['*']\n" + testSandbox + "instructions: Route.\n",
}

func newTestEnv(t *testing.T) *testEnv {
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
		(*agent.Agent)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	dir := t.TempDir()
	for name, body := range templates {
		err = os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Unix socket paths are capped near 104 bytes, too short for
	// t.TempDir() on macOS.
	socketDir, err := os.MkdirTemp("", "agc")
	if err != nil {
		t.Fatal(err)
	}

	rt := &fakeRuntime{statuses: map[string]string{}, specs: map[string]sandbox.Spec{}}
	bus := events.New()
	msg := messaging.NewService(db)
	t.Setenv(sandboxes.EnvVolumeRoot, t.TempDir())
	sandboxesAPI := sandboxes.New(context.Background(), db, msg, rt, "", socketDir)
	defaults := controller.DefaultsSettings{Harness: "claude_code", Model: "claude-opus-5-5", Owner: "franco"}
	tasksAPI := tasks.New(db, sandboxesAPI, msg, defaults, "", task.Store{Dir: filepath.Join(t.TempDir(), "specs")})
	tasksAPI.SetEvents(bus)
	api := New(db, tasksAPI, msg, defaults, dir)
	api.SetEvents(bus)

	t.Cleanup(func() {
		api.Wait()
		sandboxesAPI.Wait()
		_ = os.RemoveAll(socketDir)
	})

	return &testEnv{api: api, tasks: tasksAPI, sbx: sandboxesAPI, rt: rt, db: db, msg: msg, bus: bus}
}

func (env *testEnv) create(t *testing.T, role string) *AgentView {
	t.Helper()

	view, err := env.api.Create(context.Background(), nil, &CreateRequest{Role: role})
	if err != nil {
		t.Fatal(err)
	}

	return view
}

func (env *testEnv) get(t *testing.T, name string) *AgentView {
	t.Helper()

	view, err := env.api.Get(context.Background(), nil, &GetRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}

	return view
}

// wake wakes name and waits for its home to boot.
func (env *testEnv) wake(t *testing.T, name string) *AgentView {
	t.Helper()

	_, err := env.api.Wake(context.Background(), nil, &WakeRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	env.sbx.Wait()

	return env.get(t, name)
}

func TestCreateNumbersPerRoleAndNeverReuses(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	first := env.create(t, "reviewer")
	if first.Name != "reviewer-1" || first.Role != "reviewer" || first.Kept || first.State != StateAsleep || first.Home != nil {
		t.Fatalf("first = %+v", first)
	}
	if first.Harness != "claude_code" || first.Model != "claude-opus-5-5" || first.Owner != "franco" {
		t.Fatalf("first harness, model, owner = %s %s %s, want the defaults", first.Harness, first.Model, first.Owner)
	}
	if !reflect.DeepEqual(first.Tasks, []AgentTask{}) {
		t.Fatalf("tasks = %#v, want empty and not nil", first.Tasks)
	}
	if len(first.Conversations) != 1 || first.Conversations[0].Channel != "@franco/reviewer-1" || first.LastActiveAt != nil {
		t.Fatalf("conversations = %+v, last_active_at = %v: want the DM only, never active", first.Conversations, first.LastActiveAt)
	}

	// The role is case-insensitive and keeps the case it was typed in.
	second := env.create(t, "Reviewer")
	if second.Name != "reviewer-2" || second.Role != "Reviewer" {
		t.Fatalf("second = %+v", second)
	}

	// Each role counts on its own; the template's harness and model win
	// over the defaults.
	watcher := env.create(t, "watcher")
	if watcher.Name != "watcher-1" || watcher.Harness != "codex_cli" || watcher.Model != "gpt-5" {
		t.Fatalf("watcher = %+v", watcher)
	}

	// A kept agent keeps its number, so reviewer-2 is never handed out
	// again.
	_, err := env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-2", Name: "ada"})
	if err != nil {
		t.Fatal(err)
	}

	third := env.create(t, "reviewer")
	if third.Name != "reviewer-3" {
		t.Fatalf("third = %s, want reviewer-3", third.Name)
	}

	// A name kept as another agent's is skipped.
	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-3", Name: "reviewer-4"})
	if err != nil {
		t.Fatal(err)
	}

	fifth := env.create(t, "reviewer")
	if fifth.Name != "reviewer-5" {
		t.Fatalf("fifth = %s, want reviewer-5", fifth.Name)
	}

	// A kept agent that changes role keeps its number in the role it was
	// born in.
	env.create(t, "main")
	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "main-1", Name: "boss", Role: "reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	name := env.create(t, "main").Name
	if name != "main-2" {
		t.Fatalf("main after a role change = %s, want main-2", name)
	}
	name = env.create(t, "reviewer").Name
	if name != "reviewer-6" {
		t.Fatalf("reviewer after a role change = %s, want reviewer-6", name)
	}

	// The request's harness wins; a harness unlike the template's takes
	// no model from it.
	codex, err := env.api.Create(ctx, nil, &CreateRequest{Role: "reviewer", Harness: "codex_cli"})
	if err != nil {
		t.Fatal(err)
	}
	if codex.Harness != "codex_cli" || codex.Model != "" {
		t.Fatalf("codex = %+v", codex)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Role: "writer"})
	if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), "writer.yaml") {
		t.Fatalf("unknown role: %v, want EINVALID naming writer.yaml", err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Role: "../main"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("role with a path: %v, want EINVALID", err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Role: "reviewer", Harness: "vim"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("unknown harness: %v, want EINVALID", err)
	}
}

func TestKeepRenamesAndRefusesTakenNames(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	env.create(t, "reviewer")
	env.create(t, "reviewer")

	_, err := env.tasks.Create(ctx, nil, &tasks.CreateRequest{Title: "Review 42", Brief: "Review it.", Manager: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}

	woken := env.wake(t, "reviewer-1")
	if woken.Home == nil || *woken.Home != "reviewer-1" {
		t.Fatalf("home = %v, want reviewer-1", woken.Home)
	}

	kept, err := env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-1", Name: "ada", Role: "watcher"})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Name != "ada" || !kept.Kept || kept.Role != "watcher" {
		t.Fatalf("kept = %+v", kept)
	}
	// The home keeps its name; the tasks follow the agent.
	if kept.Home == nil || *kept.Home != "reviewer-1" || kept.State != StateAwake {
		t.Fatalf("kept home = %v, state %s", kept.Home, kept.State)
	}
	if !reflect.DeepEqual(kept.Tasks, []AgentTask{{Name: "review-42", Status: "open"}}) {
		t.Fatalf("kept tasks = %+v", kept.Tasks)
	}
	if kept.Conversations[0].Channel != "@franco/ada" {
		t.Fatalf("conversations = %+v, want the DM under the new name first", kept.Conversations)
	}

	home, err := env.sbx.Home(ctx, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if home.Agent != "ada" || home.Task != "" {
		t.Fatalf("home agent = %q, task = %q, want ada and none", home.Agent, home.Task)
	}

	_, err = env.api.Get(ctx, nil, &GetRequest{Name: "reviewer-1"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("old name: %v, want ENOTFOUND", err)
	}

	// Renaming a kept agent again, and to its own name, both work.
	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "ada", Name: "ada"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-2", Name: "ada"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("taken name: %v, want ECONFLICT", err)
	}

	for _, bad := range []string{"Ada", "ada!", "-ada", "ada--b", "", strings.Repeat("a", 33)} {
		_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-2", Name: bad})
		if ez.ErrorCode(err) != ez.EINVALID {
			t.Fatalf("name %q: %v, want EINVALID", bad, err)
		}
	}

	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "reviewer-2", Name: "bob", Role: "writer"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("role without a template: %v, want EINVALID", err)
	}

	_, err = env.api.Keep(ctx, nil, &KeepRequest{Agent: "nobody", Name: "bob"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("unknown agent: %v, want ENOTFOUND", err)
	}
}

func TestStateFollowsTheHomeSandbox(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "reviewer")

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	updates := env.bus.Subscribe(subCtx)

	// The first wake allocates the home and answers waking while it boots.
	env.rt.gate = make(chan struct{})
	waking, err := env.api.Wake(ctx, nil, &WakeRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if waking.State != StateWaking || waking.Home == nil || *waking.Home != "reviewer-1" || waking.LastActiveAt == nil {
		t.Fatalf("waking = %+v", waking)
	}

	// Waking again while it boots does nothing.
	_, err = env.api.Wake(ctx, nil, &WakeRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	close(env.rt.gate)
	env.sbx.Wait()
	env.rt.gate = nil

	awake := env.get(t, "reviewer-1")
	if awake.State != StateAwake {
		t.Fatalf("state = %s, want awake", awake.State)
	}

	spec := env.rt.specs["reviewer-1"]
	if spec.Env[sandboxes.EnvAgent] != "reviewer-1" || spec.Env[task.EnvTask] != "" || spec.DiskMiB != 4096 {
		t.Fatalf("home spec env = %v, disk %d MiB", spec.Env, spec.DiskMiB)
	}

	wantCalls := []string{
		"create reviewer-1",
		"write " + sandboxes.AgentBriefPath,
		"terminal /usr/local/bin/agc agent run --agent reviewer-1 --harness claude_code --model claude-opus-5-5 --brief /home/agent/AGENT.md",
	}
	if !reflect.DeepEqual(env.rt.Calls(), wantCalls) {
		t.Fatalf("calls = %q, want %q", env.rt.Calls(), wantCalls)
	}

	// The home is a sandbox with no task, linked to its agent.
	list, err := env.sbx.List(ctx, nil, &sandboxes.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Sandboxes) != 1 || list.Sandboxes[0].Agent != "reviewer-1" || list.Sandboxes[0].Task != "" || list.Sandboxes[0].Instructions != "Review.\n" {
		t.Fatalf("sandboxes = %+v", list.Sandboxes)
	}

	// Waking an awake agent does nothing.
	env.wake(t, "reviewer-1")
	if len(env.rt.Calls()) != len(wantCalls) {
		t.Fatalf("wake on an awake agent called the runtime: %q", env.rt.Calls())
	}

	asleep, err := env.api.Sleep(ctx, nil, &SleepRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if asleep.State != StateAsleep || asleep.Home == nil {
		t.Fatalf("asleep = %+v", asleep)
	}

	_, err = env.api.Sleep(ctx, nil, &SleepRequest{Name: "reviewer-1"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("sleep while asleep: %v, want ECONFLICT", err)
	}

	// A stopped home is started, not allocated again.
	again := env.wake(t, "reviewer-1")
	if again.State != StateAwake {
		t.Fatalf("state = %s, want awake", again.State)
	}
	calls := env.rt.Calls()
	if calls[len(calls)-1] != "start reviewer-1" {
		t.Fatalf("calls = %q, want a start last", calls)
	}

	// Any other status of the home is the agent's state, verbatim.
	env.rt.set("reviewer-1", "stopping")
	state := env.get(t, "reviewer-1").State
	if state != "stopping" {
		t.Fatalf("state = %s, want stopping", state)
	}

	_, err = env.api.Wake(ctx, nil, &WakeRequest{Name: "reviewer-1"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("wake while stopping: %v, want ECONFLICT", err)
	}

	// Every state change was published as agent.state.
	states := []string{}
	for len(updates) > 0 {
		event := <-updates
		if event.Type != events.AgentState {
			continue
		}
		states = append(states, event.Data.(StateEvent).Agent.State)
	}
	want := []string{StateWaking, StateAwake, StateAsleep, StateAwake}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("agent.state = %q, want %q", states, want)
	}
}

func TestFailedHomeIsShownAndAllocatedAgain(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "reviewer")
	env.wake(t, "reviewer-1")

	record, err := taskmodels.GetSandboxByName(ctx, env.db, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	msg := "boom"
	record.BootError = &msg
	err = record.Update(ctx, env.db, "boot_error")
	if err != nil {
		t.Fatal(err)
	}

	state := env.get(t, "reviewer-1").State
	if state != sandboxes.StatusFailed {
		t.Fatalf("state = %s, want failed", state)
	}

	// The volume of the failed home stays, so the new home takes the next
	// free name.
	err = os.MkdirAll(filepath.Join(os.Getenv(sandboxes.EnvVolumeRoot), task.VolumeName("reviewer-1")), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	view := env.wake(t, "reviewer-1")
	if view.State != StateAwake || view.Home == nil || *view.Home != "reviewer-1-2" {
		t.Fatalf("view = %+v", view)
	}
}

func TestDeleteRemovesTheHomeAndKeepsTheVolume(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "reviewer")
	env.create(t, "reviewer")
	env.wake(t, "reviewer-1")

	deleted, err := env.api.Delete(ctx, nil, &DeleteRequest{Name: "reviewer-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Deleted || deleted.Name != "reviewer-1" {
		t.Fatalf("deleted = %+v", deleted)
	}

	_, err = taskmodels.GetSandboxByName(ctx, env.db, "reviewer-1")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("home after delete: %v, want ENOTFOUND", err)
	}

	calls := env.rt.Calls()
	if calls[len(calls)-1] != "remove reviewer-1" {
		t.Fatalf("calls = %q, want the home removed", calls)
	}

	// An agent that never woke has no home to delete.
	_, err = env.api.Delete(ctx, nil, &DeleteRequest{Name: "reviewer-2"})
	if err != nil {
		t.Fatal(err)
	}

	list, err := env.api.List(ctx, nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Agents) != 0 {
		t.Fatalf("agents = %+v, want none", list.Agents)
	}

	_, err = env.api.Delete(ctx, nil, &DeleteRequest{Name: "reviewer-1"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("delete twice: %v, want ENOTFOUND", err)
	}
}

// postAs joins name with caller and posts body in channel.
func (env *testEnv) postAs(t *testing.T, caller messaging.Caller, name string, channel string, body string) {
	t.Helper()
	ctx := context.Background()

	p, err := env.msg.Join(ctx, caller, name)
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.msg.Post(ctx, caller, p.ID, channel, body, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	env.api.Wait()
	env.sbx.Wait()
}

func TestChannelsOfAnAgent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "watcher")

	for _, title := range []string{"Fix CI", "Bump deps"} {
		_, err := env.tasks.Create(ctx, nil, &tasks.CreateRequest{Title: title, Brief: "Do it.", Manager: "watcher-1"})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := env.tasks.Create(ctx, nil, &tasks.CreateRequest{Title: "Not mine", Brief: "Do it.", Manager: "franco"})
	if err != nil {
		t.Fatal(err)
	}

	for _, channel := range []string{"#ci-main", "#ci-nightly", "#general"} {
		_, err = env.msg.PostAsSystem(ctx, channel, "hello")
		if err != nil {
			t.Fatal(err)
		}
	}

	record, err := agent.GetAgentByName(ctx, env.db, "watcher-1")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := LoadTemplate(env.api.dir, record.Role)
	if err != nil {
		t.Fatal(err)
	}

	reads, err := env.api.Reads(ctx, record, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	wantReads := []string{"@franco/watcher-1", "#fix-ci", "#bump-deps", "#ci-*"}
	if !reflect.DeepEqual(reads, wantReads) {
		t.Fatalf("reads = %q, want %q", reads, wantReads)
	}

	channels, err := env.api.Channels(ctx, record, tmpl)
	if err != nil {
		t.Fatal(err)
	}
	wantChannels := []string{"@franco/watcher-1", "#fix-ci", "#bump-deps", "#ci-main", "#ci-nightly"}
	if !reflect.DeepEqual(channels, wantChannels) {
		t.Fatalf("channels = %q, want %q", channels, wantChannels)
	}

	view := env.get(t, "watcher-1")
	wantTasks := []AgentTask{{Name: "fix-ci", Status: "open"}, {Name: "bump-deps", Status: "open"}}
	if !reflect.DeepEqual(view.Tasks, wantTasks) {
		t.Fatalf("tasks = %+v, want %+v", view.Tasks, wantTasks)
	}

	// Conversations: the DM, then the tasks' channels (each with the
	// opening agc posted), then where the agent posted itself.
	got := []string{}
	for _, c := range view.Conversations {
		got = append(got, c.Channel)
	}
	if !reflect.DeepEqual(got, []string{"@franco/watcher-1", "#fix-ci", "#bump-deps"}) {
		t.Fatalf("conversations = %q", got)
	}
	if view.LastActiveAt == nil || !view.LastActiveAt.Equal(view.Conversations[2].LastAt) {
		t.Fatalf("last_active_at = %v, want the newest message, %v", view.LastActiveAt, view.Conversations[2].LastAt)
	}

	env.wake(t, "watcher-1")
	env.postAs(t, messaging.Caller{Agent: "watcher-1", Grants: []messaging.Grant{{Channel: "#ci-main"}}}, "watcher-1", "#ci-main", "green")

	view = env.get(t, "watcher-1")
	last := view.Conversations[len(view.Conversations)-1]
	if last.Channel != "#ci-main" || !view.LastActiveAt.Equal(last.LastAt) {
		t.Fatalf("conversations = %+v, last_active_at = %v", view.Conversations, view.LastActiveAt)
	}

	if !MatchChannel("*", "@franco/x") || MatchChannel("#ci-*", "#cd-main") || !MatchChannel("#fix-ci", "#fix-ci") {
		t.Fatal("MatchChannel")
	}
}

// woken returns which of names the runtime created or started a home for.
func (env *testEnv) woken(names ...string) []string {
	out := []string{}
	for _, name := range names {
		for _, call := range env.rt.Calls() {
			if call == "create "+name || call == "start "+name {
				out = append(out, name)
				break
			}
		}
	}

	return out
}

func TestWakeHookWakesOnlyMatchingAsleepAgents(t *testing.T) {
	ctx := context.Background()
	task42 := messaging.Caller{Task: "review-42"}

	t.Run("a message in a channel an agent reads", func(t *testing.T) {
		env := newTestEnv(t)
		env.create(t, "reviewer")
		env.create(t, "watcher")

		env.postAs(t, messaging.Caller{Task: "ci-main"}, "builder", "#ci-main", "red")

		woken := env.woken("reviewer-1", "watcher-1")
		if !reflect.DeepEqual(woken, []string{"watcher-1"}) {
			t.Fatalf("woken = %q, want watcher-1 only", woken)
		}
	})

	t.Run("a message in an agent's task channel", func(t *testing.T) {
		env := newTestEnv(t)
		env.create(t, "reviewer")
		env.create(t, "reviewer")

		_, err := env.tasks.Create(ctx, nil, &tasks.CreateRequest{Name: "review-42", Brief: "Review it.", Manager: "reviewer-2"})
		if err != nil {
			t.Fatal(err)
		}
		// Creating the task posted agc's opening in #review-42, which
		// wakes nobody.
		env.api.Wait()
		woken := env.woken("reviewer-1", "reviewer-2")
		if len(woken) != 0 {
			t.Fatalf("agc's post woke %q", woken)
		}

		env.postAs(t, task42, "review-42", "#review-42", "QUESTION: which branch?")

		woken = env.woken("reviewer-1", "reviewer-2")
		if !reflect.DeepEqual(woken, []string{"reviewer-2"}) {
			t.Fatalf("woken = %q, want reviewer-2 only", woken)
		}
	})

	t.Run("agc's posts and an agent's own wake nobody", func(t *testing.T) {
		env := newTestEnv(t)
		env.create(t, "watcher")
		env.wake(t, "watcher-1")
		_, err := env.api.Sleep(ctx, nil, &SleepRequest{Name: "watcher-1"})
		if err != nil {
			t.Fatal(err)
		}
		before := len(env.rt.Calls())

		_, err = env.msg.PostAsSystem(ctx, "#ci-main", "Accepted by franco.")
		if err != nil {
			t.Fatal(err)
		}
		env.api.Wait()

		env.postAs(t, messaging.Caller{Agent: "watcher-1", Grants: []messaging.Grant{{Channel: "#ci-main"}}}, "watcher-1", "#ci-main", "still red")

		calls := env.rt.Calls()
		if len(calls) != before {
			t.Fatalf("calls after = %q, want none past %d", calls, before)
		}

		// Another agent's post does wake it.
		env.create(t, "reviewer")
		env.wake(t, "reviewer-1")
		env.postAs(t, messaging.Caller{Agent: "reviewer-1", Grants: []messaging.Grant{{Channel: "#ci-main"}}}, "reviewer-1", "#ci-main", "looked at it")

		calls = env.rt.Calls()
		if calls[len(calls)-1] != "start watcher-1" {
			t.Fatalf("calls = %q, want watcher-1 started", calls)
		}
	})

	t.Run("awake agents are left to their loop", func(t *testing.T) {
		env := newTestEnv(t)
		env.create(t, "main")
		env.wake(t, "main-1")
		before := len(env.rt.Calls())

		env.postAs(t, messaging.Caller{Human: true}, "franco", "#anything", "hi")

		calls := env.rt.Calls()
		if len(calls) != before {
			t.Fatalf("calls = %q, want none past %d", calls, before)
		}
	})
}

// homeClient talks to a home's socket as its VM would.
func homeClient(env *testEnv, home string) *msgclient.Client {
	path := filepath.Join(env.sbx.SocketDir(), home+".sock")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", path)
		},
	}

	return &msgclient.Client{BaseURL: "http://home", HTTP: &http.Client{Transport: transport}}
}

func TestHomeSocketServesTheAgent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "reviewer")
	env.wake(t, "reviewer-1")
	client := homeClient(env, "reviewer-1")

	created := &tasks.TaskView{}
	err := client.CreateTask(ctx, map[string]any{"title": "Review 42", "brief": "Review it.", "manager": "someone-else"}, created)
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "review-42" || created.Manager != "reviewer-1" || created.Owner != "franco" {
		t.Fatalf("created = %+v, want managed by reviewer-1 and owned by franco", created)
	}

	got := &HomeAgent{}
	err = client.Agent(ctx, got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agent.Name != "reviewer-1" || got.Idle != "20m" || !reflect.DeepEqual(got.Channels, []string{"@franco/reviewer-1", "#review-42"}) {
		t.Fatalf("GET /agent = %+v", got)
	}

	// The home speaks as its agent, in its channels only.
	p, err := client.Join(ctx, "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Agent != "reviewer-1" || p.Task != "" {
		t.Fatalf("participant = %+v", p)
	}

	posted, err := client.Post(ctx, p.ID, "#review-42", "Looking.", nil)
	if err != nil {
		t.Fatal(err)
	}
	if posted.Agent != "reviewer-1" {
		t.Fatalf("message agent = %q", posted.Agent)
	}

	_, err = client.Post(ctx, p.ID, "#general", "hi", nil)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("post outside its channels: %v, want ENOTAUTHORIZED", err)
	}

	_, err = env.tasks.Create(ctx, nil, &tasks.CreateRequest{Title: "Not mine", Brief: "b", Manager: "franco"})
	if err != nil {
		t.Fatal(err)
	}

	err = client.AcceptTask(ctx, "not-mine", &tasks.TaskView{})
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("accept another's task: %v, want ENOTAUTHORIZED", err)
	}

	accepted := &tasks.TaskView{}
	err = client.AcceptTask(ctx, "review-42", accepted)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != "done" || accepted.AcceptedBy != "reviewer-1" {
		t.Fatalf("accepted = %+v", accepted)
	}

	err = client.Deliver(ctx, "done", &tasks.TaskView{})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("deliver from a home: %v, want EINVALID", err)
	}

	err = client.AgentSleep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env.sbx.Wait()

	state := env.get(t, "reviewer-1").State
	if state != StateAsleep {
		t.Fatalf("state after POST /agent/sleep = %s, want asleep", state)
	}
}

func TestMainReadsEveryChannel(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.create(t, "main")
	env.create(t, "reviewer")
	env.wake(t, "main-1")
	env.wake(t, "reviewer-1")

	_, err := env.msg.PostAsSystem(ctx, "#general", "hello")
	if err != nil {
		t.Fatal(err)
	}

	main := homeClient(env, "main-1")
	p, err := main.Join(ctx, "main-1")
	if err != nil {
		t.Fatal(err)
	}

	read, err := main.Read(ctx, "#general", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Messages) != 1 {
		t.Fatalf("main read %+v", read.Messages)
	}

	_, err = main.Post(ctx, p.ID, "#somewhere-new", "routing", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = homeClient(env, "reviewer-1").Read(ctx, "#general", 0, 10)
	if ez.ErrorCode(err) != ez.ENOTAUTHORIZED {
		t.Fatalf("reviewer read #general: %v, want ENOTAUTHORIZED", err)
	}

	if !env.api.isMain("main-1") || env.api.isMain("reviewer-1") || env.api.isMain("nobody") {
		t.Fatal("isMain")
	}
}

func TestShippedTemplatesLoad(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "agents")

	reviewer, err := LoadTemplate(dir, "Reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if reviewer.Harness != "claude_code" || len(reviewer.Reads) != 0 || reviewer.Instructions == "" || reviewer.Sandbox.Disk == "" {
		t.Fatalf("reviewer = %+v", reviewer)
	}

	main, err := LoadTemplate(dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(main.Reads, []string{"*"}) || main.Instructions == "" {
		t.Fatalf("main = %+v", main)
	}
}
