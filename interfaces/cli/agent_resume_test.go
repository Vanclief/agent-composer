package cli

import (
	"context"
	"reflect"
	"testing"

	"github.com/vanclief/ez"
)

func TestAgentLoopSavesItsSessionUnderTheTaskKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	sessions, err := loadSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = sessions.Set("main", "someone-elses")
	if err != nil {
		t.Fatal(err)
	}

	harness := &fakeHarness{}
	loop := &agentLoop{harness: harness, msg: &fakeMessenger{cancel: cancel}, task: "fix-login", name: "fix-login", brief: "# Fix login\n", sessions: sessions}

	err = loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(harness.turns, []string{"|# Fix login\n"}) {
		t.Fatalf("turns = %q, want one on the brief", harness.turns)
	}

	saved := map[string]string{}
	readJSONFile(t, dir+"/"+sessionsFile, &saved)
	want := map[string]string{"main": "someone-elses", "task:fix-login": "s1"}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("sessions file = %v, want %v", saved, want)
	}
}

func TestAgentLoopResumesTheTaskSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dir := t.TempDir()
	sessions, err := loadSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = sessions.Set(taskSessionKey("fix-login"), "old")
	if err != nil {
		t.Fatal(err)
	}

	msg := &fakeMessenger{cancel: cancel}
	harness := &fakeHarness{}
	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", sessions: sessions}

	err = loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(harness.turns, []string{"old|" + resumePrompt}) {
		t.Fatalf("turns = %q, want the resume line on the saved session", harness.turns)
	}
	if !reflect.DeepEqual(msg.posts, []string{"#fix-login me: answer 1"}) {
		t.Fatalf("posts = %q", msg.posts)
	}
	if sessions.Get(taskSessionKey("fix-login")) != "s1" {
		t.Fatalf("saved = %q, want the harness's latest session", sessions.Get(taskSessionKey("fix-login")))
	}
}

// resumeFails fails a resumed session and answers a fresh one.
type resumeFails struct {
	turns []string
}

func (h *resumeFails) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	h.turns = append(h.turns, session+"|"+prompt)
	if session == "gone" {
		return "", "", ez.New(ez.EINTERNAL, "No conversation found with session ID: gone", nil)
	}

	return "started over", "fresh", nil
}

func TestAgentLoopStartsOverWhenTheSessionIsGone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sessions, err := loadSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = sessions.Set(taskSessionKey("fix-login"), "gone")
	if err != nil {
		t.Fatal(err)
	}

	msg := &fakeMessenger{cancel: cancel}
	harness := &resumeFails{}
	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", sessions: sessions}

	err = loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"gone|" + resumePrompt, "|# Fix login\n"}
	if !reflect.DeepEqual(harness.turns, want) {
		t.Fatalf("turns = %q, want %q", harness.turns, want)
	}
	if !reflect.DeepEqual(msg.posts, []string{"#fix-login me: started over"}) {
		t.Fatalf("posts = %q, want only the fresh answer", msg.posts)
	}
	if sessions.Get(taskSessionKey("fix-login")) != "fresh" {
		t.Fatalf("saved = %q, want the fresh session", sessions.Get(taskSessionKey("fix-login")))
	}
}
