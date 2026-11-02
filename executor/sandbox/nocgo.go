//go:build !cgo

package sandbox

import (
	"context"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// Executor is unavailable in builds without cgo.
type Executor struct{}

func unavailable() error {
	return ez.New(ez.EUNAVAILABLE, "sandbox executor is not in this build: agc was compiled without cgo", nil)
}

func Create(ctx context.Context, spec Spec) (*Executor, error) {
	return nil, unavailable()
}

func Connect(ctx context.Context, name string) (*Executor, error) {
	return nil, unavailable()
}

func Stop(ctx context.Context, name string) error {
	return unavailable()
}

func Start(ctx context.Context, name string, secrets []Secret) error {
	return unavailable()
}

func Remove(ctx context.Context, name string) error {
	return unavailable()
}

func Status(ctx context.Context, name string) (string, error) {
	return "", unavailable()
}

func List(ctx context.Context, labelKey string) ([]Info, error) {
	return nil, unavailable()
}

func (e *Executor) Detach(ctx context.Context) error {
	return unavailable()
}

func (e *Executor) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	return nil, unavailable()
}

func (e *Executor) LookPath(ctx context.Context, name string) error {
	return unavailable()
}

func (e *Executor) WriteFile(ctx context.Context, path string, data []byte) error {
	return unavailable()
}

func (e *Executor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, unavailable()
}

func (e *Executor) RemoveFile(ctx context.Context, path string) error {
	return unavailable()
}

func (e *Executor) TempDir() string {
	return "/tmp"
}

func (e *Executor) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	return nil, unavailable()
}

func (e *Executor) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	return nil, unavailable()
}

func (e *Executor) AttachTerminal(ctx context.Context, id string, replayBytes int) (executor.TerminalStream, error) {
	return nil, unavailable()
}

func (e *Executor) ListJobs(ctx context.Context) ([]executor.Job, error) {
	return nil, unavailable()
}

func (e *Executor) TailJob(ctx context.Context, id string, chunks int) (*executor.JobOutput, error) {
	return nil, unavailable()
}
