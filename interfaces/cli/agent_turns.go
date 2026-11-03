package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vanclief/ez"
)

// defaultThreadsDir holds a persistent agent's per-thread git worktrees
// (ADR 0016 item 7), on the home's private disk.
const defaultThreadsDir = "/home/agent/threads"

// turnRunner runs a persistent agent's turns (ADR 0016 item 7): one queue
// per session key, so the turns of one thread run in order and never two
// at once, and those of different threads run concurrently, at most limit
// at a time. A turn dispatched while its thread has one queued is folded
// into it: one prompt with every message that arrived meanwhile.
type turnRunner struct {
	take  func(ctx context.Context, t homeTurn)
	clock func() time.Time

	mu    sync.Mutex
	limit int
	// running counts the turns being taken; busy holds their keys.
	running int
	busy    map[string]bool
	// queued holds the next turn of each key, order the keys in the
	// order their turns were queued.
	queued map[string]*homeTurn
	order  []string
	// last is when the last turn ended, the loop's start before any.
	last time.Time
	wg   sync.WaitGroup
}

func newTurnRunner(take func(ctx context.Context, t homeTurn), clock func() time.Time, limit int) *turnRunner {
	return &turnRunner{
		take:   take,
		clock:  clock,
		limit:  max(limit, 1),
		busy:   map[string]bool{},
		queued: map[string]*homeTurn{},
		last:   clock(),
	}
}

// turnsLimit is GET /agent's turns as the loop runs it: 0, from an agc
// that predates the field, is one turn at a time, as before.
func turnsLimit(turns int) int {
	if turns < 1 {
		return 1
	}

	return turns
}

// setLimit changes how many turns run at once; a higher limit starts
// queued turns now, a lower one lets the running turns finish.
func (r *turnRunner) setLimit(ctx context.Context, limit int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limit = max(limit, 1)
	r.startReady(ctx)
}

// dispatch queues t behind its thread's turns and starts it when its
// thread is free and a slot is.
func (r *turnRunner) dispatch(ctx context.Context, t homeTurn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	queued, ok := r.queued[t.key]
	if ok {
		queued.records = append(queued.records, t.records...)
		queued.channel = t.channel
		queued.root = t.root
	} else {
		r.queued[t.key] = &t
		r.order = append(r.order, t.key)
	}

	r.startReady(ctx)
}

// startReady starts the queued turns, oldest first, whose thread runs
// nothing, while there is a free slot. The caller holds mu.
func (r *turnRunner) startReady(ctx context.Context) {
	waiting := []string{}
	for _, key := range r.order {
		if ctx.Err() != nil {
			// The loop is stopping: drop what has not started.
			delete(r.queued, key)
			continue
		}

		if r.busy[key] || r.running >= r.limit {
			waiting = append(waiting, key)
			continue
		}

		t := *r.queued[key]
		delete(r.queued, key)
		r.busy[key] = true
		r.running++
		r.wg.Add(1)

		go r.run(ctx, t)
	}

	r.order = waiting
}

func (r *turnRunner) run(ctx context.Context, t homeTurn) {
	defer r.wg.Done()

	r.take(ctx, t)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.running--
	delete(r.busy, t.key)
	r.last = r.clock()
	r.startReady(ctx)
}

// active reports whether a turn is running or queued.
func (r *turnRunner) active() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.running > 0 || len(r.order) > 0
}

// lastEnd is when the last turn ended.
func (r *turnRunner) lastEnd() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.last
}

// wait blocks until every running and queued turn has ended.
func (r *turnRunner) wait() {
	r.wg.Wait()
}

// harnessDirKey carries the directory a turn's harness runs in.
type harnessDirKey struct{}

// withHarnessDir makes runHarness run in dir; "" keeps the current one.
func withHarnessDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, harnessDirKey{}, dir)
}

// harnessDir is the directory withHarnessDir set, "" for none.
func harnessDir(ctx context.Context) string {
	dir, _ := ctx.Value(harnessDirKey{}).(string)

	return dir
}

// homeRepo is the git repository the home loop runs in, the clone
// AllocateHome made (the home's job runs in it), or "" when dir is not
// the top of a repository.
func homeRepo(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}

	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return ""
	}

	here, err := filepath.EvalSymlinks(dir)
	if err != nil || here != top {
		return ""
	}

	return top
}

// git runs git in dir, its output in the error.
func git(ctx context.Context, dir string, args ...string) error {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return ez.New(ez.EINTERNAL, "git "+strings.Join(args, " ")+" failed: "+strings.TrimSpace(string(out)), err)
	}

	return nil
}

// threadWorktree is where a thread's harness runs: its own git worktree
// of the home's clone, threads/<channel without #>-<root>, created
// detached at HEAD on the thread's first turn and reused after. main, a
// home without a repository, a thread whose session started before it had
// a worktree (Claude Code resumes a session only in the directory it
// started in) and a failed git worktree add run in the loop's own
// directory: "".
func (l *homeLoop) threadWorktree(ctx context.Context, t homeTurn, session string) string {
	if l.repo == "" || l.role == mainRole || t.key == mainSessionKey {
		return ""
	}

	threads := l.threads
	if threads == "" {
		threads = defaultThreadsDir
	}

	path := filepath.Join(threads, fmt.Sprintf("%s-%d", strings.TrimPrefix(t.channel, "#"), t.root))

	_, err := os.Stat(path)
	if err == nil {
		return path
	}
	if !errors.Is(err, os.ErrNotExist) || session != "" {
		return ""
	}

	// Two threads adding worktrees at once would race on the clone's
	// worktree metadata.
	l.gitMu.Lock()
	defer l.gitMu.Unlock()

	err = os.MkdirAll(threads, 0o755)
	if err == nil {
		err = git(ctx, l.repo, "worktree", "add", "--detach", path, "HEAD")
	}
	if err != nil {
		l.logf("cannot add a worktree for %s, running in %s: %s", t.key, l.repo, ez.ErrorMessage(err))
		return ""
	}

	l.logf("added the worktree %s for %s", path, t.key)

	return path
}

// worktreeLine is the line a thread's prompt carries when it runs in its
// own worktree.
func worktreeLine(path string, clone string) string {
	return fmt.Sprintf("You work in the git worktree `%s` of `%s`; other threads of yours run in their own worktrees at the same time, so branch names are shared: fetch, work on a detached HEAD or a branch named after your thread, and push with `git push origin HEAD:<branch>`.\n", path, clone)
}
