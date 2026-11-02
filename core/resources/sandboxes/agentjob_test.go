package sandboxes

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/executor"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// jobRuntime is the fake runtime whose sandboxes keep a job log.
type jobRuntime struct {
	*fakeRuntime
	jobs []executor.Job
	out  map[string]*executor.JobOutput
}

func (r *jobRuntime) Connect(ctx context.Context, name string) (Sandbox, error) {
	return &jobSandbox{fakeSandbox: fakeSandbox{rt: r.fakeRuntime}, rt: r}, nil
}

type jobSandbox struct {
	fakeSandbox
	rt *jobRuntime
}

func (s *jobSandbox) ListJobs(ctx context.Context) ([]executor.Job, error) {
	return s.rt.jobs, nil
}

func (s *jobSandbox) TailJob(ctx context.Context, id string, chunks int) (*executor.JobOutput, error) {
	return s.rt.out[id], nil
}

func TestAgentJob(t *testing.T) {
	e := newTestEnv(t)
	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	code := 1
	rt := &jobRuntime{
		fakeRuntime: e.rt,
		jobs: []executor.Job{
			{ID: "shell", Command: []string{"bash"}, Running: true, CreatedAt: at.Add(time.Hour)},
			{ID: "agent", Command: []string{"agc", "agent", "run", "--task", "x"}, ExitCode: &code, CreatedAt: at},
		},
		out: map[string]*executor.JobOutput{
			"agent": {At: at.Add(time.Minute), Data: []byte("starting\r\nerror: unknown flag\r\n")},
		},
	}
	e.api.rt = rt

	record := &taskmodels.Sandbox{ID: uuid.New(), Name: "x", Command: "agc agent run --task x"}
	job, err := e.api.AgentJob(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.ID != "agent" || job.Running || job.ExitCode == nil || *job.ExitCode != 1 {
		t.Fatalf("AgentJob = %+v, want the exited agent job", job)
	}
	if !job.LastOutputAt.Equal(at.Add(time.Minute)) || job.LastLine != "error: unknown flag" {
		t.Fatalf("AgentJob output = %v %q", job.LastOutputAt, job.LastLine)
	}

	record.Command = ""
	job, err = e.api.AgentJob(context.Background(), record)
	if err != nil || job != nil {
		t.Fatalf("AgentJob without a command = %+v, %v; want nil", job, err)
	}
}
