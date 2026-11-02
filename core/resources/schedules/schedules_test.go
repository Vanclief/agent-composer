package schedules

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/vanclief/ez"
	_ "modernc.org/sqlite"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

var testNow = time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)

type testEnv struct {
	api   *API
	tasks *tasks.API
	db    *bun.DB
	msg   *messaging.Service
	dir   string
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
		(*schedulemodels.Schedule)(nil),
		(*schedulemodels.Item)(nil),
	}
	for _, table := range tables {
		_, err = db.NewCreateTable().Model(table).Exec(context.Background())
		if err != nil {
			t.Fatal(err)
		}
	}

	dir := t.TempDir()
	secretsPath := filepath.Join(dir, "secrets.env")
	err = os.WriteFile(secretsPath, []byte("GITHUB_TOKEN=ghp_value\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	socketDir, err := os.MkdirTemp("", "agc")
	if err != nil {
		t.Fatal(err)
	}

	msg := messaging.NewService(db)
	sandboxesAPI := sandboxes.New(context.Background(), db, msg, sandboxes.SandboxRuntime{}, secretsPath, socketDir)
	specs := task.Store{Dir: filepath.Join(dir, "specs")}
	defaults := controller.DefaultsSettings{Harness: "claude_code", Model: "claude-opus-5-5", Owner: "franco"}
	tasksAPI := tasks.New(db, sandboxesAPI, msg, defaults, secretsPath, specs)

	api := New(db, tasksAPI, secretsPath)
	api.now = func() time.Time {
		return testNow
	}

	t.Cleanup(func() {
		api.Wait()
		sandboxesAPI.Wait()
		_ = os.RemoveAll(socketDir)
	})

	return &testEnv{api: api, tasks: tasksAPI, db: db, msg: msg, dir: dir}
}

// script writes an executable shell script and returns its path.
func (env *testEnv) script(t *testing.T, name string, body string) string {
	t.Helper()

	path := filepath.Join(env.dir, name)
	err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func reviewTemplate() schedulemodels.Template {
	return schedulemodels.Template{
		Title:   "Review {{.title}}",
		Brief:   "Review the pull request {{.url}} and post the review.",
		Accept:  []string{"the review of {{.url}} is posted"},
		Manager: "orchestrator",
	}
}

func (env *testEnv) create(t *testing.T, request *CreateRequest) *ScheduleView {
	t.Helper()

	view, err := env.api.Create(context.Background(), nil, request)
	if err != nil {
		t.Fatal(err)
	}

	return view
}

func (env *testEnv) run(t *testing.T, name string) *RunResponse {
	t.Helper()

	response, err := env.api.Run(context.Background(), nil, &RunRequest{Name: name})
	if err != nil {
		t.Fatal(err)
	}

	return response
}

func TestTickWithoutPollCreatesATask(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	created := env.create(t, &CreateRequest{
		Name:  "digest",
		Every: "24h",
		Task: schedulemodels.Template{
			Title:   `Digest for {{.Now.Format "2006-01-02"}}`,
			Brief:   "Summarize yesterday.",
			Manager: "orchestrator",
		},
	})
	if !created.Enabled || created.LastRunAt != nil || created.NextRunAt == nil || !created.NextRunAt.Equal(created.CreatedAt) {
		t.Fatalf("created = %+v", created)
	}

	bus := events.New()
	env.api.SetEvents(bus)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream := bus.Subscribe(subCtx)

	response := env.run(t, "digest")
	if response.Error != "" || len(response.Created) != 1 {
		t.Fatalf("response = %+v", response)
	}

	// schedule.ran carries the schedule and what it created.
	ran := false
	for !ran {
		select {
		case event := <-stream:
			if event.Type != events.ScheduleRan {
				continue
			}
			data := event.Data.(RanEvent)
			if data.Schedule.Name != "digest" || data.Schedule.Running || len(data.Created) != 1 || data.Created[0].Name != "digest-20261102-0930" {
				t.Fatalf("schedule.ran = %+v", data)
			}
			ran = true
		case <-time.After(5 * time.Second):
			t.Fatal("no schedule.ran event")
		}
	}

	made := response.Created[0]
	if made.Name != "digest-20261102-0930" || made.Title != "Digest for 2026-11-02" || made.Assignee != "claude_code" || made.Owner != "franco" {
		t.Fatalf("task = %+v", made)
	}

	view := response.Schedule
	if view.Running || view.LastRunAt == nil || !view.LastRunAt.Equal(testNow) || !view.NextRunAt.Equal(testNow.Add(24*time.Hour)) {
		t.Fatalf("schedule = %+v", view)
	}

	// Created like POST /api/tasks: agc opened its channel.
	read, err := env.msg.Read(ctx, messaging.Caller{Human: true}, "#digest-20261102-0930", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read) == 0 {
		t.Fatal("no opening message in #digest-20261102-0930")
	}

	items, err := env.api.ListItems(ctx, nil, &ListItemsRequest{Name: "digest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 1 || items.Items[0].Key != made.Name || items.Items[0].TaskID != made.ID {
		t.Fatalf("items = %+v", items.Items)
	}

	// The same minute gives the same name, which the tasks API refuses.
	again := env.run(t, "digest")
	if len(again.Created) != 0 || !strings.Contains(again.Error, "already exists") || again.Schedule.LastError != again.Error {
		t.Fatalf("again = %+v", again)
	}
}

func TestTickWithPollCreatesOneTaskPerNewKey(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	t.Setenv("AGC_SCHEDULE_TEST_LEAK", "leaked")

	listFile := filepath.Join(env.dir, "prs.jsonl")
	err := os.WriteFile(listFile, []byte(`{"key":"pr-1@aaa","url":"https://github.com/o/r/pull/1","title":"Fix the icon"}

{"key":"pr-2@bbb","url":"https://github.com/o/r/pull/2","title":"Add schedules"}
{"key":"pr-1@aaa","url":"https://github.com/o/r/pull/1","title":"Fix the icon"}
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// The command sees PATH, HOME and the secret it names, nothing else.
	poll := env.script(t, "poll.sh", `[ "$GITHUB_TOKEN" = ghp_value ] || { echo "no token" >&2; exit 1; }
[ -z "$AGC_SCHEDULE_TEST_LEAK" ] || { echo "env leaked" >&2; exit 1; }
[ -n "$HOME" ] || { echo "no HOME" >&2; exit 1; }
cat `+listFile)

	env.create(t, &CreateRequest{
		Name:  "pr-reviews",
		Every: "2m",
		Task:  reviewTemplate(),
		Poll:  &schedulemodels.Poll{Command: poll, Secrets: []string{"GITHUB_TOKEN"}},
	})

	first := env.run(t, "pr-reviews")
	if first.Error != "" || len(first.Created) != 2 {
		t.Fatalf("first = %+v", first)
	}

	made := first.Created[0]
	if made.Name != "pr-reviews-"+keySuffix("pr-1@aaa") || made.Title != "Review Fix the icon" ||
		made.Brief != "Review the pull request https://github.com/o/r/pull/1 and post the review." ||
		len(made.Accept) != 1 || made.Accept[0] != "the review of https://github.com/o/r/pull/1 is posted" {
		t.Fatalf("task = %+v", made)
	}

	second := env.run(t, "pr-reviews")
	if second.Error != "" || len(second.Created) != 0 {
		t.Fatalf("second = %+v", second)
	}

	// A new push is a new key.
	err = os.WriteFile(listFile, []byte(`{"key":"pr-1@ccc","url":"https://github.com/o/r/pull/1","title":"Fix the icon"}`+"\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	third := env.run(t, "pr-reviews")
	if third.Error != "" || len(third.Created) != 1 || third.Created[0].Name != "pr-reviews-"+keySuffix("pr-1@ccc") {
		t.Fatalf("third = %+v", third)
	}

	items, err := env.api.ListItems(ctx, nil, &ListItemsRequest{Name: "pr-reviews"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items.Items) != 3 {
		t.Fatalf("items = %+v", items.Items)
	}

	// Deleting the schedule keeps its tasks.
	_, err = env.api.Delete(ctx, nil, &DeleteRequest{Name: "pr-reviews"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := env.tasks.List(ctx, nil, &tasks.ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tasks) != 3 {
		t.Fatalf("tasks after delete = %d", len(list.Tasks))
	}
}

func TestFailingPollSetsLastErrorAndCreatesNothing(t *testing.T) {
	cases := []struct {
		name    string
		command string
		want    string
	}{
		{"exits", `echo "boom ghp_value" >&2; exit 3`, "poll command failed: exit status 3: boom [GITHUB_TOKEN]"},
		{"bad-json", `echo '{"key":"a","title":"ok"}'; echo 'not json'`, "poll output line 2 is not a JSON object"},
		{"no-key", `echo '{"title":"ok"}'`, `poll output line 1 has no key`},
		{"not-a-string", `echo '{"key":"a","number":7}'`, "field number is not a string"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)

			env.create(t, &CreateRequest{
				Name:  "pr-reviews",
				Every: "2m",
				Task:  reviewTemplate(),
				Poll:  &schedulemodels.Poll{Command: env.script(t, "poll.sh", tc.command), Secrets: []string{"GITHUB_TOKEN"}},
			})

			response := env.run(t, "pr-reviews")
			if len(response.Created) != 0 || !strings.Contains(response.Error, tc.want) || response.Schedule.LastError != response.Error {
				t.Fatalf("response = %+v, want error %q", response, tc.want)
			}
			if strings.Contains(response.Error, "ghp_value") {
				t.Fatalf("the secret value is in last_error: %s", response.Error)
			}

			list, err := env.tasks.List(context.Background(), nil, &tasks.ListRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Tasks) != 0 {
				t.Fatalf("tasks = %+v", list.Tasks)
			}
		})
	}
}

func TestPollTimesOutAndNeedsItsSecrets(t *testing.T) {
	env := newTestEnv(t)
	env.api.pollTimeout = 200 * time.Millisecond

	env.create(t, &CreateRequest{
		Name:  "slow",
		Every: "1m",
		Task:  reviewTemplate(),
		Poll:  &schedulemodels.Poll{Command: "sleep 5"},
	})
	response := env.run(t, "slow")
	if !strings.Contains(response.Error, "timed out after 200ms") {
		t.Fatalf("response = %+v", response)
	}

	env.create(t, &CreateRequest{
		Name:  "unset",
		Every: "1m",
		Task:  reviewTemplate(),
		Poll:  &schedulemodels.Poll{Command: "true", Secrets: []string{"OPENAI_API_KEY"}},
	})
	response = env.run(t, "unset")
	if !strings.Contains(response.Error, "secret OPENAI_API_KEY is not set") {
		t.Fatalf("response = %+v", response)
	}
}

func TestTemplateRendering(t *testing.T) {
	data := itemData(map[string]string{"key": "k", "url": "https://x/1", "title": "One"}, testNow)

	rendered, err := renderTemplate(schedulemodels.Template{
		Title:    "Review {{.title}}",
		Name:     "review-{{.key}}",
		Brief:    "{{if .url}}Review {{.url}}{{else}}none{{end}} on {{.Now.Format \"Jan 2\"}}",
		Accept:   []string{"{{$.url}} reviewed"},
		Assignee: "{{.title}}",
	}, data)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Title != "Review One" || rendered.Name != "review-k" || rendered.Brief != "Review https://x/1 on Nov 2" ||
		rendered.Accept[0] != "https://x/1 reviewed" || rendered.Assignee != "{{.title}}" {
		t.Fatalf("rendered = %+v", rendered)
	}

	_, err = renderTemplate(schedulemodels.Template{Brief: "Review {{.uri}}"}, data)
	if err == nil || !strings.Contains(err.Error(), `map has no entry for key "uri"`) {
		t.Fatalf("missing field: err = %v", err)
	}

	fields, err := templateFields(schedulemodels.Template{
		Title:  "{{.title}} {{.Now}}",
		Brief:  "{{range .items}}{{.}}{{end}}{{with .owner}}{{.}}{{end}}",
		Accept: []string{"{{if eq .state \"open\"}}open{{end}}", "{{$.repo | printf \"%s\"}}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, field := range fields {
		got[field] = true
	}
	for _, want := range []string{"title", "Now", "items", "owner", "state", "repo"} {
		if !got[want] {
			t.Fatalf("fields = %v, missing %s", fields, want)
		}
	}

	long := strings.Repeat("a", 60)
	name := defaultName(long, "20261102-0930")
	if len(name) != 63 || !strings.HasSuffix(name, "-20261102-0930") {
		t.Fatalf("default name %q", name)
	}
	if defaultName("pr-reviews", keySuffix("x")) != "pr-reviews-"+keySuffix("x") || len(keySuffix("x")) != 8 {
		t.Fatalf("key suffix %q", keySuffix("x"))
	}
}

func TestCreateRejects(t *testing.T) {
	env := newTestEnv(t)

	good := func() *CreateRequest {
		return &CreateRequest{Name: "pr-reviews", Every: "2m", Task: reviewTemplate(), Poll: &schedulemodels.Poll{Command: "true"}}
	}

	cases := []struct {
		name   string
		change func(r *CreateRequest)
		want   string
	}{
		{"name", func(r *CreateRequest) { r.Name = "PR Reviews" }, "schedule name"},
		{"every", func(r *CreateRequest) { r.Every = "30s" }, "shorter than the minimum"},
		{"every-missing", func(r *CreateRequest) { r.Every = "" }, "every is required"},
		{"every-bad", func(r *CreateRequest) { r.Every = "often" }, "not a duration"},
		{"command", func(r *CreateRequest) { r.Poll.Command = " " }, "poll.command is required"},
		{"secret", func(r *CreateRequest) { r.Poll.Secrets = []string{"github_token"} }, "secret name"},
		{"secret-hosts", func(r *CreateRequest) { r.Poll.Secrets = []string{"GITHUB_TOKEN@api.github.com"} }, "runs on the host"},
		{"syntax", func(r *CreateRequest) { r.Task.Brief = "Review {{.url" }, "task.brief is not a valid template"},
		{"manager", func(r *CreateRequest) { r.Task.Manager = "" }, "manager is required"},
		{"brief", func(r *CreateRequest) { r.Task.Brief = "" }, "brief is required"},
		{"person-model", func(r *CreateRequest) {
			r.Task.Assignee = "franco"
			r.Task.Model = "gpt"
		}, "is a person"},
		{"task-name", func(r *CreateRequest) { r.Task.Name = "Review {{.url}}" }, "task name"},
		{"no-poll-title", func(r *CreateRequest) {
			r.Poll = nil
			r.Task.Title = "Review {{.url}}"
		}, `no entry for key "url"`},
	}

	for _, tc := range cases {
		request := good()
		tc.change(request)

		_, err := env.api.Create(context.Background(), nil, request)
		if ez.ErrorCode(err) != ez.EINVALID || !strings.Contains(ez.ErrorMessage(err), tc.want) {
			t.Fatalf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}

	env.create(t, good())
	_, err := env.api.Create(context.Background(), nil, good())
	if ez.ErrorCode(err) != ez.ECONFLICT {
		t.Fatalf("duplicate: err = %v", err)
	}
}

// blockingRunner holds every poll until release is closed.
type blockingRunner struct {
	started chan struct{}
	release chan struct{}

	mu    sync.Mutex
	calls int
}

func (b *blockingRunner) run(ctx context.Context, command string, env []string) ([]byte, []byte, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()

	b.started <- struct{}{}
	<-b.release

	return []byte(`{"key":"pr-1","url":"https://x/1","title":"One"}`), nil, nil
}

func (b *blockingRunner) Calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func TestTicksNeverOverlap(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	runner := &blockingRunner{started: make(chan struct{}, 4), release: make(chan struct{})}
	env.api.run = runner.run

	env.create(t, &CreateRequest{Name: "pr-reviews", Every: "2m", Task: reviewTemplate(), Poll: &schedulemodels.Poll{Command: "gh"}})

	// The scheduler starts a tick: the schedule never ran, so it is due.
	env.api.RunDue(ctx)
	<-runner.started

	view, err := env.api.Get(ctx, nil, &GetRequest{Name: "pr-reviews"})
	if err != nil {
		t.Fatal(err)
	}
	if !view.Running {
		t.Fatalf("view = %+v, want running", view)
	}

	// Due again while it runs: skipped by the scheduler, a conflict for run.
	env.api.RunDue(ctx)
	_, err = env.api.Run(ctx, nil, &RunRequest{Name: "pr-reviews"})
	if ez.ErrorCode(err) != ez.ECONFLICT || !strings.Contains(ez.ErrorMessage(err), "already running") {
		t.Fatalf("run while running: err = %v", err)
	}

	close(runner.release)
	env.api.Wait()

	if runner.Calls() != 1 {
		t.Fatalf("poll ran %d times, want 1", runner.Calls())
	}

	// Ran at testNow, so not due again until testNow + 2m.
	env.api.RunDue(ctx)
	env.api.Wait()
	if runner.Calls() != 1 {
		t.Fatalf("a schedule that is not due ran: %d calls", runner.Calls())
	}

	env.api.now = func() time.Time {
		return testNow.Add(2 * time.Minute)
	}
	env.api.RunDue(ctx)
	env.api.Wait()
	if runner.Calls() != 2 {
		t.Fatalf("a due schedule did not run: %d calls", runner.Calls())
	}

	// Disabled schedules do not run, however due.
	disabled := false
	_, err = env.api.Update(ctx, nil, &UpdateRequest{Name: "pr-reviews", Every: "2m", Enabled: &disabled, Task: reviewTemplate(), Poll: &schedulemodels.Poll{Command: "gh"}})
	if err != nil {
		t.Fatal(err)
	}
	env.api.now = func() time.Time {
		return testNow.Add(time.Hour)
	}
	env.api.RunDue(ctx)
	env.api.Wait()
	if runner.Calls() != 2 {
		t.Fatalf("a disabled schedule ran: %d calls", runner.Calls())
	}
}

func TestParseFile(t *testing.T) {
	parsed, err := ParseFile([]byte(`name: pr-reviews
every: 2m
enabled: false
poll:
  command: gh search prs
  secrets: [GITHUB_TOKEN]
task:
  title: Review {{.title}}
  brief: Review the pull request {{.url}}.
  manager: orchestrator
  sandbox:
    repo: github.com/Vanclief/compose
`))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Name != "pr-reviews" || parsed.Enabled == nil || *parsed.Enabled || parsed.Poll.Secrets[0] != "GITHUB_TOKEN" ||
		parsed.Task.Sandbox == nil || parsed.Task.Sandbox.Repo != "github.com/Vanclief/compose" {
		t.Fatalf("parsed = %+v", parsed)
	}

	_, err = ParseFile([]byte("name: x\nevry: 2m\n"))
	if err == nil {
		t.Fatal("an unknown key was accepted")
	}
}

// The example in docs is a valid schedule.
func TestDocsExample(t *testing.T) {
	env := newTestEnv(t)

	request, err := LoadFile("../../../docs/schedules/pr-reviews.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(request.Poll.Command, "--jq '.[] | {key: (.url + \"@\" + .headRefOid), url, title}'") {
		t.Fatalf("command = %q", request.Poll.Command)
	}

	request.Task.Manager = "orchestrator"
	view := env.create(t, request)
	if view.Every != "2m" || view.Poll.Secrets[0] != "GITHUB_TOKEN" || view.Task.Title != "Review {{.title}}" {
		t.Fatalf("view = %+v", view)
	}
}
