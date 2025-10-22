package orchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/compose/components/scheduler"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/server/controller"
)

type Orchestrator struct {
	rootCtx   context.Context
	db        *relational.DB
	scheduler *scheduler.Scheduler
}

type hookSub struct {
	cancel      context.CancelFunc
	unsubscribe func() error
}

func NewOrchestrator(rootCtx context.Context, ctrl *controller.Controller, sch *scheduler.Scheduler) (*Orchestrator, error) {
	const op = "orchestra.NewOrchestrator"

	if ctrl == nil {
		return nil, ez.Root(op, ez.EINTERNAL, "Controller reference is nil")
	}

	o := &Orchestrator{
		rootCtx:   rootCtx,
		db:        ctrl.DB,
		scheduler: sch,
	}

	return o, nil
}

func (o *Orchestrator) CreateRuns(ctx context.Context, templateID uuid.UUID, parrallelRuns int) ([]*ParrotRunInstance, error) {
	const op = "orchestra.CreateRuns"

	instances := make([]*ParrotRunInstance, 0, parrallelRuns)

	for i := 0; i < parrallelRuns; i++ {
		p, err := NewParrotRunInstance(ctx, o.db, templateID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		instances = append(instances, p)
	}

	return instances, nil
}

func (o *Orchestrator) RunInstances(runs []*ParrotRunInstance, prompt string) error {
	const op = "orchestra.RunInstances"

	for i := range runs {
		runID := fmt.Sprintf("parrot:%s:%d:%d", runs[i].ID, time.Now().UnixNano(), i)
		o.scheduler.RunOnce(o.rootCtx, runID, func(jobCtx context.Context) {
			_, err := runs[i].Run(jobCtx, o, prompt)
			if err != nil {
				log.Error().Err(err).Str("parrot_run_id", runs[i].ID.String()).Msg("parrot run failed")
			}
		})

	}

	return nil
}
