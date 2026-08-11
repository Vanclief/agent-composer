package runtime

import (
	"context"
	"strings"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
	"github.com/vanclief/compose/components/scheduler"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

type Runtime struct {
	rootCtx   context.Context
	db        *relational.DB
	scheduler *scheduler.Scheduler
	project   string
}

type hookSub struct {
	cancel      context.CancelFunc
	unsubscribe func() error
}

type Options struct {
	ProjectDir string
}

func New(rootCtx context.Context, ctrl *controller.Controller, sch *scheduler.Scheduler, opts Options) (*Runtime, error) {
	if ctrl == nil {
		return nil, ez.Root(ez.EINTERNAL, "Controller reference is nil")
	}

	rt := &Runtime{
		rootCtx:   rootCtx,
		db:        ctrl.DB,
		scheduler: sch,
		project:   strings.TrimSpace(opts.ProjectDir),
	}

	return rt, nil
}

func (rt *Runtime) ValidateHarness(ctx context.Context, kind agent.Harness, model string, config []byte) error {
	harness, err := harnesses.New(kind)
	if err != nil {
		return ez.Wrap(err)
	}

	err = harness.Validate(ctx, model, config)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func (rt *Runtime) ProjectDir() string {
	if rt == nil {
		return ""
	}

	return rt.project
}
