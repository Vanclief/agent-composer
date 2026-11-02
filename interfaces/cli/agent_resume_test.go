package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

func writeState(t *testing.T, path string, state agentState) {
	t.Helper()

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func readState(t *testing.T, path string) agentState {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	state := agentState{}
	err = json.Unmarshal(data, &state)
	if err != nil {
		t.Fatal(err)
	}

	return state
}

func TestAgentLoopSavesItsSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), ".harness", "agc-agent.json")
	msg := &fakeMessenger{
		cancel:  cancel,
		lastIDs: map[string]int64{"#fix-login": 2},
		script:  [][]msgmodels.Message{{message(3, "orchestrator", "p-orch", "Next?")}},
	}
	loop := &agentLoop{
		harness:     &fakeHarness{answers: map[int]string{2: "DONE: fixed"}},
		msg:         msg,
		task:        "fix-login",
		name:        "fix-login",
		brief:       "# Fix login\n",
		harnessName: "claude_code",
		statePath:   path,
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := agentState{Harness: "claude_code", Task: "fix-login", Session: "s1", Cursor: 3, Delivered: true}
	got := readState(t, path)
	if got != want {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
}

func TestAgentLoopResumesItsSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), "agc-agent.json")
	writeState(t, path, agentState{Harness: "claude_code", Task: "fix-login", Session: "old", Cursor: 4})

	own := message(5, "fix-login", "p-old", "my answer before the crash")
	own.Task = "fix-login"
	msg := &fakeMessenger{
		cancel:  cancel,
		lastIDs: map[string]int64{"#fix-login": 7},
		script: [][]msgmodels.Message{
			// What was posted while the sandbox was down: its own last
			// answer (not a turn), agc's recovery line (not a turn), and
			// the manager's message (a turn).
			{own, fromAgc(6, "fix-login came back after a restart; its agent continues."), message(7, "orchestrator", "p-orch", "Still there?")},
		},
	}
	harness := &fakeHarness{}
	loop := &agentLoop{
		harness:     harness,
		msg:         msg,
		task:        "fix-login",
		name:        "fix-login",
		brief:       "# Fix login\n",
		harnessName: "claude_code",
		statePath:   path,
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	wantTurns := []string{
		"old|" + resumePrompt,
		"s1|@orchestrator in #fix-login:\n\nStill there?",
	}
	if !reflect.DeepEqual(harness.turns, wantTurns) {
		t.Fatalf("turns:\n%s", strings.Join(harness.turns, "\n---\n"))
	}
	if msg.waits[0] != 4 {
		t.Fatalf("first wait after %d, want the saved cursor 4", msg.waits[0])
	}
}

func TestAgentLoopInReviewWaitsAfterARestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), "agc-agent.json")
	writeState(t, path, agentState{Harness: "claude_code", Task: "fix-login", Session: "old", Cursor: 4, Delivered: true})

	msg := &fakeMessenger{cancel: cancel, lastIDs: map[string]int64{"#fix-login": 4}}
	harness := &fakeHarness{}
	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", harnessName: "claude_code", statePath: path}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(harness.turns) != 0 || len(msg.posts) != 0 {
		t.Fatalf("turns = %q, posts = %q; want none while in review", harness.turns, msg.posts)
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

	dir := t.TempDir()
	path := filepath.Join(dir, "agc-agent.json")
	writeState(t, path, agentState{Harness: "claude_code", Task: "fix-login", Session: "gone"})

	msg := &fakeMessenger{cancel: cancel}
	harness := &resumeFails{}
	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", harnessName: "claude_code", statePath: path}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"gone|" + resumePrompt, "|# Fix login\n"}
	if !reflect.DeepEqual(harness.turns, want) {
		t.Fatalf("turns = %q, want %q", harness.turns, want)
	}
	if readState(t, path).Session != "fresh" {
		t.Fatalf("state = %+v, want the fresh session", readState(t, path))
	}
}

func TestAgentLoopIgnoresAnotherHarnesssSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	path := filepath.Join(t.TempDir(), "agc-agent.json")
	writeState(t, path, agentState{Harness: "codex_cli", Task: "fix-login", Session: "codex-thread"})

	msg := &fakeMessenger{cancel: cancel}
	harness := &fakeHarness{}
	loop := &agentLoop{harness: harness, msg: msg, task: "fix-login", name: "fix-login", brief: "# Fix login\n", harnessName: "claude_code", statePath: path}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(harness.turns, []string{"|# Fix login\n"}) {
		t.Fatalf("turns = %q", harness.turns)
	}
}
