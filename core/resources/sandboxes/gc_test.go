package sandboxes

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/vanclief/agent-composer/core/events"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// insertDone inserts a task done at doneAt, or not done when doneAt is
// zero.
func insertDone(t *testing.T, env *testEnv, name string, doneAt time.Time) *taskmodels.Task {
	t.Helper()

	record := &taskmodels.Task{Title: "Title of " + name, Name: name, Brief: "Do " + name + "."}
	if !doneAt.IsZero() {
		record.DoneAt = &doneAt
	}
	err := record.Insert(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}

	return record
}

// gcEnv has, with the clock at now: old-done (done 25h ago, stopped, a
// 50G disk), fresh-done (done 1h ago, stopped), running-done (done 25h
// ago, running), open-task (not done, stopped) and home (an agent's
// home, stopped).
func gcEnv(t *testing.T) (*testEnv, time.Time) {
	t.Helper()

	env := newTestEnv(t)
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	env.api.now = func() time.Time {
		return now
	}
	booted := now.Add(-72 * time.Hour)

	old := insertDone(t, env, "old-done", now.Add(-25*time.Hour))
	record := insertBooted(t, env.db, old, "old-done", booted)
	record.Spec = []byte(`{"disk":"50G"}`)
	record.Volume = "agc-old-done"
	err := record.Update(context.Background(), env.db, "spec", "volume")
	if err != nil {
		t.Fatal(err)
	}
	env.rt.statuses["old-done"] = StatusStopped

	fresh := insertDone(t, env, "fresh-done", now.Add(-time.Hour))
	insertBooted(t, env.db, fresh, "fresh-done", booted)
	env.rt.statuses["fresh-done"] = StatusStopped

	running := insertDone(t, env, "running-done", now.Add(-25*time.Hour))
	insertBooted(t, env.db, running, "running-done", booted)
	env.rt.statuses["running-done"] = StatusRunning

	open := insertDone(t, env, "open-task", time.Time{})
	insertBooted(t, env.db, open, "open-task", booted)
	env.rt.statuses["open-task"] = StatusStopped

	home := &taskmodels.Sandbox{Agent: "ada", Name: "home", Spec: []byte(`{}`), BootedAt: &booted, CreatedAt: booted}
	err = home.Insert(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}
	env.rt.statuses["home"] = StatusStopped

	return env, now
}

func sandboxNames(t *testing.T, env *testEnv) []string {
	t.Helper()

	records, err := taskmodels.ListSandboxes(context.Background(), env.db)
	if err != nil {
		t.Fatal(err)
	}

	names := []string{}
	for _, record := range records {
		names = append(names, record.Name)
	}
	sort.Strings(names)

	return names
}

func TestGCDeletesOnlyStoppedSandboxesOfTasksDoneLongAgo(t *testing.T) {
	env, _ := gcEnv(t)
	bus := events.New()
	env.api.SetEvents(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	feed := bus.Subscribe(ctx)

	got, err := env.api.collect(context.Background(), 24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}

	want := []Collected{{Name: "old-done", Task: "old-done", DoneFor: "25h0m0s", Disk: "50G", DiskMiB: 50 * 1024}}
	if !reflect.DeepEqual(got.Sandboxes, want) || got.Freed != 1 || got.FreedDisk != "50G" {
		t.Fatalf("collected %+v", got)
	}

	if !reflect.DeepEqual(env.rt.Calls(), []string{"remove old-done"}) {
		t.Fatalf("calls = %q", env.rt.Calls())
	}

	left := sandboxNames(t, env)
	if !reflect.DeepEqual(left, []string{"fresh-done", "home", "open-task", "running-done"}) {
		t.Fatalf("left %q", left)
	}

	deleted := []string{}
	for len(feed) > 0 {
		event := <-feed
		status, ok := event.Data.(StatusEvent)
		if ok && status.Sandbox.Status == StatusDeleted {
			deleted = append(deleted, status.Sandbox.Name)
		}
	}
	if !reflect.DeepEqual(deleted, []string{"old-done"}) {
		t.Fatalf("sandbox.status deleted for %q", deleted)
	}
}

// The runtime's Remove never touches a named volume; nothing in the
// collector names one either, so agc-old-done stays for its transcripts.
func TestGCKeepsTheVolume(t *testing.T) {
	env, _ := gcEnv(t)

	_, err := env.api.collect(context.Background(), 24*time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}

	for _, call := range env.rt.Calls() {
		if call != "remove old-done" {
			t.Fatalf("unexpected runtime call %q", call)
		}
	}
}

func TestGCFollowsTheClock(t *testing.T) {
	env, now := gcEnv(t)

	// Two hours later fresh-done is still within 24h.
	env.api.now = func() time.Time {
		return now.Add(2 * time.Hour)
	}
	got, err := env.api.collect(context.Background(), 24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sandboxes) != 1 || got.Sandboxes[0].Name != "old-done" {
		t.Fatalf("at +2h: %+v", got.Sandboxes)
	}

	// A day later it is past.
	env.api.now = func() time.Time {
		return now.Add(24 * time.Hour)
	}
	got, err = env.api.collect(context.Background(), 24*time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, item := range got.Sandboxes {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"fresh-done", "old-done"}) {
		t.Fatalf("at +24h: %q", names)
	}
}

func TestGCDryRunDeletesNothing(t *testing.T) {
	env, _ := gcEnv(t)

	got, err := env.api.GC(context.Background(), nil, &GCRequest{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Sandboxes) != 1 || got.Sandboxes[0].Name != "old-done" || got.Freed != 0 || !got.DryRun || got.Keep != "24h0m0s" {
		t.Fatalf("dry run %+v", got)
	}
	if len(env.rt.Calls()) != 0 || len(sandboxNames(t, env)) != 5 {
		t.Fatalf("dry run changed something: calls %q", env.rt.Calls())
	}
}

func TestGCKeepOverridesTheGracePeriod(t *testing.T) {
	env, _ := gcEnv(t)

	got, err := env.api.GC(context.Background(), nil, &GCRequest{Keep: "30m"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Freed != 2 {
		t.Fatalf("keep 30m: %+v", got)
	}

	left := sandboxNames(t, env)
	if !reflect.DeepEqual(left, []string{"home", "open-task", "running-done"}) {
		t.Fatalf("left %q", left)
	}

	_, err = env.api.GC(context.Background(), nil, &GCRequest{Keep: "soon"})
	if err == nil {
		t.Fatal("keep soon was accepted")
	}
}

func TestStartGCOffWhenKeepDoneIsZero(t *testing.T) {
	env, _ := gcEnv(t)
	env.api.keepDone = 0

	// Returns at once without a run.
	env.api.StartGC(context.Background())

	if len(env.rt.Calls()) != 0 {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestStartGCRunsAtOnceAndStops(t *testing.T) {
	env, _ := gcEnv(t)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		env.api.StartGC(ctx)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for len(env.rt.Calls()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if !reflect.DeepEqual(env.rt.Calls(), []string{"remove old-done"}) {
		t.Fatalf("calls = %q", env.rt.Calls())
	}
}

func TestListCarriesDisk(t *testing.T) {
	env, _ := gcEnv(t)

	list, err := env.api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}

	disks := map[string]string{}
	for _, view := range list.Sandboxes {
		disks[view.Name] = view.Disk
	}
	if disks["old-done"] != "50G" || disks["fresh-done"] != "20G" || disks["home"] != "20G" {
		t.Fatalf("disks %v", disks)
	}
}
