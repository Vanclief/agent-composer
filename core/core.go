package core

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/compose/components/logger"
	"github.com/vanclief/compose/components/scheduler"
)

const tickTime = 1 * time.Minute

// Stack represents the core services required by any interface.
type Stack struct {
	Controller *controller.Controller
	Scheduler  *scheduler.Scheduler
	Runtime    *runtime.Runtime
	AgentsAPI  *agents.API
	HooksAPI   *hooks.API
}

type StackOptions struct {
	ShellRoot string
}

// New builds the application stack (controller, scheduler, runtime, APIs).
func NewStack(rootCtx context.Context, opts StackOptions) (*Stack, error) {
	ctrl, err := controller.New()
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

	agentsAPI := agents.NewAPI(ctrl, rt)
	hooksAPI := hooks.NewAPI(ctrl, rt)

	return &Stack{
		Controller: ctrl,
		Scheduler:  sch,
		Runtime:    rt,
		AgentsAPI:  agentsAPI,
		HooksAPI:   hooksAPI,
	}, nil
}

// StartScheduler blocks while the scheduler runs until the context is canceled.
func (s *Stack) StartScheduler(ctx context.Context) {
	s.Scheduler.Start(ctx)
}
