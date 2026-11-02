package watchdog

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
)

func TestCrashedSandboxIsRestartedThreeTimesAnHour(t *testing.T) {
	e := newEnv(t)
	e.probe.status = sandboxes.StatusCrashed

	e.at(t, time.Minute)
	e.at(t, 2*time.Minute)
	e.at(t, 3*time.Minute)
	if e.probe.restarted != 3 {
		t.Fatalf("restarted %d times, want 3", e.probe.restarted)
	}

	e.at(t, 4*time.Minute)
	e.at(t, 5*time.Minute)
	if e.probe.restarted != 3 {
		t.Fatalf("restarted %d times after the limit, want 3", e.probe.restarted)
	}

	want := []string{
		"dms-whatsapp crashed; agc restarted it (restart 1 of 3 this hour) and its agent continues.",
		"dms-whatsapp crashed; agc restarted it (restart 2 of 3 this hour) and its agent continues.",
		"dms-whatsapp crashed; agc restarted it (restart 3 of 3 this hour) and its agent continues.",
		"agc gave up restarting dms-whatsapp: it crashed again after 3 restarts within an hour. It stays down until someone starts it.",
	}
	got := e.posts(t, "#dms-whatsapp")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("#dms-whatsapp = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(e.posts(t, "#franco"), want) {
		t.Fatalf("#franco = %q, want %q", e.posts(t, "#franco"), want)
	}

	// Someone starts it; a later crash is restarted again once the
	// hour has passed.
	e.probe.status = sandboxes.StatusRunning
	e.at(t, 70*time.Minute)
	e.probe.status = sandboxes.StatusCrashed
	e.at(t, 71*time.Minute)
	if e.probe.restarted != 4 {
		t.Fatalf("restarted %d times after a manual start, want 4", e.probe.restarted)
	}
}

func TestFailedRestartIsSaidAndCountsAsATry(t *testing.T) {
	e := newEnv(t)
	e.probe.status = sandboxes.StatusCrashed
	e.probe.restartErr = ez.New(ez.EINTERNAL, "msb: no such image", nil)
	e.dog.settings.Restarts = 1

	e.at(t, time.Minute)
	e.at(t, 2*time.Minute)
	e.at(t, 3*time.Minute)

	want := []string{
		"dms-whatsapp crashed and agc could not restart it (restart 1 of 1 this hour): msb: no such image",
		"agc gave up restarting dms-whatsapp: 1 restart within an hour, the last failed with: msb: no such image. It stays down until someone starts it.",
	}
	got := e.posts(t, "#dms-whatsapp")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("#dms-whatsapp = %q, want %q", got, want)
	}
}

func TestStoppedOrDoneSandboxIsNotRestarted(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.probe.status = sandboxes.StatusCrashed

	stopped := start
	e.box.StoppedAt = &stopped
	err := e.box.Update(ctx, e.db, "stopped_at")
	if err != nil {
		t.Fatal(err)
	}

	e.at(t, time.Minute)
	if e.probe.restarted != 0 {
		t.Fatalf("restarted a sandbox agc stopped")
	}

	e.dog.settings.Restarts = 0
	e.box.StoppedAt = nil
	err = e.box.Update(ctx, e.db, "stopped_at")
	if err != nil {
		t.Fatal(err)
	}

	e.at(t, 2*time.Minute)
	if e.probe.restarted != 0 || len(e.posts(t, "#dms-whatsapp")) != 0 {
		t.Fatalf("restarted with restarts: 0")
	}
}

func TestRestartWithoutTheAgentIsSaid(t *testing.T) {
	e := newEnv(t)
	e.probe.status = sandboxes.StatusCrashed
	e.probe.restartErr = &sandboxes.AgentNotRelaunched{Reason: "its agent was not relaunched: the sandbox's image predates `agc agent run`"}

	e.at(t, time.Minute)

	want := []string{"dms-whatsapp crashed; agc restarted it (restart 1 of 3 this hour), but its agent was not relaunched: the sandbox's image predates `agc agent run`."}
	got := e.posts(t, "#dms-whatsapp")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("#dms-whatsapp = %q, want %q", got, want)
	}
}
