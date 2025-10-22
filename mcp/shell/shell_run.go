package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// ExecOutcome captures the result of a shell execution.
type ExecOutcome struct {
	ExitCode int
	TimedOut bool
	Stdout   string
	Stderr   string
}

// runBashIsolated starts /bin/bash as a new process group and ensures the entire
// process tree is terminated on timeout/cancel
func runBashIsolated(ctx context.Context, workdir string, command string) (ExecOutcome, error) {
	const bashPath = "/bin/bash"

	var out ExecOutcome
	var stdoutBuf, stderrBuf bytes.Buffer

	// Build bash with minimal profile loading and sane pipe behavior.
	// If you don't want `set -e`, drop it. `-o pipefail` is important.
	wrapped := "set -e; " + command
	cmd := exec.CommandContext(ctx, bashPath, "--noprofile", "--norc", "-o", "pipefail", "-c", wrapped)
	cmd.Dir = workdir
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// New process group so we can signal the whole subtree on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Start()
	if err != nil {
		return out, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var waitErr error
	select {
	case <-ctx.Done():
		// Kill the entire group (negative pgid). Try TERM, then KILL.
		if cmd.Process != nil {
			if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
				_ = syscall.Kill(-pgid, syscall.SIGTERM)

				select {
				case <-time.After(300 * time.Millisecond):
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				case <-done: // exited after TERM
				}
			} else {
				// Fallback: kill just the parent if we couldn't get pgid.
				_ = cmd.Process.Kill()
			}
		}
		ctxErr := ctx.Err()
		out.TimedOut = errors.Is(ctxErr, context.DeadlineExceeded)
		waitErr = ctxErr

	case waitErr = <-done:
		// exited normally or with failure
	}

	out.Stdout = stdoutBuf.String()
	out.Stderr = stderrBuf.String()

	if out.TimedOut {
		out.ExitCode = -1
		return out, context.DeadlineExceeded
	}

	if waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			out.ExitCode = ee.ExitCode()
			return out, fmt.Errorf("command exited with code %d: %w", out.ExitCode, waitErr)
		}
		// Some other error (Start already returned above; this is rare)
		out.ExitCode = -1
		return out, waitErr
	}

	out.ExitCode = 0
	return out, nil
}
