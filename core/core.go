package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	"github.com/vanclief/agent-composer/core/resources/knowledge"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/schedules"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/agent-composer/core/resources/triggers"
	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/runtime"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
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
	Messaging   *messaging.Service
	TasksAPI    *tasks.API
	// SchedulesAPI creates tasks on a schedule; only agc rest starts its
	// scheduler (ADR 0012).
	SchedulesAPI *schedules.API
	KnowledgeAPI *knowledge.API
	// AgentsAPI is persistent agents (ADR 0016).
	AgentsAPI *agents.API
	// Events is the live feed behind GET /api/events.
	Events *events.Bus
	// TriggersAPI posts outside events as messages (ADR 0014).
	TriggersAPI *triggers.API
}

type StackOptions struct {
	ProjectDir string
	LogWriter  io.Writer
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

	rt, err := runtime.New(rootCtx, ctrl, sch, runtime.Options{ProjectDir: opts.ProjectDir})
	if err != nil {
		return nil, err
	}

	hooksAPI := hooks.NewAPI(ctrl, rt)
	workflowAPI := workflowapi.NewAPI(ctrl, rt)

	// Best-effort: past runs are linked to each installed workflow's
	// permanent id. Failure must not stop the app.
	err = workflowAPI.BackfillWorkflowIDs(rootCtx)
	if err != nil {
		log.Warn().Err(err).Msg("workflow uuid backfill failed")
	}

	bus := events.New()
	msg := messaging.NewService(ctrl.DB)
	msg.SetEvents(bus)

	secretsPath, err := secrets.DefaultPath()
	if err != nil {
		return nil, err
	}

	home, err := workflowruntime.ResolveHomeDir()
	if err != nil {
		return nil, err
	}

	specs := task.Store{Dir: filepath.Join(home, "config", "specs")}
	sandboxesAPI := sandboxes.NewAPI(rootCtx, ctrl, msg, sandboxes.SandboxRuntime{}, secretsPath, filepath.Join(home, "tasks"))
	tasksAPI := tasks.NewAPI(ctrl, sandboxesAPI, msg, secretsPath, specs)
	tasksAPI.SetEvents(bus)
	schedulesAPI := schedules.NewAPI(ctrl, tasksAPI, secretsPath)
	schedulesAPI.SetEvents(bus)

	// Participant presence reads sandbox status through this seam, so
	// messaging does not import the sandboxes API.
	msg.SetPresence(sandboxesAPI)

	// Sandboxes keep running while agc is down. Their messaging listeners
	// come back here.
	err = sandboxesAPI.Resume(rootCtx)
	if err != nil {
		log.Warn().Err(err).Msg("sandbox messaging resume failed")
	}

	// The knowledge directory holds evidence/, beliefs/, decisions/ and
	// rules/ (ADR 0006).
	knowledgeDir := os.Getenv("AGC_KNOWLEDGE_DIR")
	if knowledgeDir == "" {
		knowledgeDir = filepath.Join(home, "knowledge")
	}
	knowledgeAPI := knowledge.NewAPI(knowledgeDir)

	// The agents directory holds the role templates, <role>.yaml
	// (ADR 0016). The API wakes agents on posts, answers messaging's main
	// check and serves the agent routes on homes' sockets.
	agentsDir := os.Getenv("AGC_AGENTS_DIR")
	if agentsDir == "" {
		agentsDir = filepath.Join(home, "agents")
	}
	agentsAPI := agents.NewAPI(ctrl, tasksAPI, msg, agentsDir)
	agentsAPI.SetEvents(bus)

	return &Stack{
		Controller:   ctrl,
		Scheduler:    sch,
		Runtime:      rt,
		HooksAPI:     hooksAPI,
		WorkflowAPI:  workflowAPI,
		Messaging:    msg,
		TasksAPI:     tasksAPI,
		SchedulesAPI: schedulesAPI,
		KnowledgeAPI: knowledgeAPI,
		AgentsAPI:    agentsAPI,
		Events:       bus,
		TriggersAPI:  triggers.NewAPI(ctrl, msg, secretsPath, home),
	}, nil
}

// StartScheduler blocks while the scheduler runs until the context is canceled.
// Only agc rest calls it, so it also starts the trigger polling (ADR 0014).
func (s *Stack) StartScheduler(ctx context.Context) {
	go s.TriggersAPI.Start(ctx)
	s.Scheduler.Start(ctx)
}
