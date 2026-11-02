package server

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/filesystem"
	"github.com/vanclief/agent-composer/core/resources/harnessinfo"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	knowledgeapi "github.com/vanclief/agent-composer/core/resources/knowledge"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/schedules"
	settingsapi "github.com/vanclief/agent-composer/core/resources/settings"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	workflowconversations "github.com/vanclief/agent-composer/core/resources/workflow/conversations"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	workflownodeexecutions "github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	workflowworktrees "github.com/vanclief/agent-composer/core/resources/workflow/worktrees"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/user"
	"github.com/vanclief/compose/components/ratelimit"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

type Server struct {
	RootContext   context.Context
	Ctrl          *controller.Controller
	RateLimiter   *ratelimit.WindowCounter
	HooksAPI      *hooks.API
	WorkflowAPI   *workflowapi.API
	FilesystemAPI *filesystem.API
	HarnessesAPI  *harnessinfo.API
	SettingsAPI   *settingsapi.API
	TasksAPI      *tasks.API
	SandboxesAPI  *sandboxes.API
	SchedulesAPI  *schedules.API
	KnowledgeAPI  *knowledgeapi.API
	// TurnsAPI reads agent and sandbox turns from their volumes.
	TurnsAPI  *agents.TurnsAPI
	Messaging *messaging.Service
	// Events feeds GET /api/events.
	Events *events.Bus
}

func New(rootCtx context.Context, ctrl *controller.Controller, hooksAPI *hooks.API, workflowAPI *workflowapi.API, tasksAPI *tasks.API, schedulesAPI *schedules.API, knowledgeAPI *knowledgeapi.API, msg *messaging.Service, bus *events.Bus) *Server {
	if rootCtx == nil {
		rootCtx = context.Background()
	}

	limiter := ratelimit.NewWindowCounter(ctrl.Config.App.RateLimitWindow, ctrl.Config.App.RateLimit)

	return &Server{
		RootContext:   rootCtx,
		Ctrl:          ctrl,
		RateLimiter:   limiter,
		HooksAPI:      hooksAPI,
		WorkflowAPI:   workflowAPI,
		FilesystemAPI: filesystem.NewAPI(),
		HarnessesAPI:  harnessinfo.NewAPI(),
		SettingsAPI:   settingsapi.NewAPI(),
		TasksAPI:      tasksAPI,
		SandboxesAPI:  tasksAPI.Sandboxes,
		SchedulesAPI:  schedulesAPI,
		KnowledgeAPI:  knowledgeAPI,
		TurnsAPI:      agents.NewTurnsAPI(tasksAPI.Sandboxes.VolumeRootDir(), tasksAPI.Sandboxes),
		Messaging:     msg,
		Events:        bus,
	}
}

func (s *Server) HandleRequest(request requests.Request) (interface{}, error) {
	var requester *user.User

	defer func() { logRequest(request, requester) }()

	err := request.GetBody().Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	response, err := s.handleRequest(request)
	if err != nil {
		return response, ez.Wrap(err)
	} else if response != nil {
		return response, nil
	}

	return nil, nil
}

func (s *Server) handleRequest(request requests.Request) (interface{}, error) {
	switch body := request.GetBody().(type) {
	case *hooks.ListRequest:
		return s.HooksAPI.List(request.GetContext(), nil, body)
	case *hooks.GetRequest:
		return s.HooksAPI.Get(request.GetContext(), nil, body)
	case *hooks.CreateRequest:
		return s.HooksAPI.Create(request.GetContext(), nil, body)
	case *hooks.UpdateRequest:
		return s.HooksAPI.Update(request.GetContext(), nil, body)
	case *hooks.DeleteRequest:
		return s.HooksAPI.Delete(request.GetContext(), nil, body)

	case *workflowapi.ListRequest:
		return s.WorkflowAPI.List(request.GetContext(), nil, body)
	case *workflowapi.GetRequest:
		return s.WorkflowAPI.Get(request.GetContext(), nil, body)
	case *workflowapi.UpdateNodeRequest:
		return s.WorkflowAPI.UpdateNode(request.GetContext(), nil, body)
	case *workflowapi.CreateRequest:
		return s.WorkflowAPI.Create(request.GetContext(), nil, body)
	case *workflowapi.RenameRequest:
		return s.WorkflowAPI.Rename(request.GetContext(), nil, body)
	case *workflowapi.DeleteRequest:
		return s.WorkflowAPI.Delete(request.GetContext(), nil, body)
	case *workflowapi.ComposeRequest:
		// Composer conversations outlive the default request timeout.
		ctx, cancel := context.WithTimeout(
			s.workflowExecutionStartContext(),
			10*time.Minute,
		)
		defer cancel()
		return s.WorkflowAPI.Compose(ctx, nil, body)
	case *workflowapi.SaveDraftRequest:
		return s.WorkflowAPI.SaveDraft(request.GetContext(), nil, body)
	case *workflowapi.DeleteDraftRequest:
		return s.WorkflowAPI.DeleteDraft(request.GetContext(), nil, body)
	case *settingsapi.GetRequest:
		return s.SettingsAPI.Get(request.GetContext(), nil, body)
	case *settingsapi.UpdateRequest:
		return s.SettingsAPI.Update(request.GetContext(), nil, body)
	case *workflowexecutions.CreateRequest:
		return s.WorkflowAPI.Executions.Create(s.workflowExecutionStartContext(), nil, body)
	case *workflowexecutions.ListRequest:
		return s.WorkflowAPI.Executions.List(request.GetContext(), nil, body)
	case *workflowexecutions.GetRequest:
		return s.WorkflowAPI.Executions.Get(request.GetContext(), nil, body)
	case *workflownodeexecutions.ListRequest:
		return s.WorkflowAPI.NodeExecutions.List(request.GetContext(), nil, body)
	case *workflownodeexecutions.GetRequest:
		return s.WorkflowAPI.NodeExecutions.Get(request.GetContext(), nil, body)
	case *workflowconversations.ListRequest:
		return s.WorkflowAPI.Conversations.List(request.GetContext(), nil, body)
	case *filesystem.BrowseRequest:
		return s.FilesystemAPI.Browse(request.GetContext(), nil, body)
	case *harnessinfo.ListRequest:
		return s.HarnessesAPI.List(request.GetContext(), nil, body)
	case *workflowworktrees.ListRequest:
		return s.WorkflowAPI.Worktrees.List(request.GetContext(), nil, body)
	case *workflowworktrees.CreateRequest:
		return s.WorkflowAPI.Worktrees.Create(request.GetContext(), nil, body)
	case *workflowworktrees.DeleteRequest:
		return s.WorkflowAPI.Worktrees.Delete(request.GetContext(), nil, body)

	case *tasks.ListRequest:
		return s.TasksAPI.List(request.GetContext(), nil, body)
	case *tasks.GetRequest:
		return s.TasksAPI.Get(request.GetContext(), nil, body)
	// Creating a task and allocating a sandbox return at once; the boot
	// runs in the background with its own context (ADR 0008).
	case *tasks.CreateRequest:
		return s.TasksAPI.Create(request.GetContext(), nil, body)
	case *tasks.DeliverRequest:
		return s.TasksAPI.Deliver(request.GetContext(), nil, body)
	// Accepting stops the task's sandboxes, which can outlive the default
	// request timeout (ADR 0010 item 4).
	case *tasks.AcceptRequest:
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 5*time.Minute)
		defer cancel()
		return s.TasksAPI.Accept(ctx, nil, body)
	case *tasks.RejectRequest:
		return s.TasksAPI.Reject(request.GetContext(), nil, body)
	case *tasks.DeleteRequest:
		// Every sandbox of the task is stopped and removed.
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 5*time.Minute)
		defer cancel()
		return s.TasksAPI.Delete(ctx, nil, body)

	case *sandboxes.AllocateRequest:
		return s.SandboxesAPI.Allocate(request.GetContext(), nil, body)
	case *sandboxes.ListRequest:
		return s.SandboxesAPI.List(request.GetContext(), nil, body)
	case *sandboxes.GetRequest:
		return s.SandboxesAPI.Get(request.GetContext(), nil, body)
	// Booting and shutting a VM down can outlive the default request
	// timeout, and cutting msb off mid-boot leaves the sandbox half up.
	case *sandboxes.StopRequest:
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 2*time.Minute)
		defer cancel()
		return s.SandboxesAPI.Stop(ctx, nil, body)
	case *sandboxes.StartRequest:
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 2*time.Minute)
		defer cancel()
		return s.SandboxesAPI.Start(ctx, nil, body)
	case *sandboxes.DeleteRequest:
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 2*time.Minute)
		defer cancel()
		return s.SandboxesAPI.Delete(ctx, nil, body)
	case *sandboxes.ListTerminalsRequest:
		return s.SandboxesAPI.ListTerminals(request.GetContext(), nil, body)
	case *sandboxes.StartTerminalRequest:
		return s.SandboxesAPI.StartTerminal(request.GetContext(), nil, body)
	case *sandboxes.ListTranscriptsRequest:
		return s.SandboxesAPI.ListTranscripts(request.GetContext(), nil, body)
	case *agents.ListSandboxTurnsRequest:
		return s.TurnsAPI.ListSandboxTurns(request.GetContext(), nil, body)
	case *agents.GetSandboxTurnRequest:
		return s.TurnsAPI.GetSandboxTurn(request.GetContext(), nil, body)
	case *agents.ListTurnsRequest:
		return s.TurnsAPI.ListTurns(request.GetContext(), nil, body)
	case *agents.GetTurnRequest:
		return s.TurnsAPI.GetTurn(request.GetContext(), nil, body)
	case *tasks.ListSpecsRequest:
		return s.TasksAPI.ListSpecs(request.GetContext(), nil, body)
	case *tasks.GetSpecRequest:
		return s.TasksAPI.GetSpec(request.GetContext(), nil, body)
	case *tasks.SaveSpecRequest:
		return s.TasksAPI.SaveSpec(request.GetContext(), nil, body)
	case *tasks.DeleteSpecRequest:
		return s.TasksAPI.DeleteSpec(request.GetContext(), nil, body)
	case *tasks.ListSecretsRequest:
		return s.TasksAPI.ListSecrets(request.GetContext(), nil, body)

	case *schedules.ListRequest:
		return s.SchedulesAPI.List(request.GetContext(), nil, body)
	case *schedules.GetRequest:
		return s.SchedulesAPI.Get(request.GetContext(), nil, body)
	case *schedules.CreateRequest:
		return s.SchedulesAPI.Create(request.GetContext(), nil, body)
	case *schedules.UpdateRequest:
		return s.SchedulesAPI.Update(request.GetContext(), nil, body)
	case *schedules.DeleteRequest:
		return s.SchedulesAPI.Delete(request.GetContext(), nil, body)
	case *schedules.ListItemsRequest:
		return s.SchedulesAPI.ListItems(request.GetContext(), nil, body)
	// A tick runs the poll command (up to 60s) and creates its tasks.
	case *schedules.RunRequest:
		ctx, cancel := context.WithTimeout(s.workflowExecutionStartContext(), 5*time.Minute)
		defer cancel()
		return s.SchedulesAPI.Run(ctx, nil, body)

	// The live stream lasts as long as the client stays: no timeout, the
	// handler passes the connection's own context.
	case *events.SubscribeRequest:
		return s.Events.Subscribe(request.GetContext()), nil

	case *knowledgeapi.ListRequest:
		return s.KnowledgeAPI.List(request.GetContext(), nil, body)
	case *knowledgeapi.WhyRequest:
		return s.KnowledgeAPI.Why(request.GetContext(), nil, body)
	case *knowledgeapi.DoubtRequest:
		return s.KnowledgeAPI.Doubt(request.GetContext(), nil, body)
	case *knowledgeapi.ConflictsRequest:
		return s.KnowledgeAPI.Conflicts(request.GetContext(), nil, body)

	default:
		return nil, ez.New(ez.EINVALID, "Unsupported request type", nil)
	}
}

func (s *Server) workflowExecutionStartContext() context.Context {
	if s.RootContext != nil {
		return s.RootContext
	}

	return context.Background()
}

func logRequest(request requests.Request, requester *user.User) {
	newLog := log.Info().
		Str("id", request.GetID()).
		Type("body_type", request.GetBody()).
		Str("latency", time.Since(request.GetCreatedAt()).String()).
		Str("request_ip", request.GetIP())

	if request.GetClient() != "" {
		newLog.Str("request_client", request.GetClient())
	}

	if requester != nil {
		newLog.Int64("user_id", requester.ID)
	}

	newLog.Msg("Request Handled")
}
