// Package executor defines where agc runs programs: on this host, or inside a
// Task VM (docs/decisions/0003-tasks-run-in-microsandbox-vms.md). Harnesses
// and workflow steps call programs only through an Executor so they do not
// know which one they are on.
package executor

import (
	"context"
	"io"
)

// Command is one program invocation.
type Command struct {
	Name string
	Args []string
	// Dir is the working directory. Empty means the executor's default.
	Dir string
	// Env is added to the inherited environment of the program.
	Env map[string]string
	// Stdin is written to the program and then closed. Nil means no input.
	Stdin []byte
}

// Result is the outcome of a Command that ran to completion.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Executor runs programs and moves files in one environment.
//
// Run returns an error only when the program could not be started or the
// context ended. A non-zero exit is reported in Result.ExitCode, with the
// output captured, so callers decide what it means.
type Executor interface {
	Run(ctx context.Context, cmd Command) (*Result, error)
	// LookPath returns nil when name is an executable on PATH in the
	// environment, and an ez.ENOTFOUND error otherwise.
	LookPath(ctx context.Context, name string) error
	WriteFile(ctx context.Context, path string, data []byte) error
	ReadFile(ctx context.Context, path string) ([]byte, error)
	RemoveFile(ctx context.Context, path string) error
	// TempDir is a writable scratch directory in the environment.
	TempDir() string
}

// Terminal describes an interactive program kept running by the environment
// after the caller disconnects.
type Terminal struct {
	ID      string
	Command []string
	Running bool
}

// TerminalStream is one attachment to a Terminal. Recv returns io.EOF when the
// program exits.
type TerminalStream interface {
	Recv(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, data []byte) error
	Resize(ctx context.Context, rows uint16, cols uint16) error
	io.Closer
}

// TerminalExecutor is implemented by executors that can keep interactive
// programs alive between attachments. The host executor does not implement
// it in v1; callers type-assert.
type TerminalExecutor interface {
	Executor
	StartTerminal(ctx context.Context, cmd Command) (*Terminal, error)
	ListTerminals(ctx context.Context) ([]Terminal, error)
	// AttachTerminal replays up to replayBytes of recent output first.
	AttachTerminal(ctx context.Context, id string, replayBytes int) (TerminalStream, error)
}
