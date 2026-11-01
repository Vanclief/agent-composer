//go:build cgo

package sandbox

import (
	"context"
	"errors"
	"io"

	m "github.com/superradcompany/microsandbox/sdk/go"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// StartTerminal starts cmd as a detached TTY job that outlives this process.
func (e *Executor) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	env := map[string]string{"TERM": "xterm-256color"}
	for k, v := range cmd.Env {
		env[k] = v
	}
	opts := []m.JobExecOption{m.WithExecTTY(true), m.WithExecEnv(env)}
	if cmd.Dir != "" {
		opts = append(opts, m.WithExecCwd(cmd.Dir))
	}

	job, err := e.sb.ExecDetached(ctx, cmd.Name, cmd.Args, opts...)
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot start terminal "+cmd.Name, err)
	}
	id := job.ID()
	err = job.Close()
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot release job "+id, err)
	}

	return &executor.Terminal{
		ID:      id,
		Command: append([]string{cmd.Name}, cmd.Args...),
		Running: true,
	}, nil
}

// ListTerminals lists the sandbox's jobs as the runtime reports them.
func (e *Executor) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	var terminals []executor.Terminal
	var listOpts []m.JobListOption
	for {
		page, err := e.sb.ListJobs(ctx, listOpts...)
		if err != nil {
			return nil, ez.New(ez.EINTERNAL, "cannot list jobs", err)
		}
		for _, job := range page.Items {
			terminals = append(terminals, executor.Terminal{
				ID:      job.ID,
				Command: job.Command,
				Running: job.State == "running",
			})
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return terminals, nil
		}
		listOpts = []m.JobListOption{m.WithJobCursor(*page.NextCursor)}
	}
}

// AttachTerminal attaches to job id, replaying up to replayBytes of recent
// output first.
func (e *Executor) AttachTerminal(ctx context.Context, id string, replayBytes int) (executor.TerminalStream, error) {
	job, err := e.sb.GetJob(ctx, id)
	if err != nil {
		return nil, ez.New(ez.ENOTFOUND, "cannot find job "+id, err)
	}
	var opts []m.JobAttachOption
	if replayBytes > 0 {
		opts = append(opts, m.WithJobReplayRecent(uint(replayBytes)))
	}
	attachment, err := job.Attach(ctx, opts...)
	if err != nil {
		job.Close()
		return nil, ez.New(ez.EINTERNAL, "cannot attach to job "+id, err)
	}
	return &terminalStream{job: job, attachment: attachment}, nil
}

type terminalStream struct {
	job        *m.Job
	attachment *m.JobAttachment
}

// Recv returns the next chunk of output, skipping gaps in the replay, and
// io.EOF once the program has exited.
func (s *terminalStream) Recv(ctx context.Context) ([]byte, error) {
	for {
		ev, err := s.attachment.Recv(ctx)
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if err != nil {
			return nil, ez.New(ez.EINTERNAL, "cannot read terminal "+s.job.ID(), err)
		}
		switch ev.Type {
		case "output":
			return ev.Output.Data, nil
		case "completed":
			return nil, io.EOF
		}
	}
}

func (s *terminalStream) Write(ctx context.Context, data []byte) error {
	err := s.attachment.WriteStdin(ctx, data)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot write to terminal "+s.job.ID(), err)
	}
	return nil
}

func (s *terminalStream) Resize(ctx context.Context, rows uint16, cols uint16) error {
	err := s.attachment.Resize(ctx, rows, cols)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot resize terminal "+s.job.ID(), err)
	}
	return nil
}

// Close detaches and releases the handles. The program keeps running.
func (s *terminalStream) Close() error {
	defer s.job.Close()
	defer s.attachment.Close()
	err := s.attachment.Detach(context.Background())
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot detach from terminal "+s.job.ID(), err)
	}
	return nil
}
