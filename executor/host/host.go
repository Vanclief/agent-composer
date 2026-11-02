// Package host runs programs directly on this machine. It is how agc ran
// harnesses before Tasks and stays for hosts without a VM runtime.
package host

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// Executor runs programs as subprocesses of agc and uses the local filesystem.
type Executor struct{}

func New() *Executor {
	return &Executor{}
}

// Run runs cmd to completion. When it returns an error, the output captured
// so far is returned with it.
func (e *Executor) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	if cmd.User != "" {
		return nil, ez.New(ez.EINVALID, "the host executor runs as agc's own user, not "+cmd.User, nil)
	}

	command := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	command.Dir = cmd.Dir

	// A nil Env inherits the environment and lets os/exec set PWD to Dir, so
	// only replace it when there is something to add.
	if len(cmd.Env) > 0 {
		env := os.Environ()
		for key, value := range cmd.Env {
			env = append(env, key+"="+value)
		}
		command.Env = env
	}

	if cmd.Stdin != nil {
		command.Stdin = bytes.NewReader(cmd.Stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	result := &executor.Result{}

	var exitErr *exec.ExitError
	exited := errors.As(err, &exitErr)
	if exited {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			result.ExitCode = status.ExitStatus()
		}
	}

	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()

	if err == nil {
		return result, nil
	}

	if ctx.Err() != nil {
		return result, ez.New(ez.EUNAVAILABLE, cmd.Name+" was canceled", ctx.Err())
	}

	if !exited {
		return result, ez.New(ez.EINTERNAL, "failed to run "+cmd.Name, err)
	}

	return result, nil
}

func (e *Executor) LookPath(ctx context.Context, name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return ez.New(ez.ENOTFOUND, name+" is not installed", err)
	}

	return nil
}

func (e *Executor) WriteFile(ctx context.Context, path string, data []byte) error {
	err := os.WriteFile(path, data, 0o600)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (e *Executor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return data, nil
}

func (e *Executor) RemoveFile(ctx context.Context, path string) error {
	err := os.Remove(path)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (e *Executor) TempDir() string {
	return os.TempDir()
}
