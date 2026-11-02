package sandboxes

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/task"
)

// workTimeout bounds the git commands that check a clone.
const workTimeout = 10 * time.Second

// Work is what would be lost if a sandbox were stopped or deleted: its
// clone's uncommitted changes and commits on no remote branch (ADR 0008
// item 6). It is read live and never cached.
type Work struct {
	// Dirty is true when git status --porcelain prints any line.
	Dirty bool `json:"dirty"`
	// Unpushed counts the commits reachable from HEAD that are on no
	// remote-tracking branch, upstream or not: what would be lost.
	Unpushed  int       `json:"unpushed"`
	CheckedAt time.Time `json:"checked_at"`
}

// addWork sets view.Work when the sandbox is running and has a repo.
func (api *API) addWork(ctx context.Context, view *SandboxView) {
	if view.Status != StatusRunning || strings.TrimSpace(view.Spec.Repo) == "" {
		return
	}

	view.Work = api.checkWork(ctx, view.Name, view.Spec)
}

// checkWork reads the work in a running sandbox's clone. Any failure is
// logged once and yields nil, never an error.
func (api *API) checkWork(ctx context.Context, name string, spec task.Spec) *Work {
	ctx, cancel := context.WithTimeout(ctx, workTimeout)
	defer cancel()

	sb, err := api.rt.Connect(ctx, name)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", name).Msg("cannot check the clone for unpushed work")
		return nil
	}
	defer func() {
		_ = sb.Detach(context.Background())
	}()

	work, err := readWork(ctx, sb, spec)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", name).Msg("cannot check the clone for unpushed work")
		return nil
	}

	return work
}

// runner is the part of a sandbox readWork needs, so tests can fake it.
type runner interface {
	Run(ctx context.Context, cmd executor.Command) (*executor.Result, error)
}

// readWork runs git in the spec's clone through exec.
func readWork(ctx context.Context, exec runner, spec task.Spec) (*Work, error) {
	_, repoDir, err := spec.RepoURL()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	status, err := gitOutput(ctx, exec, repoDir, "status", "--porcelain")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	work := &Work{Dirty: parseDirty(status), CheckedAt: time.Now().UTC()}

	count, err := gitOutput(ctx, exec, repoDir, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return nil, ez.Wrap(err)
	}

	work.Unpushed, err = parseCount(count)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return work, nil
}

// gitOutput runs git with args in dir and returns its stdout, failing on a
// non-zero exit with the tail of stderr.
func gitOutput(ctx context.Context, exec runner, dir string, args ...string) (string, error) {
	result, err := exec.Run(ctx, executor.Command{Name: "git", Args: args, Dir: dir})
	if err != nil {
		return "", ez.Wrap(err)
	}

	if result.ExitCode != 0 {
		stderr := strings.TrimSpace(string(result.Stderr))
		if len(stderr) > maxStderrBytes {
			stderr = "..." + stderr[len(stderr)-maxStderrBytes:]
		}
		return "", ez.New(ez.EINTERNAL, "git "+strings.Join(args, " ")+" exited "+strconv.Itoa(result.ExitCode)+": "+stderr, nil)
	}

	return string(result.Stdout), nil
}

// parseDirty is true when git status --porcelain printed any line.
func parseDirty(out string) bool {
	return strings.TrimSpace(out) != ""
}

// parseCount reads git rev-list --count.
func parseCount(out string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, ez.New(ez.EINTERNAL, "git rev-list --count printed "+strconv.Quote(out), err)
	}

	if count < 0 {
		return 0, ez.New(ez.EINTERNAL, "git rev-list --count printed "+strconv.Quote(out), nil)
	}

	return count, nil
}
