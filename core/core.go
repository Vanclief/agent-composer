package core

import (
	"context"
	"io"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/compose/components/logger"
	"github.com/vanclief/compose/components/scheduler"
)

const tickTime = 1 * time.Minute

// Stack represents the core services required by any interface.
type Stack struct {
	Controller  *controller.Controller
	Scheduler   *scheduler.Scheduler
	Runtime     *runtime.Runtime
	HooksAPI    *hooks.API
	WorkflowAPI *workflowapi.API
}

type StackOptions struct {
	ShellRoot string
	LogWriter io.Writer
}

// New builds the application stack (controller, scheduler, runtime, APIs).
func NewStack(rootCtx context.Context, opts StackOptions) (*Stack, error) {
	ctrl, err := controller.NewWithLogWriter(opts.LogWriter)
	if err != nil {
		return nil, err
	}

	var schedOpts []scheduler.Option
	if ctrl.Config.App.Debug {
		l := logger.NewZero(log.Logger)
		schedOpts = append(schedOpts, scheduler.WithLogger(l))
	}

	sch, err := scheduler.New(tickTime, schedOpts...)
	if err != nil {
		return nil, err
	}

	rt, err := runtime.New(rootCtx, ctrl, sch, runtime.Options{ShellRoot: opts.ShellRoot})
	if err != nil {
		return nil, err
	}

	hooksAPI := hooks.NewAPI(ctrl, rt)
	workflowAPI := workflowapi.NewAPI(ctrl, rt)

	// Best-effort: every installed workflow gets its permanent uuid
	// and past runs are linked to it. Failure must not stop the app.
	err = workflowAPI.BackfillWorkflowUUIDs(rootCtx)
	if err != nil {
		log.Warn().Err(err).Msg("workflow uuid backfill failed")
	}

	return &Stack{
		Controller:  ctrl,
		Scheduler:   sch,
		Runtime:     rt,
		HooksAPI:    hooksAPI,
		WorkflowAPI: workflowAPI,
	}, nil
}

// StartScheduler blocks while the scheduler runs until the context is canceled.
func (s *Stack) StartScheduler(ctx context.Context) {
	s.Scheduler.Start(ctx)
}
