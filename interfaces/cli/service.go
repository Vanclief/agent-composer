package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/interfaces/cli/client"
	"github.com/vanclief/agent-composer/task"
)

// serverEnv names the agc server the task and sandbox commands call
// instead of the local database and runtime.
const serverEnv = "AGC_SERVER"

// taskService is what the task and sandbox commands need, served either
// in-process (localService) or by a remote agc server (client.Client).
type taskService interface {
	CreateTask(ctx context.Context, request *tasks.CreateRequest) (*tasks.TaskView, error)
	ListTasks(ctx context.Context) (*tasks.ListResponse, error)
	GetTask(ctx context.Context, ref string) (*tasks.TaskView, error)
	DoneTask(ctx context.Context, ref string) (*tasks.TaskView, error)
	DeleteTask(ctx context.Context, ref string) (*tasks.DeleteResponse, error)

	Allocate(ctx context.Context, taskRef string, sandbox task.Sandbox) (*sandboxes.SandboxView, error)
	ListSandboxes(ctx context.Context) (*sandboxes.ListResponse, error)
	GetSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error)
	StopSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error)
	StartSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error)
	DeleteSandbox(ctx context.Context, ref string) (*sandboxes.DeleteResponse, error)

	ListSpecs(ctx context.Context) (*tasks.ListSpecsResponse, error)
	GetSpec(ctx context.Context, name string) (*task.Saved, error)
	SaveSpec(ctx context.Context, name string, yaml string) (*task.Saved, error)
	DeleteSpec(ctx context.Context, name string) (*task.Saved, error)

	// WaitSandbox returns once the sandbox is running or failed. Locally
	// the boot runs in this process, so it waits for it to end.
	WaitSandbox(ctx context.Context, ref string, onStatus func(view *sandboxes.SandboxView)) (*sandboxes.SandboxView, error)
}

// remoteFlags are added to every task and sandbox command, so --server and
// --json work after the subcommand as well as before it.
func remoteFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "server", Usage: "agc server to call instead of the local database and runtime (default $" + serverEnv + ")"},
		&cli.BoolFlag{Name: "json", Usage: "Print the raw JSON view"},
	}
}

// withRemoteFlags adds remoteFlags to cmd and all its subcommands.
func withRemoteFlags(cmd *cli.Command) *cli.Command {
	cmd.Flags = append(cmd.Flags, remoteFlags()...)
	for _, sub := range cmd.Subcommands {
		withRemoteFlags(sub)
	}

	return cmd
}

// serverURL is the --server given nearest the command, else AGC_SERVER.
// Empty means work locally.
func serverURL(c *cli.Context) string {
	for _, ctx := range c.Lineage() {
		for _, name := range ctx.LocalFlagNames() {
			if name == "server" {
				return strings.TrimSpace(ctx.String("server"))
			}
		}
	}

	return strings.TrimSpace(os.Getenv(serverEnv))
}

// jsonOutput reports whether --json was given at any level.
func jsonOutput(c *cli.Context) bool {
	for _, ctx := range c.Lineage() {
		for _, name := range ctx.LocalFlagNames() {
			if name == "json" {
				return ctx.Bool("json")
			}
		}
	}

	return false
}

// withService runs fn against the server when one is set, else against an
// in-process stack like workflow run.
func withService(c *cli.Context, fn func(ctx context.Context, svc taskService) error) error {
	server := serverURL(c)
	if server != "" {
		err := fn(c.Context, client.New(server))
		if err != nil {
			fmt.Fprintln(c.App.ErrWriter, ez.ErrorMessage(err))
			return &PrintedError{Err: err}
		}

		return nil
	}

	stack, err := core.NewStack(c.Context, core.StackOptions{ProjectDir: c.String("project")})
	if err != nil {
		return err
	}
	defer stack.Controller.DB.Close() // nolint:errcheck // Close errors are not actionable here.

	return fn(c.Context, localService{api: stack.TasksAPI})
}

// PrintedError is an error whose message the command already printed on
// stderr; the caller exits 1 without logging it again.
type PrintedError struct {
	Err error
}

func (e *PrintedError) Error() string {
	return e.Err.Error()
}

func (e *PrintedError) Unwrap() error {
	return e.Err
}

// localService serves taskService from the in-process tasks API.
type localService struct {
	api *tasks.API
}

func (s localService) CreateTask(ctx context.Context, request *tasks.CreateRequest) (*tasks.TaskView, error) {
	return s.api.Create(ctx, nil, request)
}

func (s localService) ListTasks(ctx context.Context) (*tasks.ListResponse, error) {
	return s.api.List(ctx, nil, &tasks.ListRequest{})
}

func (s localService) GetTask(ctx context.Context, ref string) (*tasks.TaskView, error) {
	return s.api.Get(ctx, nil, &tasks.GetRequest{Name: ref})
}

func (s localService) DoneTask(ctx context.Context, ref string) (*tasks.TaskView, error) {
	return s.api.Done(ctx, nil, &tasks.DoneRequest{Name: ref})
}

func (s localService) DeleteTask(ctx context.Context, ref string) (*tasks.DeleteResponse, error) {
	return s.api.Delete(ctx, nil, &tasks.DeleteRequest{Name: ref})
}

func (s localService) Allocate(ctx context.Context, taskRef string, sandbox task.Sandbox) (*sandboxes.SandboxView, error) {
	return s.api.Sandboxes.Allocate(ctx, nil, &sandboxes.AllocateRequest{TaskName: taskRef, Sandbox: sandbox})
}

func (s localService) ListSandboxes(ctx context.Context) (*sandboxes.ListResponse, error) {
	return s.api.Sandboxes.List(ctx, nil, &sandboxes.ListRequest{})
}

func (s localService) GetSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return s.api.Sandboxes.Get(ctx, nil, &sandboxes.GetRequest{Name: ref})
}

func (s localService) StopSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return s.api.Sandboxes.Stop(ctx, nil, &sandboxes.StopRequest{Name: ref})
}

func (s localService) StartSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return s.api.Sandboxes.Start(ctx, nil, &sandboxes.StartRequest{Name: ref})
}

func (s localService) DeleteSandbox(ctx context.Context, ref string) (*sandboxes.DeleteResponse, error) {
	return s.api.Sandboxes.Delete(ctx, nil, &sandboxes.DeleteRequest{Name: ref})
}

func (s localService) ListSpecs(ctx context.Context) (*tasks.ListSpecsResponse, error) {
	return s.api.ListSpecs(ctx, nil, &tasks.ListSpecsRequest{})
}

func (s localService) GetSpec(ctx context.Context, name string) (*task.Saved, error) {
	return s.api.GetSpec(ctx, nil, &tasks.GetSpecRequest{Name: name})
}

func (s localService) SaveSpec(ctx context.Context, name string, yaml string) (*task.Saved, error) {
	return s.api.SaveSpec(ctx, nil, &tasks.SaveSpecRequest{Name: name, YAML: yaml})
}

func (s localService) DeleteSpec(ctx context.Context, name string) (*task.Saved, error) {
	return s.api.DeleteSpec(ctx, nil, &tasks.DeleteSpecRequest{Name: name})
}

func (s localService) WaitSandbox(ctx context.Context, ref string, onStatus func(view *sandboxes.SandboxView)) (*sandboxes.SandboxView, error) {
	s.api.Sandboxes.Wait()

	view, err := s.GetSandbox(ctx, ref)
	if err != nil {
		return nil, err
	}

	if onStatus != nil {
		onStatus(view)
	}

	return view, nil
}
