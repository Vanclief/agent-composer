package tasks

import (
	"context"
	"database/sql"
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
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// fakeRuntime boots every sandbox as running and records removals.
type fakeRuntime struct {
	mu       sync.Mutex
	statuses map[string]string
	calls    []string
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

func (f *fakeRuntime) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	f.record("create " + spec.Name)
	f.mu.Lock()
	f.statuses[spec.Name] = "running"
	f.mu.Unlock()
	return &fakeSandbox{}, nil
}

func (f *fakeRuntime) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	return &fakeSandbox{}, nil
}

func (f *fakeRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
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
	status, ok := f.statuses[name]
	if !ok {
		return "", ez.New(ez.ENOTFOUND, "cannot find sandbox "+name, nil)
	}
	return status, nil
}

// fakeSandbox succeeds at every boot step. Other methods panic on the nil
// embedded interface, which no test reaches.
type fakeSandbox struct {
	executor.TerminalExecutor
}

func (s *fakeSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	return &executor.Result{}, nil
}

func (s *fakeSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	return nil
}

func (s *fakeSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	return &executor.Terminal{ID: "job_1", Running: true}, nil
}

func (s *fakeSandbox) Detach(ctx context.Context) error {
	return nil
}

type testEnv struct {
	api *API
	rt  *fakeRuntime
	db  *bun.DB
	msg *messaging.Service
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
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	secretsPath := filepath.Join(t.TempDir(), "secrets.env")
	err = os.WriteFile(secretsPath, []byte("GITHUB_TOKEN=ghp_value\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Unix socket paths are capped near 104 bytes, too short for
	// t.TempDir() on macOS.
	socketDir, err := os.MkdirTemp("", "agc")
	if err != nil {
		t.Fatal(err)
	}

	rt := &fakeRuntime{statuses: map[string]string{}}
	msg := messaging.NewService(db)
	sandboxesAPI := sandboxes.New(context.Background(), db, msg, rt, secretsPath, socketDir)
	specs := task.Store{Dir: filepath.Join(t.TempDir(), "specs")}
	api := New(db, sandboxesAPI, msg, controller.DefaultsSettings{Harness: "claude_code", Model: "claude-opus-5-5", Owner: "franco"}, secretsPath, specs)

	t.Cleanup(func() {
		sandboxesAPI.Wait()
		_ = os.RemoveAll(socketDir)
	})

	return &testEnv{api: api, rt: rt, db: db, msg: msg}
}

func TestCreateWithoutSandbox(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	view, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Title: "Call the bank", Brief: "Ask about the fee.", Assignee: " Franco ", For: "fees"})
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "call-the-bank" || view.Status != "open" || view.Assignee != "Franco" || view.For != "fees" || view.DoneAt != nil {
		t.Fatalf("view = %+v", view)
	}
	if view.Sandboxes == nil || len(view.Sandboxes) != 0 {
		t.Fatalf("sandboxes = %#v, want empty and not nil", view.Sandboxes)
	}
	if len(env.rt.Calls()) != 0 {
		t.Fatalf("runtime was called: %q", env.rt.Calls())
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Title: "Call the bank", Brief: "again"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("expected ECONFLICT for a duplicate name, got %v", err)
	}
}

func TestCreateWithSandboxReturnsAtOnce(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	request := &CreateRequest{Manager: "orchestrator", Name: "review-42", Sandbox: &task.Sandbox{Spec: task.Spec{Repo: "github.com/o/n"}, Command: "claude"}}
	view, err := env.api.Create(ctx, nil, request)
	if err != nil {
		t.Fatal(err)
	}

	if view.Status != "open" || len(view.Sandboxes) != 1 {
		t.Fatalf("view = %+v", view)
	}
	sb := view.Sandboxes[0]
	if sb.Name != "review-42" || sb.Task != "review-42" || sb.Status != "starting" || sb.Command != "claude" || sb.Spec.Repo != "github.com/o/n" {
		t.Fatalf("sandbox = %+v", sb)
	}

	env.api.Sandboxes.Wait()

	view, err = env.api.Get(ctx, nil, &GetRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" || view.Sandboxes[0].Status != "running" {
		t.Fatalf("after boot: %+v", view)
	}
}

func TestCreateTitleAndName(t *testing.T) {
	cases := []struct {
		title     string
		name      string
		wantTitle string
		wantName  string
	}{
		{"Review facebook ads", "", "Review facebook ads", "review-facebook-ads"},
		{"", "review-42", "review-42", "review-42"},
		{" Fix login ", " login-fix ", "Fix login", "login-fix"},
	}

	for _, tc := range cases {
		env := newTestEnv(t)

		view, err := env.api.Create(context.Background(), nil, &CreateRequest{Manager: "orchestrator", Title: tc.title, Name: tc.name, Brief: "b"})
		if err != nil {
			t.Fatal(err)
		}

		if view.Title != tc.wantTitle || view.Name != tc.wantName {
			t.Fatalf("title %q name %q: got title %q name %q", tc.title, tc.name, view.Title, view.Name)
		}
	}
}

func TestCreateRejects(t *testing.T) {
	env := newTestEnv(t)

	cases := []struct {
		request CreateRequest
		want    string
	}{
		{CreateRequest{Manager: "orchestrator", Title: "  ", Name: "", Brief: "b"}, "title or name is required"},
		{CreateRequest{Manager: "orchestrator", Title: "???", Brief: "b"}, "has no letters or digits"},
		{CreateRequest{Manager: "orchestrator", Title: strings.Repeat("x", 121), Name: "long", Brief: "b"}, "longer than 120"},
		{CreateRequest{Manager: "orchestrator", Title: "Fine", Name: "Not A Slug", Brief: "b"}, "is invalid"},
		{CreateRequest{Manager: "orchestrator", Title: "No brief"}, "brief is required"},
		{CreateRequest{Manager: "orchestrator", Title: "Bad ref", Sandbox: &task.Sandbox{Spec: task.Spec{Ref: "master"}}}, "ref is set but repo is not"},
		{CreateRequest{Title: "No manager", Brief: "b"}, "manager is required"},
		{CreateRequest{Manager: "Not A Name", Title: "Bad manager", Brief: "b"}, "manager name"},
		{CreateRequest{Manager: "orchestrator", Owner: "#franco", Title: "Bad owner", Brief: "b"}, "owner name"},
		{CreateRequest{Manager: "orchestrator", Title: "Person model", Brief: "b", Assignee: "ana", Model: "claude-opus-5-5"}, "assignee ana is a person"},
	}

	for _, tc := range cases {
		_, err := env.api.Create(context.Background(), nil, &tc.request)
		if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), tc.want) {
			t.Fatalf("%+v: expected EINVALID containing %q, got %v", tc.request, tc.want, err)
		}
	}
}

func TestCreateMissingSecretLeavesNoTask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	request := &CreateRequest{Manager: "orchestrator", Name: "review-42", Sandbox: &task.Sandbox{Spec: task.Spec{Secrets: []string{"OPENAI_API_KEY"}}}}
	_, err := env.api.Create(ctx, nil, request)
	if ez.ErrorCode(err) != ez.ENOTFOUND || !strings.Contains(ez.ErrorMessage(err), "OPENAI_API_KEY") {
		t.Fatalf("expected ENOTFOUND naming the secret, got %v", err)
	}

	_, err = taskmodels.GetTaskByName(ctx, env.db, "review-42")
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected no task row, got %v", err)
	}
}

func TestDeliverAcceptAndReject(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "review-42", Accept: []string{" findings posted "}, Sandbox: &task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Sandboxes.Wait()

	if len(created.Accept) != 1 || created.Accept[0] != "findings posted" {
		t.Fatalf("accept = %q", created.Accept)
	}

	view, err := env.api.Deliver(ctx, nil, &DeliverRequest{Name: "review-42", Summary: "first"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = env.api.Deliver(ctx, nil, &DeliverRequest{ID: view.ID, Summary: " Posted 3 findings. "})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != StatusReview || view.DeliveredAt == nil || view.Summary != "Posted 3 findings." || view.Sandboxes[0].Status != "running" {
		t.Fatalf("delivered: %+v", view)
	}

	view, err = env.api.Reject(ctx, nil, &RejectRequest{Name: "review-42", By: "orchestrator", Why: "finding 2 is wrong"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != StatusRunning || view.DeliveredAt != nil || view.Summary != "Posted 3 findings." {
		t.Fatalf("rejected: %+v", view)
	}

	// By defaults to the owner.
	view, err = env.api.Accept(ctx, nil, &AcceptRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != StatusDone || view.DoneAt == nil || view.AcceptedBy != "franco" || view.DeliveredAt != nil {
		t.Fatalf("accepted: %+v", view)
	}
	if view.Sandboxes[0].Status != "stopped" {
		t.Fatalf("sandbox not stopped: %+v", view.Sandboxes[0])
	}

	first := *view.DoneAt
	view, err = env.api.Accept(ctx, nil, &AcceptRequest{ID: view.ID, By: "orchestrator"})
	if err != nil || !view.DoneAt.Equal(first) || view.AcceptedBy != "franco" {
		t.Fatalf("second accept: %+v, err %v", view, err)
	}

	_, err = env.api.Reject(ctx, nil, &RejectRequest{Name: "review-42", Why: "late"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("reject after accept: %v", err)
	}
	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "review-42", Summary: "again"})
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("deliver after accept: %v", err)
	}

	messages, err := env.msg.Read(ctx, messaging.Caller{Human: true}, "#review-42", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, m := range messages[2:] {
		got = append(got, m.From+": "+m.Body)
	}
	want := []string{"agc: Delivered: first", "agc: Delivered: Posted 3 findings.", "agc: Rejected by orchestrator: finding 2 is wrong", "agc: Accepted by franco."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("messages = %q", got)
	}

	calls := env.rt.Calls()
	if len(calls) != 2 || calls[1] != "stop review-42" {
		t.Fatalf("calls = %q", calls)
	}
}

func TestAcceptRejectAndDeliverValidate(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"deliver without summary", DeliverRequest{Name: "x", Summary: " "}.Validate()},
		{"reject without why", RejectRequest{Name: "x", Why: "\n"}.Validate()},
		{"accept by a bad name", AcceptRequest{Name: "x", By: "Not A Name"}.Validate()},
		{"create with an empty criterion", CreateRequest{Name: "x", Brief: "b", Manager: "m", Accept: []string{"ok", "  "}}.Validate()},
	}

	for _, tc := range cases {
		if ez.ErrorCode(tc.err) != ez.EINVALID {
			t.Fatalf("%s: %v", tc.name, tc.err)
		}
	}

	// Without an owner, by has nothing to default to.
	env := newTestEnv(t)
	ctx := context.Background()
	env.api.defaults.Owner = ""

	_, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "ownerless", Brief: "b"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Accept(ctx, nil, &AcceptRequest{Name: "ownerless"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("accept without by or owner: %v", err)
	}
}

func TestListAndStatus(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	_, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "human", Brief: "b"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "stopped", Sandbox: &task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "busy", Sandbox: &task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "delivered", Sandbox: &task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Sandboxes.Wait()

	_, err = env.api.Sandboxes.Stop(ctx, nil, &sandboxes.StopRequest{Name: "stopped"})
	if err != nil {
		t.Fatal(err)
	}

	// A delivered task is in review even with its sandbox running.
	_, err = env.api.Deliver(ctx, nil, &DeliverRequest{Name: "delivered", Summary: "done it"})
	if err != nil {
		t.Fatal(err)
	}

	list, err := env.api.List(ctx, nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, view := range list.Tasks {
		got = append(got, view.Name+"="+view.Status)
	}

	want := []string{"delivered=review", "busy=running", "stopped=open", "human=open"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestDeleteRemovesSandboxesThenTask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created, err := env.api.Create(ctx, nil, &CreateRequest{Manager: "orchestrator", Name: "review-42", Sandbox: &task.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.api.Sandboxes.Allocate(ctx, nil, &sandboxes.AllocateRequest{TaskID: created.ID})
	if err != nil {
		t.Fatal(err)
	}
	env.api.Sandboxes.Wait()

	deleted, err := env.api.Delete(ctx, nil, &DeleteRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Deleted || deleted.Name != "review-42" {
		t.Fatalf("delete response %+v", deleted)
	}

	// The two boots run concurrently, so their creates come in either
	// order; the delete then goes newest first.
	calls := env.rt.Calls()
	creates := map[string]bool{}
	if len(calls) > 2 {
		creates[calls[0]] = true
		creates[calls[1]] = true
	}
	wantCreates := map[string]bool{"create review-42": true, "create review-42-2": true}
	wantDeletes := []string{"stop review-42-2", "remove review-42-2", "stop review-42", "remove review-42"}
	if len(calls) != 6 || !reflect.DeepEqual(creates, wantCreates) || !reflect.DeepEqual(calls[2:], wantDeletes) {
		t.Fatalf("calls = %q", calls)
	}

	list, err := env.api.Sandboxes.List(ctx, nil, &sandboxes.ListRequest{})
	if err != nil || len(list.Sandboxes) != 0 {
		t.Fatalf("sandboxes left: %+v, err %v", list, err)
	}

	_, err = env.api.Get(ctx, nil, &GetRequest{Name: "review-42"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected ENOTFOUND after delete, got %v", err)
	}
}

func TestSpecs(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	saved, err := env.api.SaveSpec(ctx, nil, &SaveSpecRequest{Name: "Backend review", YAML: "# mine\nrepo: github.com/o/n\n"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "backend-review" || saved.YAML != "# mine\nrepo: github.com/o/n\n" {
		t.Fatalf("saved = %+v", saved)
	}

	_, err = env.api.SaveSpec(ctx, nil, &SaveSpecRequest{Name: "broken", YAML: "ref: master\n"})
	if ez.ErrorCode(err) != ez.EINVALID {
		t.Fatalf("expected EINVALID for an invalid spec, got %v", err)
	}

	list, err := env.api.ListSpecs(ctx, nil, &ListSpecsRequest{})
	if err != nil || len(list.Specs) != 1 || list.Specs[0].Name != "backend-review" {
		t.Fatalf("list = %+v, err %v", list, err)
	}

	got, err := env.api.GetSpec(ctx, nil, &GetSpecRequest{Name: "backend-review"})
	if err != nil || got.YAML != saved.YAML {
		t.Fatalf("get = %+v, err %v", got, err)
	}

	deleted, err := env.api.DeleteSpec(ctx, nil, &DeleteSpecRequest{Name: "backend-review"})
	if err != nil || deleted.Name != "backend-review" || deleted.YAML != saved.YAML {
		t.Fatalf("delete = %+v, err %v", deleted, err)
	}

	_, err = env.api.GetSpec(ctx, nil, &GetSpecRequest{Name: "backend-review"})
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		t.Fatalf("expected ENOTFOUND after delete, got %v", err)
	}
}
