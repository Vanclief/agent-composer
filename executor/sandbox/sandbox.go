//go:build cgo

package sandbox

import (
	"context"

	m "github.com/superradcompany/microsandbox/sdk/go"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/ez"
)

// Executor runs programs in one running sandbox. It holds a live handle.
// Call Detach when done with it. The sandbox keeps running either way.
type Executor struct {
	sb *m.Sandbox
}

// Create creates and boots a detached sandbox from spec through the msb
// CLI, then connects to it. Secrets reach msb only through its environment,
// so the runtime stores references to them, never their values.
func Create(ctx context.Context, spec Spec) (*Executor, error) {
	err := spec.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}
	err = runMsb(ctx, spec.Secrets, createArgs(spec)...)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	return Connect(ctx, spec.Name)
}

// Connect returns an Executor for a running sandbox. It does not start a
// stopped one, because booting needs the secrets that Start passes to msb.
func Connect(ctx context.Context, name string) (*Executor, error) {
	h, err := m.GetSandbox(ctx, name)
	if err != nil {
		return nil, sdkError(err, "cannot find sandbox "+name)
	}
	if h.Status() != m.SandboxStatusRunning {
		return nil, ez.New(ez.EUNAVAILABLE, "sandbox "+name+" is "+string(h.Status())+", start it first", nil)
	}
	sb, err := h.Connect(ctx)
	if err != nil {
		return nil, sdkError(err, "cannot connect to sandbox "+name)
	}
	return &Executor{sb: sb}, nil
}

// Stop shuts the sandbox down and keeps its disk.
func Stop(ctx context.Context, name string) error {
	h, err := m.GetSandbox(ctx, name)
	if err != nil {
		return sdkError(err, "cannot find sandbox "+name)
	}
	err = h.Stop(ctx)
	if err != nil {
		return sdkError(err, "cannot stop sandbox "+name)
	}
	return nil
}

// Start boots a stopped sandbox, detached, through the msb CLI. Every boot
// needs the values of the secrets the sandbox was created with.
func Start(ctx context.Context, name string, secrets []Secret) error {
	err := runMsb(ctx, secrets, "start", name)
	if err != nil {
		return ez.Wrap(err)
	}
	return nil
}

// Remove deletes a stopped sandbox and its disk. Volumes it mounted remain.
func Remove(ctx context.Context, name string) error {
	err := m.RemoveSandbox(ctx, name)
	if err != nil {
		return sdkError(err, "cannot remove sandbox "+name)
	}
	return nil
}

// Status returns the runtime's status word for the sandbox.
func Status(ctx context.Context, name string) (string, error) {
	h, err := m.GetSandbox(ctx, name)
	if err != nil {
		return "", sdkError(err, "cannot find sandbox "+name)
	}
	return string(h.Status()), nil
}

// List returns the sandboxes carrying the label labelKey, with any value.
// An empty labelKey returns every sandbox.
func List(ctx context.Context, labelKey string) ([]Info, error) {
	var infos []Info
	var listOpts []m.SandboxListOption
	for {
		page, err := m.ListSandboxesWith(ctx, listOpts...)
		if err != nil {
			return nil, sdkError(err, "cannot list sandboxes")
		}
		for _, h := range page.Sandboxes {
			config, err := h.Config()
			if err != nil {
				return nil, ez.New(ez.EINTERNAL, "cannot read config of sandbox "+h.Name(), err)
			}
			_, ok := config.Labels[labelKey]
			if labelKey != "" && !ok {
				continue
			}
			infos = append(infos, Info{Name: h.Name(), Image: config.Image, Status: string(h.Status())})
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			return infos, nil
		}
		listOpts = []m.SandboxListOption{m.WithListCursor(*page.NextCursor)}
	}
}

// Detach releases the handle and leaves the sandbox running.
func (e *Executor) Detach(ctx context.Context) error {
	err := e.sb.Detach(ctx)
	if err != nil {
		return sdkError(err, "cannot release sandbox "+e.sb.Name())
	}
	return nil
}

// Run runs cmd to completion as the sandbox user. Output is streamed rather
// than collected in one call because collected output is capped by the SDK's
// 1 MiB response buffer.
func (e *Executor) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	opts := []m.ExecOption{m.WithExecEnv(cmd.Env)}
	if cmd.Dir != "" {
		opts = append(opts, m.WithExecCwd(cmd.Dir))
	}
	if cmd.Stdin != nil {
		opts = append(opts, m.WithExecStdinPipe())
	}

	h, err := e.sb.ExecStream(ctx, cmd.Name, cmd.Args, opts...)
	if err != nil {
		return nil, sdkError(err, "cannot run "+cmd.Name)
	}
	defer h.Close()

	if cmd.Stdin != nil {
		stdin := h.TakeStdin()
		_, err = stdin.WriteCtx(ctx, cmd.Stdin)
		if err != nil {
			return nil, sdkError(err, "cannot write stdin of "+cmd.Name)
		}
		err = stdin.Close()
		if err != nil {
			return nil, sdkError(err, "cannot close stdin of "+cmd.Name)
		}
	}

	result := &executor.Result{}
	for {
		ev, err := h.Recv(ctx)
		if err != nil {
			return nil, sdkError(err, "cannot read output of "+cmd.Name)
		}
		switch ev.Kind {
		case m.ExecEventStdout:
			result.Stdout = append(result.Stdout, ev.Data...)
		case m.ExecEventStderr:
			result.Stderr = append(result.Stderr, ev.Data...)
		case m.ExecEventExited:
			result.ExitCode = ev.ExitCode
		case m.ExecEventFailed:
			message := "cannot start " + cmd.Name
			if ev.Failure != nil {
				message += ": " + ev.Failure.Message
			}
			return nil, ez.New(ez.EINTERNAL, message, nil)
		case m.ExecEventDone:
			return result, nil
		}
	}
}

// LookPath reports whether name is a command in the sandbox user's PATH.
func (e *Executor) LookPath(ctx context.Context, name string) error {
	res, err := e.Run(ctx, executor.Command{Name: "sh", Args: []string{"-c", `command -v "$1"`, "sh", name}})
	if err != nil {
		return ez.Wrap(err)
	}
	if res.ExitCode != 0 {
		return ez.New(ez.ENOTFOUND, name+" is not in PATH in the sandbox", nil)
	}
	return nil
}

// WriteFile writes data to path as the sandbox user, so the file belongs to
// that user. The SDK filesystem API writes as root.
func (e *Executor) WriteFile(ctx context.Context, path string, data []byte) error {
	res, err := e.Run(ctx, executor.Command{
		Name:  "sh",
		Args:  []string{"-c", `cat > "$1"`, "sh", path},
		Stdin: data,
	})
	if err != nil {
		return ez.Wrap(err)
	}
	if res.ExitCode != 0 {
		return ez.New(ez.EINTERNAL, "cannot write "+path+": "+string(res.Stderr), nil)
	}
	return nil
}

// ReadFile reads path through the SDK filesystem API.
func (e *Executor) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := e.sb.FS().Read(ctx, path)
	if err != nil {
		return nil, sdkError(err, "cannot read "+path)
	}
	return data, nil
}

// RemoveFile removes path through the SDK filesystem API.
func (e *Executor) RemoveFile(ctx context.Context, path string) error {
	err := e.sb.FS().Remove(ctx, path)
	if err != nil {
		return sdkError(err, "cannot remove "+path)
	}
	return nil
}

// TempDir is the guest's /tmp.
func (e *Executor) TempDir() string {
	return "/tmp"
}

func sdkError(err error, message string) error {
	code := ez.EINTERNAL
	if m.IsKind(err, m.ErrSandboxNotFound) || m.IsKind(err, m.ErrPathNotFound) {
		code = ez.ENOTFOUND
	}
	return ez.New(code, message, err)
}
