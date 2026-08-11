// Package worktree wraps the git CLI to manage linked worktrees.
// Git is the source of truth: nothing is persisted here — every
// operation reads or mutates what `git worktree` already knows.
package worktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vanclief/ez"
)

// Manager creates and inspects worktrees under a global root directory.
type Manager struct {
	// Root is the directory where new worktrees are created,
	// e.g. ~/.agent_composer/worktrees.
	Root string
}

// DefaultRoot is where worktrees live unless configured otherwise.
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "agent_composer_worktrees")
	}
	return filepath.Join(home, ".agent_composer", "worktrees")
}

// Info describes one worktree of a repository.
type Info struct {
	Path     string `json:"path"`
	Branch   string `json:"branch,omitempty"`
	Head     string `json:"head,omitempty"`
	IsMain   bool   `json:"is_main"`
	Detached bool   `json:"detached"`
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", ez.New(ez.EINTERNAL, message, err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RepoRoot resolves the enclosing git repository of path. ok is false
// when path is not inside a git repository (not an error — callers use
// it to decide whether to offer worktrees at all).
func (m *Manager) RepoRoot(ctx context.Context, path string) (string, bool, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", false, ez.Wrap(err)
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", false, nil
	}

	root, err := runGit(ctx, abs, "rev-parse", "--show-toplevel")
	if err != nil {
		// Non-zero exit means "not a repository" here, not a failure.
		return "", false, nil
	}

	return root, true, nil
}

// List returns every worktree of the repository, main checkout first.
func (m *Manager) List(ctx context.Context, repo string) ([]Info, error) {
	output, err := runGit(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	infos := []Info{}
	current := Info{}
	flush := func() {
		if current.Path != "" {
			current.IsMain = len(infos) == 0
			infos = append(infos, current)
		}
		current = Info{}
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(
				strings.TrimPrefix(line, "branch "),
				"refs/heads/",
			)
		case line == "detached":
			current.Detached = true
		}
	}
	flush()

	return infos, nil
}

func (m *Manager) branchExists(ctx context.Context, repo, branch string) bool {
	_, err := runGit(
		ctx, repo,
		"show-ref", "--verify", "--quiet", "refs/heads/"+branch,
	)
	return err == nil
}

func (m *Manager) remoteBranchExists(ctx context.Context, repo, branch string) bool {
	_, err := runGit(
		ctx, repo,
		"show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch,
	)
	return err == nil
}

// Branch is a branch name and where it is known from.
type Branch struct {
	Name     string `json:"name"`
	IsLocal  bool   `json:"is_local"`
	IsRemote bool   `json:"is_remote"`
}

// Branches lists local and origin branches (as of the last fetch).
func (m *Manager) Branches(ctx context.Context, repo string) ([]Branch, error) {
	output, err := runGit(
		ctx, repo,
		"for-each-ref", "--format=%(refname)",
		"refs/heads", "refs/remotes/origin",
	)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	index := map[string]*Branch{}
	order := []string{}
	record := func(name string, remote bool) {
		if name == "" || name == "HEAD" {
			return
		}
		branch, known := index[name]
		if !known {
			branch = &Branch{Name: name}
			index[name] = branch
			order = append(order, name)
		}
		if remote {
			branch.IsRemote = true
		} else {
			branch.IsLocal = true
		}
	}

	for _, line := range strings.Split(output, "\n") {
		ref := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(ref, "refs/heads/"):
			record(strings.TrimPrefix(ref, "refs/heads/"), false)
		case strings.HasPrefix(ref, "refs/remotes/origin/"):
			record(strings.TrimPrefix(ref, "refs/remotes/origin/"), true)
		}
	}

	branches := make([]Branch, 0, len(order))
	for _, name := range order {
		branches = append(branches, *index[name])
	}
	return branches, nil
}

// Fetch updates origin refs so remote branches are current.
func (m *Manager) Fetch(ctx context.Context, repo string) error {
	_, err := runGit(ctx, repo, "fetch", "origin", "--prune")
	if err != nil {
		return ez.Wrap(err)
	}
	return nil
}

var unsafePathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func shortHash(value string, length int) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:length]
}

func (m *Manager) worktreeDir(repo, branch string) string {
	repoKey := filepath.Base(repo) + "-" + shortHash(repo, 8)
	branchKey := unsafePathChars.ReplaceAllString(branch, "-")
	dir := filepath.Join(m.Root, repoKey, branchKey)

	// A same-named dir left by a *different* branch gets a suffix.
	if _, err := os.Stat(dir); err == nil {
		dir = dir + "-" + shortHash(branch, 6)
	}
	return dir
}

// Resolve returns the worktree directory for (repo, branch):
//  1. the branch already has a worktree      → reuse its path
//  2. the branch exists without a worktree   → check it out
//  3. the branch does not exist              → create it from base (default HEAD)
func (m *Manager) Resolve(ctx context.Context, repo, branch, base string) (string, bool, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", false, ez.New(ez.EINVALID, "A branch name is required", nil)
	}

	existing, err := m.List(ctx, repo)
	if err != nil {
		return "", false, ez.Wrap(err)
	}
	for _, info := range existing {
		if info.Branch == branch {
			if info.IsMain {
				return "", false, ez.New(
					ez.EINVALID,
					"Branch "+branch+" is checked out in the main worktree at "+info.Path+"; pick another branch",
					nil,
				)
			}
			return info.Path, false, nil
		}
	}

	dir := m.worktreeDir(repo, branch)
	err = os.MkdirAll(filepath.Dir(dir), 0o755)
	if err != nil {
		return "", false, ez.Wrap(err)
	}

	if m.branchExists(ctx, repo, branch) {
		_, err = runGit(ctx, repo, "worktree", "add", dir, branch)
		if err != nil {
			return "", false, ez.Wrap(err)
		}
		return normalizePath(dir), true, nil
	}

	// A branch known only from origin: create the local branch from
	// the remote-tracking ref (tracking is set up by git's defaults).
	if strings.TrimSpace(base) == "" && m.remoteBranchExists(ctx, repo, branch) {
		base = "origin/" + branch
	}

	base = strings.TrimSpace(base)
	if strings.HasPrefix(base, "origin/") || strings.HasPrefix(base, "refs/remotes/") {
		// Best effort: a stale remote base is worse than a slow fetch.
		_, _ = runGit(ctx, repo, "fetch", "origin")
	}
	if base == "" {
		base = "HEAD"
	}

	_, err = runGit(ctx, repo, "worktree", "add", "-b", branch, dir, base)
	if err != nil {
		return "", false, ez.Wrap(err)
	}
	return normalizePath(dir), true, nil
}

// normalizePath matches git's own reporting (macOS tempdirs are
// symlinked under /private, and `git worktree list` prints the target).
func normalizePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// Remove deletes a worktree (never its branch). Git refuses dirty
// worktrees unless force is set — that refusal is surfaced verbatim.
func (m *Manager) Remove(ctx context.Context, repo, path string, force bool) error {
	infos, err := m.List(ctx, repo)
	if err != nil {
		return ez.Wrap(err)
	}
	for _, info := range infos {
		if info.Path == path && info.IsMain {
			return ez.New(ez.EINVALID, "Refusing to remove the main worktree", nil)
		}
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	_, err = runGit(ctx, repo, args...)
	if err != nil {
		return ez.Wrap(err)
	}
	return nil
}

// Prune drops stale administrative entries for manually deleted dirs.
func (m *Manager) Prune(ctx context.Context, repo string) error {
	_, err := runGit(ctx, repo, "worktree", "prune")
	if err != nil {
		return ez.Wrap(err)
	}
	return nil
}
