//go:build cgo

package sandbox

import (
	"context"

	m "github.com/superradcompany/microsandbox/sdk/go"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
)

// ListJobs lists every job of the sandbox, finished ones and those of
// earlier boots included, with their exit codes.
func (e *Executor) ListJobs(ctx context.Context) ([]executor.Job, error) {
	jobs := []executor.Job{}
	listOpts := []m.JobListOption{m.WithAllJobs()}
	for {
		page, err := e.sb.ListJobs(ctx, listOpts...)
		if err != nil {
			return nil, ez.New(ez.EINTERNAL, "cannot list jobs", err)
		}
		for _, info := range page.Items {
			jobs = append(jobs, jobFromInfo(info))
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return jobs, nil
		}
		listOpts = []m.JobListOption{m.WithAllJobs(), m.WithJobCursor(*page.NextCursor)}
	}
}

func jobFromInfo(info m.JobInfo) executor.Job {
	job := executor.Job{
		ID:        info.ID,
		Command:   info.Command,
		Running:   info.State == "running",
		ExitCode:  info.ExitCode,
		CreatedAt: UnixTime(info.CreatedAt),
	}
	if info.FinishedAt != nil {
		finished := UnixTime(*info.FinishedAt)
		job.FinishedAt = &finished
	}

	return job
}

// TailJob reads the last chunks of job id's retained output from the
// runtime's job log, which msb keeps with a timestamp per chunk.
func (e *Executor) TailJob(ctx context.Context, id string, chunks int) (*executor.JobOutput, error) {
	job, err := e.sb.GetJob(ctx, id)
	if err != nil {
		return nil, ez.New(ez.ENOTFOUND, "cannot find job "+id, err)
	}
	defer job.Close()

	tail := uint(chunks)
	entries, err := job.Logs(ctx, m.JobLogOptions{Tail: &tail})
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot read the log of job "+id, err)
	}

	out := &executor.JobOutput{}
	for _, entry := range entries {
		out.Data = append(out.Data, entry.Data...)
		at := UnixTime(entry.Timestamp)
		if at.After(out.At) {
			out.At = at
		}
	}

	return out, nil
}
