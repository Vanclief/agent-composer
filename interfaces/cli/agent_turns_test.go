package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	msgclient "github.com/vanclief/agent-composer/messaging/client"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// gateCall is one harness run held until the test releases it.
type gateCall struct {
	prompt  string
	session string
	release chan struct{}
}

// gateHarness blocks every run until released: each run is sent on calls
// and answers "answer to <session>" once its release is closed.
type gateHarness struct {
	calls chan gateCall

	mu sync.Mutex
	n  int
}

func (h *gateHarness) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	h.mu.Lock()
	h.n++
	next := session
	if next == "" {
		next = fmt.Sprintf("s%d", h.n)
	}
	h.mu.Unlock()

	call := gateCall{prompt: prompt, session: session, release: make(chan struct{})}
	h.calls <- call

	select {
	case <-call.release:
	case <-ctx.Done():
	}

	return "answer on " + next, next, nil
}

// next is the next harness run, failing the test after a while.
func (h *gateHarness) next(t *testing.T) gateCall {
	t.Helper()

	select {
	case call := <-h.calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("no harness run started")
	}

	return gateCall{}
}

// none checks that no harness run starts for a moment.
func (h *gateHarness) none(t *testing.T) {
	t.Helper()

	select {
	case call := <-h.calls:
		t.Fatalf("a run started that should wait: %q", call.prompt)
	case <-time.After(100 * time.Millisecond):
	}
}

func (m *fakeHomeMessenger) postCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.posts)
}

func (m *fakeHomeMessenger) hasSlept() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.slept
}

// waitPosts waits until n answers are posted.
func waitPosts(t *testing.T, msg *fakeHomeMessenger, n int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for msg.postCount() < n {
		if time.Now().After(deadline) {
			t.Fatalf("%d posts, want %d", msg.postCount(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestHomeLoopRunsThreadsAtOnceAndOneThreadInOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The loop reaches its third wait once both batches are dispatched; it
	// holds there until the test is done with the turns.
	third := make(chan struct{})
	proceed := make(chan struct{})

	msg := &fakeHomeMessenger{
		cancel: cancel,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#a", "#b"}, Idle: "15m", Turns: 3},
		script: [][]msgmodels.Message{
			{homeMsg(10, "#a", "franco", 0, "first in a"), homeMsg(20, "#b", "franco", 0, "first in b")},
			{homeMsg(11, "#a", "franco", 10, "more in a"), homeMsg(12, "#a", "franco", 10, "and more in a")},
		},
	}
	msg.onWait = func(n int) {
		if n == 3 {
			close(third)
			<-proceed
		}
	}
	harness := &gateHarness{calls: make(chan gateCall)}
	loop, _ := newHomeLoop(t, msg, &homeHarness{})
	loop.harness = harness

	done := make(chan error)
	go func() {
		done <- loop.run(ctx)
	}()

	// Both threads' turns run at the same time: neither is released.
	first := harness.next(t)
	second := harness.next(t)
	byThread := map[string]gateCall{}
	for _, call := range []gateCall{first, second} {
		if strings.Contains(call.prompt, "first in a") {
			byThread["#a"] = call
		}
		if strings.Contains(call.prompt, "first in b") {
			byThread["#b"] = call
		}
	}
	if len(byThread) != 2 {
		t.Fatalf("the two runs are not one per thread: %q, %q", first.prompt, second.prompt)
	}

	// The second batch is dispatched while they run, and the cursor is
	// saved then, before any answer.
	<-third
	cursor := cursorState{}
	readJSONFile(t, filepath.Join(loop.dir, cursorFile), &cursor)
	if cursor.After != 12 || msg.postCount() != 0 {
		t.Fatalf("cursor %d with %d posts, want 12 with none", cursor.After, msg.postCount())
	}

	// #a's next turn waits for its first, whatever else finishes.
	close(byThread["#b"].release)
	waitPosts(t, msg, 1)
	harness.none(t)

	close(byThread["#a"].release)
	folded := harness.next(t)
	if folded.session != "s1" && folded.session != "s2" {
		t.Fatalf("the next turn in #a does not resume its session: %q", folded.session)
	}
	if !strings.Contains(folded.prompt, "(11): more in a") || !strings.Contains(folded.prompt, "(12): and more in a") || strings.Contains(folded.prompt, "first in a") {
		t.Fatalf("the waiting turn does not fold both messages in:\n%s", folded.prompt)
	}
	close(folded.release)
	waitPosts(t, msg, 3)

	close(proceed)
	err := <-done
	if err != nil {
		t.Fatal(err)
	}

	replies := map[int64]int{}
	for _, post := range msg.posts {
		replies[post.replyTo]++
	}
	if !reflect.DeepEqual(replies, map[int64]int{10: 2, 20: 1}) {
		t.Fatalf("posts = %+v", msg.posts)
	}

	// Both threads' sessions are saved, whichever Set came last.
	sessions := map[string]string{}
	readJSONFile(t, filepath.Join(loop.dir, sessionsFile), &sessions)
	if len(sessions) != 2 || sessions["#a:10"] == "" || sessions["#b:20"] == "" {
		t.Fatalf("sessions = %v", sessions)
	}
}

func TestHomeLoopDoesNotSleepWhileATurnRuns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeHomeMessenger{
		cancel: cancel,
		idle:   true,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm"}, Idle: "90s", Turns: 2},
		script: [][]msgmodels.Message{{homeMsg(1, "#dm", "franco", 0, "a long review")}},
	}
	harness := &gateHarness{calls: make(chan gateCall, 1)}
	loop, clock := newHomeLoop(t, msg, &homeHarness{})
	loop.harness = harness

	// By the sixth wait five minutes have passed with the turn running,
	// well past the 90s idle; then the turn ends.
	sleptEarly := make(chan bool, 1)
	msg.onWait = func(n int) {
		if n == 6 {
			sleptEarly <- msg.hasSlept()
			call := harness.next(t)
			close(call.release)
			waitPosts(t, msg, 1)
		}
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if <-sleptEarly {
		t.Fatal("the loop slept with a turn running")
	}

	// While the turn ran every wait was a full one; it slept 90s after
	// the turn ended, which was 4 minutes in at the earliest.
	for i, timeout := range msg.timeouts[:6] {
		if timeout != time.Minute {
			t.Fatalf("wait %d timed out after %s while the turn ran: %v", i+1, timeout, msg.timeouts)
		}
	}
	if !msg.slept || clock.Before(homeT0.Add(4*time.Minute+90*time.Second)) {
		t.Fatalf("slept %v at %s", msg.slept, clock.Sub(homeT0))
	}
}

// gitRepo makes a repository with one commit, skipping without git.
func gitRepo(t *testing.T) string {
	t.Helper()

	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	repo := filepath.Join(dir, "agent-composer")
	err = os.Mkdir(repo, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "first"},
	} {
		err = git(context.Background(), repo, args...)
		if err != nil {
			t.Fatal(err)
		}
	}

	return repo
}

func worktreeList(t *testing.T, repo string) string {
	t.Helper()

	out, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree list: %s", out)
	}

	return string(out)
}

func TestHomeLoopRunsEachThreadInItsOwnWorktree(t *testing.T) {
	repo := gitRepo(t)
	threads := filepath.Join(filepath.Dir(repo), "threads")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A worktree whose directory is gone is pruned at start.
	gone := filepath.Join(threads, "gone-1")
	err := git(ctx, repo, "worktree", "add", "--detach", gone, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll(gone)
	if err != nil {
		t.Fatal(err)
	}

	msg := &fakeHomeMessenger{
		cancel: cancel,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#b", "#dm", "#c"}, Idle: "15m", Turns: 1},
		script: [][]msgmodels.Message{
			{homeMsg(11, "#b", "franco", 0, "review fix-login"), homeMsg(10, "#dm", "franco", 0, "hi")},
			{homeMsg(12, "#b", "franco", 11, "and now?"), homeMsg(31, "#c", "franco", 30, "still on it?")},
		},
	}
	harness := &homeHarness{}
	loop, _ := newHomeLoop(t, msg, harness)
	loop.repo = repo
	loop.threads = threads

	// #c:30 has a session from before worktrees: it keeps the clone.
	err = writeJSONFile(filepath.Join(loop.dir, sessionsFile), map[string]string{"#c:30": "kept"})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	b := filepath.Join(threads, "b-11")
	dm := filepath.Join(threads, "dm-10")
	wantDirs := []string{b, dm, b, ""}
	if !reflect.DeepEqual(harness.dirs, wantDirs) {
		t.Fatalf("dirs = %q, want %q", harness.dirs, wantDirs)
	}

	line := worktreeLine(b, repo)
	if !strings.Contains(harness.turns[0].prompt, line) || !strings.Contains(harness.turns[2].prompt, line) || strings.Contains(harness.turns[3].prompt, "git worktree") {
		t.Fatalf("turns = %#v", harness.turns)
	}

	list := worktreeList(t, repo)
	for _, path := range []string{b, dm} {
		if !strings.Contains(list, "worktree "+path+"\n") {
			t.Fatalf("%s is not a worktree of the clone:\n%s", path, list)
		}
	}
	if strings.Count(list, "detached") != 2 || strings.Contains(list, gone) {
		t.Fatalf("worktrees:\n%s", list)
	}

	// main and a home without a clone run where the loop runs.
	turn := homeTurn{key: "#b:11", channel: "#b", root: 11}
	loop.role = mainRole
	if loop.threadWorktree(ctx, turn, "") != "" {
		t.Fatal("main got a worktree")
	}
	loop.role = "reviewer"
	loop.repo = ""
	if loop.threadWorktree(ctx, turn, "") != "" {
		t.Fatal("a home without a clone got a worktree")
	}
}

func TestHomeRepoIsTheTopOfARepository(t *testing.T) {
	repo := gitRepo(t)
	ctx := context.Background()

	sub := filepath.Join(repo, "sub")
	err := os.Mkdir(sub, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	if homeRepo(ctx, repo) != repo {
		t.Fatalf("homeRepo(%s) = %q", repo, homeRepo(ctx, repo))
	}
	if homeRepo(ctx, sub) != "" || homeRepo(ctx, filepath.Dir(repo)) != "" {
		t.Fatal("a directory that is not a repository's top is a repo")
	}
}

func TestTurnsLimit(t *testing.T) {
	cases := map[int]int{0: 1, -1: 1, 1: 1, 3: 3, 8: 8}
	for turns, want := range cases {
		if turnsLimit(turns) != want {
			t.Errorf("turnsLimit(%d) = %d, want %d", turns, turnsLimit(turns), want)
		}
	}
}
