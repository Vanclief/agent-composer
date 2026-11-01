package tasks

import (
	"context"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor/sandbox"
)

// SandboxRuntime is the Runtime backed by microsandbox.
type SandboxRuntime struct{}

var _ Runtime = SandboxRuntime{}

func (SandboxRuntime) Create(ctx context.Context, spec sandbox.Spec) (Sandbox, error) {
	sb, err := sandbox.Create(ctx, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sb, nil
}

func (SandboxRuntime) Connect(ctx context.Context, name string) (Sandbox, error) {
	sb, err := sandbox.Connect(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return sb, nil
}

func (SandboxRuntime) Start(ctx context.Context, name string, secrets []sandbox.Secret) error {
	return sandbox.Start(ctx, name, secrets)
}

func (SandboxRuntime) Stop(ctx context.Context, name string) error {
	return sandbox.Stop(ctx, name)
}

func (SandboxRuntime) Remove(ctx context.Context, name string) error {
	return sandbox.Remove(ctx, name)
}

func (SandboxRuntime) Status(ctx context.Context, name string) (string, error) {
	return sandbox.Status(ctx, name)
}
