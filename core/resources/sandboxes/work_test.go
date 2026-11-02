package sandboxes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/task"
)

// fakeGit answers git commands by their joined args.
type fakeGit struct {
	results map[string]*executor.Result
	fail    bool
	dirs    []string
}

func (g *fakeGit) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	g.dirs = append(g.dirs, cmd.Dir)
	if g.fail {
		return nil, ez.New(ez.EUNAVAILABLE, "sandbox is not reachable", nil)
	}

	result, ok := g.results[cmd.Name+" "+strings.Join(cmd.Args, " ")]
	if !ok {
		return &executor.Result{ExitCode: 1, Stderr: []byte("unexpected command")}, nil
	}

	return result, nil
}

const (
	gitStatus   = "git status --porcelain"
	gitUnpushed = "git rev-list --count HEAD --not --remotes"
)

func out(stdout string) *executor.Result {
	return &executor.Result{Stdout: []byte(stdout)}
}

func TestReadWork(t *testing.T) {
	cases := []struct {
		name     string
		results  map[string]*executor.Result
		dirty    bool
		unpushed int
	}{
		{"clean", map[string]*executor.Result{gitStatus: out(""), gitUnpushed: out("0\n")}, false, 0},
		{"dirty and unpushed", map[string]*executor.Result{gitStatus: out(" M main.go\n?? new.go\n"), gitUnpushed: out("2\n")}, true, 2},
		{"unpushed only", map[string]*executor.Result{gitStatus: out("\n"), gitUnpushed: out("1\n")}, false, 1},
		// A fresh local branch with no upstream still counts its commits:
		// the same command runs with or without one.
		{"no upstream", map[string]*executor.Result{gitStatus: out("?? new.go\n"), gitUnpushed: out("3\n")}, true, 3},
	}

	spec := task.Spec{Repo: "github.com/vanclief/agent-composer"}
	for _, tc := range cases {
		git := &fakeGit{results: tc.results}
		before := time.Now().UTC().Add(-time.Second)

		work, err := readWork(context.Background(), git, spec)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}

		if work.Dirty != tc.dirty || work.Unpushed != tc.unpushed {
			t.Fatalf("%s: got %+v", tc.name, work)
		}

		if work.CheckedAt.Before(before) {
			t.Fatalf("%s: checked_at %s", tc.name, work.CheckedAt)
		}

		for _, dir := range git.dirs {
			if dir != "/home/agent/agent-composer" {
				t.Fatalf("%s: ran git in %q", tc.name, dir)
			}
		}
	}
}

func TestReadWorkFails(t *testing.T) {
	spec := task.Spec{Repo: "github.com/vanclief/agent-composer"}

	cases := []struct {
		name string
		git  *fakeGit
	}{
		{"exec fails", &fakeGit{fail: true}},
		{"not a repository", &fakeGit{results: map[string]*executor.Result{gitStatus: {ExitCode: 128, Stderr: []byte("fatal: not a git repository")}}}},
		{"bad count", &fakeGit{results: map[string]*executor.Result{gitStatus: out(""), gitUnpushed: out("lots\n")}}},
		{"rev-list fails", &fakeGit{results: map[string]*executor.Result{gitStatus: out("")}}},
	}

	for _, tc := range cases {
		work, err := readWork(context.Background(), tc.git, spec)
		if err == nil {
			t.Fatalf("%s: got %+v, want an error", tc.name, work)
		}
	}
}

func TestParseCount(t *testing.T) {
	cases := map[string]int{"0\n": 0, "2\n": 2, " 17 ": 17}
	for in, want := range cases {
		got, err := parseCount(in)
		if err != nil || got != want {
			t.Fatalf("parseCount(%q) = %d, %v", in, got, err)
		}
	}

	for _, in := range []string{"", "-1", "x"} {
		_, err := parseCount(in)
		if err == nil {
			t.Fatalf("parseCount(%q) did not fail", in)
		}
	}
}

func TestParseDirty(t *testing.T) {
	if parseDirty("") || parseDirty("\n") {
		t.Fatal("empty status is dirty")
	}

	if !parseDirty(" M a.go\n") {
		t.Fatal("modified file is clean")
	}
}

// TestGetWork checks Get leaves work null
// for a stopped sandbox and for one without a repo, and that a failed
// check is null, not an error.
func TestGetWork(t *testing.T) {
	env := newTestEnv(t)
	owner := insertTask(t, env.db, "review-42")

	withRepo := insertBooted(t, env.db, owner, "review-42", time.Now())
	withRepo.Spec = []byte(`{"repo":"github.com/o/n"}`)
	err := withRepo.Update(context.Background(), env.db, "spec")
	if err != nil {
		t.Fatal(err)
	}
	insertBooted(t, env.db, owner, "review-42-2", time.Now())

	env.rt.statuses["review-42"] = "running"
	env.rt.statuses["review-42-2"] = "running"

	// The fake sandbox answers every Run with empty stdout and exit 0,
	// so the count is unreadable and the check fails.
	view, err := env.api.Get(context.Background(), nil, &GetRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Work != nil {
		t.Fatalf("failed check gave %+v", view.Work)
	}

	calls := strings.Join(env.rt.Calls(), "\n")
	if !strings.Contains(calls, "run git status --porcelain in /home/agent/n") {
		t.Fatalf("did not check the clone: %s", calls)
	}

	view, err = env.api.Get(context.Background(), nil, &GetRequest{Name: "review-42-2"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Work != nil {
		t.Fatalf("sandbox without repo has work %+v", view.Work)
	}

	env.rt.statuses["review-42"] = "stopped"
	before := len(env.rt.Calls())
	view, err = env.api.Get(context.Background(), nil, &GetRequest{Name: "review-42"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Work != nil || len(env.rt.Calls()) != before {
		t.Fatalf("stopped sandbox was checked: %+v %v", view.Work, env.rt.Calls()[before:])
	}

	env.rt.statuses["review-42"] = "running"
	before = len(env.rt.Calls())
	list, err := env.api.List(context.Background(), nil, &ListRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(env.rt.Calls()) != before {
		t.Fatalf("list without work ran %v", env.rt.Calls()[before:])
	}
	if len(list.Sandboxes) != 2 {
		t.Fatalf("list %+v", list.Sandboxes)
	}
}
