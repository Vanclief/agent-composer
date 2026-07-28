package server

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/filesystem"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	workflowconversations "github.com/vanclief/agent-composer/core/resources/workflow/conversations"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	workflownodeexecutions "github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	workflowworktrees "github.com/vanclief/agent-composer/core/resources/workflow/worktrees"
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
}

func New(rootCtx context.Context, ctrl *controller.Controller, hooksAPI *hooks.API, workflowAPI *workflowapi.API) *Server {
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
	}
}

func (s *Server) HandleRequest(request requests.Request) (interface{}, error) {
	const op = "rest.Server.HandleRequest"

	var requester *user.User

	defer func() { logRequest(request, requester) }()

	err := request.GetBody().Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	response, err := s.handleRequest(request)
	if err != nil {
		return response, ez.Wrap(op, err)
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
	case *workflowworktrees.ListRequest:
		return s.WorkflowAPI.Worktrees.List(request.GetContext(), nil, body)
	case *workflowworktrees.CreateRequest:
		return s.WorkflowAPI.Worktrees.Create(request.GetContext(), nil, body)
	case *workflowworktrees.DeleteRequest:
		return s.WorkflowAPI.Worktrees.Delete(request.GetContext(), nil, body)

	default:
		return nil, ez.New("rest.Server.handleRequest", ez.EINVALID, "Unsupported request type", nil)
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
