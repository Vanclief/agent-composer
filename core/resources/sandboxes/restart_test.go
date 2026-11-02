package sandboxes

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/executor"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// systemPosts lists agc's messages in channel, oldest first.
func systemPosts(t *testing.T, e *testEnv, channel string) []string {
	t.Helper()

	messages := []msgmodels.Message{}
	err := e.db.NewSelect().Model(&messages).Where("channel = ?", channel).Where(`"from" = ?`, msgmodels.SystemName).Order("id ASC").Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	bodies := []string{}
	for _, message := range messages {
		bodies = append(bodies, message.Body)
	}

	return bodies
}

func TestResumeRestartsCrashedSandboxesOfOpenTasks(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	now := time.Now()

	open := &taskmodels.Task{Title: "Recovery", Name: "recovery", Owner: "franco"}
	err := open.Insert(ctx, env.db)
	if err != nil {
		t.Fatal(err)
	}
	doneAt := now
	done := &taskmodels.Task{Title: "Shipped", Name: "shipped", DoneAt: &doneAt}
	err = done.Insert(ctx, env.db)
	if err != nil {
		t.Fatal(err)
	}

	crashed := insertBooted(t, env.db, open, "recovery", now)
	crashed.Command = "agc agent run --task recovery"
	err = crashed.Update(ctx, env.db, "command")
	if err != nil {
		t.Fatal(err)
	}
	stopped := insertBooted(t, env.db, open, "recovery-2", now)
	stopped.StoppedAt = &now
	err = stopped.Update(ctx, env.db, "stopped_at")
	if err != nil {
		t.Fatal(err)
	}
	insertBooted(t, env.db, done, "shipped", now)

	env.rt.statuses["recovery"] = StatusCrashed
	env.rt.statuses["recovery-2"] = StatusStopped
	env.rt.statuses["shipped"] = StatusCrashed

	// Without restart, as every stack but the server's: nothing starts.
	err = env.api.Resume(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls without restart = %v", env.rt.Calls())
	}

	err = env.api.Resume(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"start recovery", "terminal agc agent run --task recovery in /home/agent", "detach"}
	if !reflect.DeepEqual(env.rt.Calls(), want) {
		t.Fatalf("calls = %v, want %v", env.rt.Calls(), want)
	}
	if !reflect.DeepEqual(env.api.listenerNames(), []string{"recovery"}) {
		t.Fatalf("listeners = %v", env.api.listenerNames())
	}

	line := []string{"recovery came back after a restart; its agent continues."}
	if !reflect.DeepEqual(systemPosts(t, env, "#recovery"), line) {
		t.Fatalf("#recovery = %q", systemPosts(t, env, "#recovery"))
	}
	if !reflect.DeepEqual(systemPosts(t, env, "#franco"), line) {
		t.Fatalf("#franco = %q", systemPosts(t, env, "#franco"))
	}
	if len(systemPosts(t, env, "#shipped")) != 0 {
		t.Fatal("posted about a done task")
	}
}

func TestResumePostsAFailedRestartOnce(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	owner := insertTask(t, env.db, "broken")

	record := insertBooted(t, env.db, owner, "broken", time.Now())
	// A spec naming a secret the store does not have cannot start.
	record.Spec = []byte(`{"secrets":[{"name":"MISSING_TOKEN","hosts":["api.example.com"]}]}`)
	err := record.Update(ctx, env.db, "spec")
	if err != nil {
		t.Fatal(err)
	}
	env.rt.statuses["broken"] = StatusCrashed

	err = env.api.Resume(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	got := systemPosts(t, env, "#broken")
	if len(got) != 1 || !strings.HasPrefix(got[0], "broken did not come back after a restart: sandbox broken is crashed and could not start: ") {
		t.Fatalf("#broken = %q", got)
	}
	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls = %v", env.rt.Calls())
	}
}

func TestRestartKeepsTheAgentJobItFinds(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	owner := insertTask(t, e.db, "x")
	record := insertBooted(t, e.db, owner, "x", time.Now())
	record.Command = "agc agent run --task x"
	e.rt.statuses["x"] = StatusCrashed

	command := []string{"agc", "agent", "run", "--task", "x"}
	rt := &jobRuntime{
		fakeRuntime: e.rt,
		jobs:        []executor.Job{{ID: "agent", Command: command, Running: true, CreatedAt: time.Now()}},
	}
	e.api.rt = rt

	err := e.api.Restart(ctx, record, owner)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"start x", "detach"}
	if !reflect.DeepEqual(e.rt.Calls(), want) {
		t.Fatalf("calls = %v, want %v", e.rt.Calls(), want)
	}

	// A job msb still lists as running from before the crash is not the
	// agent: it starts again.
	rt.jobs = []executor.Job{{ID: "stale", Command: command, Running: true, CreatedAt: time.Now().Add(-3 * time.Hour)}}
	e.rt.statuses["x"] = StatusCrashed

	err = e.api.Restart(ctx, record, owner)
	if err != nil {
		t.Fatal(err)
	}

	want = append(want, "start x", "terminal agc agent run --task x in /home/agent", "detach")
	if !reflect.DeepEqual(e.rt.Calls(), want) {
		t.Fatalf("calls = %v, want %v", e.rt.Calls(), want)
	}
}

func TestRecoverable(t *testing.T) {
	now := time.Now()
	failed := "boom"
	open := &taskmodels.Task{Name: "t"}
	done := &taskmodels.Task{Name: "t", DoneAt: &now}

	cases := []struct {
		name   string
		record taskmodels.Sandbox
		owner  *taskmodels.Task
		status string
		want   bool
	}{
		{"crashed", taskmodels.Sandbox{BootedAt: &now}, open, StatusCrashed, true},
		{"stopped without stopped_at", taskmodels.Sandbox{BootedAt: &now}, open, StatusStopped, true},
		{"running", taskmodels.Sandbox{BootedAt: &now}, open, StatusRunning, false},
		{"stopped by agc", taskmodels.Sandbox{BootedAt: &now, StoppedAt: &now}, open, StatusCrashed, false},
		{"task done", taskmodels.Sandbox{BootedAt: &now}, done, StatusCrashed, false},
		{"task gone", taskmodels.Sandbox{BootedAt: &now}, &taskmodels.Task{}, StatusCrashed, false},
		{"never booted", taskmodels.Sandbox{}, open, StatusCrashed, false},
		{"boot failed", taskmodels.Sandbox{BootedAt: &now, BootError: &failed}, open, StatusCrashed, false},
	}

	for _, c := range cases {
		got := Recoverable(&c.record, c.owner, c.status)
		if got != c.want {
			t.Errorf("%s: Recoverable = %v, want %v", c.name, got, c.want)
		}
	}
}
