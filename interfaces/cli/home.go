package cli

import (
	"context"
	"os"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// insideHome reports whether this agc runs in a persistent agent's home
// (ADR 0016): the home sets AGC_AGENT and no AGC_TASK, and messaging
// reaches the host over vsock (or AGC_MSG_URL, for tests).
func insideHome() bool {
	if os.Getenv(sandboxes.EnvAgent) == "" || os.Getenv(task.EnvTask) != "" {
		return false
	}

	if os.Getenv("AGC_MSG_URL") != "" {
		return true
	}

	_, err := os.Stat("/dev/vsock")
	return err == nil
}

// homeService is taskService inside an agent's home, which cannot reach
// agc rest: agc task create, get, accept and reject go over the home's
// messaging socket, as the agent (it manages what it creates and reads,
// accepts and rejects only what it manages). Everything else says it is
// not served here.
type homeService struct{}

func notInHome(what string) error {
	return ez.New(ez.EINVALID, "inside an agent's home only agc task create, get, accept and reject work ("+what+" does not): the home reaches agc over its messaging socket only", nil)
}

func (homeService) Defaults() (controller.DefaultsSettings, bool) {
	return controller.DefaultsSettings{}, false
}

func (homeService) CreateTask(ctx context.Context, request *tasks.CreateRequest) (*tasks.TaskView, error) {
	out := &tasks.TaskView{}

	err := newMsgClient().CreateTask(ctx, request, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

func (homeService) AcceptTask(ctx context.Context, ref string, by string) (*tasks.TaskView, error) {
	if by != "" {
		return nil, ez.New(ez.EINVALID, "inside an agent's home the agent accepts, so --by is not taken", nil)
	}

	out := &tasks.TaskView{}

	err := newMsgClient().AcceptTask(ctx, ref, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

func (homeService) ListTasks(ctx context.Context) (*tasks.ListResponse, error) {
	return nil, notInHome("task list")
}

func (homeService) GetTask(ctx context.Context, ref string) (*tasks.TaskView, error) {
	out := &tasks.TaskView{}

	err := newMsgClient().GetTask(ctx, ref, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

func (homeService) DeliverTask(ctx context.Context, ref string, summary string, report *taskmodels.Report) (*tasks.TaskView, error) {
	return nil, notInHome("task deliver")
}

func (homeService) RejectTask(ctx context.Context, ref string, by string, why string) (*tasks.TaskView, error) {
	if by != "" {
		return nil, ez.New(ez.EINVALID, "inside an agent's home the agent rejects, so --by is not taken", nil)
	}

	out := &tasks.TaskView{}

	err := newMsgClient().RejectTask(ctx, ref, why, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

func (homeService) DeleteTask(ctx context.Context, ref string) (*tasks.DeleteResponse, error) {
	return nil, notInHome("task delete")
}

func (homeService) Allocate(ctx context.Context, taskRef string, sandbox task.Sandbox) (*sandboxes.SandboxView, error) {
	return nil, notInHome("allocating a sandbox")
}

func (homeService) ListSandboxes(ctx context.Context) (*sandboxes.ListResponse, error) {
	return nil, notInHome("sandbox list")
}

func (homeService) GetSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return nil, notInHome("sandbox get")
}

func (homeService) StopSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return nil, notInHome("sandbox stop")
}

func (homeService) StartSandbox(ctx context.Context, ref string) (*sandboxes.SandboxView, error) {
	return nil, notInHome("sandbox start")
}

func (homeService) DeleteSandbox(ctx context.Context, ref string) (*sandboxes.DeleteResponse, error) {
	return nil, notInHome("sandbox delete")
}

func (homeService) GCSandboxes(ctx context.Context, request *sandboxes.GCRequest) (*sandboxes.GCResponse, error) {
	return nil, notInHome("sandbox gc")
}

func (homeService) ListSpecs(ctx context.Context) (*tasks.ListSpecsResponse, error) {
	return nil, notInHome("task spec ls")
}

func (homeService) GetSpec(ctx context.Context, name string) (*task.Saved, error) {
	return nil, notInHome("--from a saved spec")
}

func (homeService) SaveSpec(ctx context.Context, name string, yaml string) (*task.Saved, error) {
	return nil, notInHome("task spec save")
}

func (homeService) DeleteSpec(ctx context.Context, name string) (*task.Saved, error) {
	return nil, notInHome("task spec rm")
}

func (homeService) WaitSandbox(ctx context.Context, ref string, onStatus func(view *sandboxes.SandboxView)) (*sandboxes.SandboxView, error) {
	return nil, notInHome("--wait")
}
