package sandboxes

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

var _ executor.JobWatcher = (*sandbox.Executor)(nil)

// tailChunks is how many output chunks AgentJob reads to find the last
// terminal line.
const tailChunks = 32

// AgentJob is the state of a sandbox's agent job (ADR 0009 item 2: the
// job started from the sandbox's command), read from the runtime.
type AgentJob struct {
	ID      string
	Running bool
	// ExitCode is nil while it runs or when the runtime does not say.
	ExitCode   *int
	StartedAt  time.Time
	FinishedAt *time.Time
	// LastOutputAt is when it last wrote to its terminal, zero if never.
	LastOutputAt time.Time
	// LastLine is the last non-blank line it wrote, escapes stripped.
	LastLine string
}

// AgentJob reads the agent job of a booted, running sandbox: the newest
// job whose command is the sandbox's command. It returns nil when the
// sandbox has no command or the runtime has no such job.
func (api *API) AgentJob(ctx context.Context, record *taskmodels.Sandbox) (*AgentJob, error) {
	fields := strings.Fields(record.Command)
	if len(fields) == 0 {
		return nil, nil
	}

	sb, err := api.rt.Connect(ctx, record.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer func() {
		err := sb.Detach(context.Background())
		if err != nil {
			log.Warn().Err(err).Str("sandbox", record.Name).Msg("cannot release sandbox handle")
		}
	}()

	watcher, ok := sb.(executor.JobWatcher)
	if !ok {
		return nil, ez.New(ez.EUNAVAILABLE, "the runtime of sandbox "+record.Name+" keeps no job log", nil)
	}

	jobs, err := watcher.ListJobs(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	job := newestWithCommand(jobs, fields)
	if job == nil {
		return nil, nil
	}

	out, err := watcher.TailJob(ctx, job.ID, tailChunks)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &AgentJob{
		ID:           job.ID,
		Running:      job.Running,
		ExitCode:     job.ExitCode,
		StartedAt:    job.CreatedAt,
		FinishedAt:   job.FinishedAt,
		LastOutputAt: out.At,
		LastLine:     LastLine(out.Data),
	}, nil
}

// newestWithCommand returns the newest job running command, nil if none.
func newestWithCommand(jobs []executor.Job, command []string) *executor.Job {
	var found *executor.Job
	for i := range jobs {
		if !slices.Equal(jobs[i].Command, command) {
			continue
		}
		if found == nil || jobs[i].CreatedAt.After(found.CreatedAt) {
			found = &jobs[i]
		}
	}

	return found
}

// Running reports whether the runtime says sandbox name is running.
func (api *API) Running(ctx context.Context, name string) (bool, error) {
	status, err := api.rt.Status(ctx, name)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return status == StatusRunning, nil
}

// Status is the runtime's status word for sandbox name, verbatim.
func (api *API) Status(ctx context.Context, name string) (string, error) {
	status, err := api.rt.Status(ctx, name)
	if err != nil {
		return "", ez.Wrap(err)
	}

	return status, nil
}
