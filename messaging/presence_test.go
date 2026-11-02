package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/vanclief/ez"
)

// fakePresence says which sandboxes exist and run, and which tasks have
// a running sandbox.
type fakePresence struct {
	sandboxes map[string]bool
	tasks     map[string]bool
}

func (f fakePresence) SandboxRunning(_ context.Context, name string) (bool, bool) {
	running, found := f.sandboxes[name]
	return running, found
}

func (f fakePresence) TaskRunning(_ context.Context, task string) bool {
	return f.tasks[task]
}

func onlyView(t *testing.T, svc *Service, id string) ParticipantView {
	t.Helper()

	view, err := svc.Participant(context.Background(), human, id)
	if err != nil {
		t.Fatal(err)
	}

	return *view
}

// sameTime allows for the database keeping microseconds only.
func sameTime(a, b time.Time) bool {
	return a.Sub(b).Abs() < time.Millisecond
}

func TestAgentOnlineFollowsItsSandbox(t *testing.T) {
	svc := newTestService(t)

	up := join(t, svc, review, "review-42-a")
	down := join(t, svc, review, "review-42-b")
	loose := join(t, svc, review, "helper")
	m := post(t, svc, review, up, "#review-42", "working")

	// Unwired: an agent that has not left is online.
	if !onlyView(t, svc, down.ID).Online {
		t.Fatal("without presence an active agent should be online")
	}

	svc.SetPresence(fakePresence{
		sandboxes: map[string]bool{"review-42-a": true, "review-42-b": false},
		tasks:     map[string]bool{"review-42": true},
	})

	view := onlyView(t, svc, up.ID)
	if !view.Online {
		t.Fatal("agent with a running sandbox should be online")
	}
	if view.LastSeenAt == nil || !sameTime(*view.LastSeenAt, m.CreatedAt) {
		t.Fatalf("last_seen_at should be the latest message %v, got %v", m.CreatedAt, view.LastSeenAt)
	}

	view = onlyView(t, svc, down.ID)
	if view.Online {
		t.Fatal("agent with a stopped sandbox should be offline, even when another of its task runs")
	}
	if view.LastSeenAt != nil {
		t.Fatalf("agent that never posted has no last_seen_at, got %v", view.LastSeenAt)
	}

	// No sandbox by that name: any running sandbox of its task counts.
	if !onlyView(t, svc, loose.ID).Online {
		t.Fatal("agent named after no sandbox should follow its task")
	}

	svc.SetPresence(fakePresence{sandboxes: map[string]bool{}, tasks: map[string]bool{}})
	if onlyView(t, svc, loose.ID).Online {
		t.Fatal("agent whose task runs nothing should be offline")
	}
}

func TestHumanOnlineWithinFiveMinutes(t *testing.T) {
	svc := newTestService(t)

	recent := join(t, svc, human, "franco")
	stale := join(t, svc, human, "maria")
	never := join(t, svc, human, "nobody")

	oneMin := time.Now().Add(-time.Minute)
	tenMin := time.Now().Add(-10 * time.Minute)
	svc.MarkSeen(recent.ID, oneMin)
	svc.MarkSeen(stale.ID, tenMin)

	view := onlyView(t, svc, recent.ID)
	if !view.Online || view.LastSeenAt == nil || !view.LastSeenAt.Equal(oneMin) {
		t.Fatalf("human seen a minute ago should be online at that time, got %+v", view)
	}

	view = onlyView(t, svc, stale.ID)
	if view.Online || view.LastSeenAt == nil || !view.LastSeenAt.Equal(tenMin) {
		t.Fatalf("human seen ten minutes ago should be offline at that time, got %+v", view)
	}

	view = onlyView(t, svc, never.ID)
	if view.Online || view.LastSeenAt != nil {
		t.Fatalf("human never seen should be offline with no last_seen_at, got %+v", view)
	}

	// A post is a sighting too: the latest of seen and posted wins.
	m := post(t, svc, human, stale, "#review-42", "back")
	view = onlyView(t, svc, stale.ID)
	if !view.Online || !sameTime(*view.LastSeenAt, m.CreatedAt) {
		t.Fatalf("human who just posted should be online, got %+v", view)
	}
}

func TestLeftAndSystemAreNeverOnline(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.SetPresence(fakePresence{sandboxes: map[string]bool{"review-42-a": true}, tasks: map[string]bool{"review-42": true}})

	agent := join(t, svc, review, "review-42-a")
	person := join(t, svc, human, "franco")
	post(t, svc, review, agent, "#review-42", "bye")
	svc.MarkSeen(person.ID, time.Now())

	for _, id := range []string{agent.ID, person.ID} {
		err := svc.Leave(ctx, human, id)
		if err != nil {
			t.Fatal(err)
		}

		view := onlyView(t, svc, id)
		if view.Online {
			t.Fatalf("left participant %s should be offline", view.Name)
		}
		if view.LastSeenAt == nil {
			t.Fatalf("left participant %s keeps its last_seen_at", view.Name)
		}
	}

	m, err := svc.PostAsSystem(ctx, "#review-42", "opening")
	if err != nil {
		t.Fatal(err)
	}

	views, err := svc.Participants(ctx, human, "#review-42")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, view := range views {
		if view.ID != m.ParticipantID {
			continue
		}

		found = true
		if view.Online {
			t.Fatal("agc should never be online")
		}
	}
	if !found {
		t.Fatalf("agc should be listed among participants, got %+v", views)
	}
}

func TestParticipantVisibilityForTasks(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	person := join(t, svc, human, "franco")
	post(t, svc, human, person, "#franco", "private")

	_, err := svc.Participant(ctx, review, person.ID)
	wantCode(t, err, ez.ENOTFOUND)

	post(t, svc, human, person, "#releases", "v1")

	_, err = svc.Participant(ctx, review, person.ID)
	if err != nil {
		t.Fatalf("a task should see a participant who posted in its grant: %v", err)
	}
}
