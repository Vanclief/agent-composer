package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	run("init", "-b", "main")
	err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("hi"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	// macOS tempdirs live behind /private symlinks; match git's view.
	resolved, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestRepoRoot(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{Root: t.TempDir()}
	repo := initRepo(t)

	root, ok, err := manager.RepoRoot(ctx, repo)
	if err != nil || !ok || root != repo {
		t.Fatalf("repo root = %q ok=%v err=%v, want %q", root, ok, err, repo)
	}

	subdir := filepath.Join(repo, "sub")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	root, ok, _ = manager.RepoRoot(ctx, subdir)
	if !ok || root != repo {
		t.Fatalf("subdir should resolve to repo root, got %q ok=%v", root, ok)
	}

	_, ok, err = manager.RepoRoot(ctx, t.TempDir())
	if err != nil || ok {
		t.Fatalf("non-repo should be ok=false, got ok=%v err=%v", ok, err)
	}
}

func TestResolveLifecycle(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{Root: t.TempDir()}
	repo := initRepo(t)

	// Case 3: new branch.
	path, created, err := manager.Resolve(ctx, repo, "feature/x", "")
	if err != nil || !created {
		t.Fatalf("create: path=%q created=%v err=%v", path, created, err)
	}

	// Case 1: reuse.
	again, created, err := manager.Resolve(ctx, repo, "feature/x", "")
	if err != nil || created || again != path {
		t.Fatalf("reuse: path=%q created=%v err=%v, want %q", again, created, err, path)
	}

	// The main checkout's branch is refused.
	_, _, err = manager.Resolve(ctx, repo, "main", "")
	if err == nil {
		t.Fatal("resolving the main branch should be refused")
	}

	infos, err := manager.List(ctx, repo)
	if err != nil || len(infos) != 2 {
		t.Fatalf("list: %v err=%v, want 2 worktrees", infos, err)
	}
	if !infos[0].IsMain || infos[0].Branch != "main" {
		t.Fatalf("first entry should be the main checkout, got %+v", infos[0])
	}

	// Clean removal; branch survives.
	err = manager.Remove(ctx, repo, path, false)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	infos, _ = manager.List(ctx, repo)
	if len(infos) != 1 {
		t.Fatalf("after remove, want 1 worktree, got %v", infos)
	}

	// Case 2: the branch still exists, so resolving checks it out again.
	_, created, err = manager.Resolve(ctx, repo, "feature/x", "")
	if err != nil || !created {
		t.Fatalf("re-checkout existing branch: created=%v err=%v", created, err)
	}
}

func TestRemoteOnlyBranch(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{Root: t.TempDir()}
	repo := initRepo(t)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}

	// A bare origin with a branch that exists only remotely.
	origin := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", origin)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v\n%s", err, output)
	}
	run("remote", "add", "origin", origin)
	run("branch", "remote-only")
	run("push", "origin", "remote-only")
	run("branch", "-D", "remote-only")
	run("fetch", "origin")

	branches, err := manager.Branches(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	var found *Branch
	for i := range branches {
		if branches[i].Name == "remote-only" {
			found = &branches[i]
		}
	}
	if found == nil || found.IsLocal || !found.IsRemote {
		t.Fatalf("remote-only branch not listed correctly: %+v", branches)
	}

	// Resolving it creates a local branch based on origin/remote-only.
	path, created, err := manager.Resolve(ctx, repo, "remote-only", "")
	if err != nil || !created || path == "" {
		t.Fatalf("resolve remote-only: path=%q created=%v err=%v", path, created, err)
	}
}

func TestRemoveRefusesMain(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{Root: t.TempDir()}
	repo := initRepo(t)

	err := manager.Remove(ctx, repo, repo, false)
	if err == nil {
		t.Fatal("removing the main worktree should be refused")
	}
}
