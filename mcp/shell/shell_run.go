package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

	// Add apply_patch function to the shell environment for codex
	const applyPatchSnippet = `
    apply_patch() {
        if [ "$#" -eq 1 ]
        then
            codex --codex-run-as-apply-patch "$1"
        else
            p="$(cat)"
            codex --codex-run-as-apply-patch "$p"
        fi
    }
    apply-patch() {
        apply_patch "$@"
    }
    applypatch()  {
        apply_patch "$@"
    }
    `

	// Build bash with minimal profile loading and sane pipe behavior.
	wrapped := "set -e\n" + applyPatchSnippet + "\n" + command

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

	var cmdErr error
	select {
	case <-ctx.Done():
		// Kill the entire group (negative pgid). Try TERM, then KILL.
		if cmd.Process != nil {
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
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
		cmdErr = ctxErr

	case cmdErr = <-done:
		// exited normally or with failure
	}

	out.Stdout = stdoutBuf.String()
	out.Stderr = stderrBuf.String()

	if out.TimedOut {
		out.ExitCode = -1
		return out, context.DeadlineExceeded
	}

	if cmdErr != nil {
		var ee *exec.ExitError
		if errors.As(cmdErr, &ee) {
			out.ExitCode = ee.ExitCode()

			msg := strings.TrimSpace(out.Stderr)
			if msg == "" {
				msg = strings.TrimSpace(out.Stdout)
			}

			return out, fmt.Errorf("%s (exit code %d)", msg, out.ExitCode)
		}

		// Some other error (Start already returned above, should be rare)
		out.ExitCode = -1
		return out, cmdErr
	}

	out.ExitCode = 0
	return out, nil
}
