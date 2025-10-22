package orchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/compose/components/scheduler"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
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

func (o *Orchestrator) CreateSessions(ctx context.Context, agentSpecID uuid.UUID, parallelSessions int) ([]*AgentSessionInstance, error) {
	const op = "orchestra.CreateSessions"

	instances := make([]*AgentSessionInstance, 0, parallelSessions)

	for i := 0; i < parallelSessions; i++ {
		session, err := NewAgentSessionInstance(ctx, o.db, agentSpecID)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		instances = append(instances, session)
	}

	return instances, nil
}

func (o *Orchestrator) RunSessions(sessions []*AgentSessionInstance, prompt string) error {
	const op = "orchestra.RunSessions"

	for i := range sessions {
		sessionID := fmt.Sprintf("agent:%s:%d:%d", sessions[i].ID, time.Now().UnixNano(), i)
		o.scheduler.RunOnce(o.rootCtx, sessionID, func(jobCtx context.Context) {
			_, err := sessions[i].Run(jobCtx, o, prompt)
			if err != nil {
				log.Error().Err(err).Str("agent_session_id", sessions[i].ID.String()).Msg("agent session failed")
			}
		})

	}

	return nil
}
