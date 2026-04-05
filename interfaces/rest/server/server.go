package server

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	workflownodeexecutions "github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	"github.com/vanclief/agent-composer/models/user"
	"github.com/vanclief/compose/components/ratelimit"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

type Server struct {
	Ctrl        *controller.Controller
	RateLimiter *ratelimit.WindowCounter
	HooksAPI    *hooks.API
	WorkflowAPI *workflowapi.API
}

func New(ctrl *controller.Controller, hooksAPI *hooks.API, workflowAPI *workflowapi.API) *Server {
	limiter := ratelimit.NewWindowCounter(ctrl.Config.App.RateLimitWindow, ctrl.Config.App.RateLimit)

	return &Server{
		Ctrl:        ctrl,
		RateLimiter: limiter,
		HooksAPI:    hooksAPI,
		WorkflowAPI: workflowAPI,
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

	case *workflowexecutions.CreateRequest:
		return s.WorkflowAPI.Executions.Create(request.GetContext(), nil, body)
	case *workflowexecutions.ListRequest:
		return s.WorkflowAPI.Executions.List(request.GetContext(), nil, body)
	case *workflowexecutions.GetRequest:
		return s.WorkflowAPI.Executions.Get(request.GetContext(), nil, body)
	case *workflownodeexecutions.ListRequest:
		return s.WorkflowAPI.NodeExecutions.List(request.GetContext(), nil, body)
	case *workflownodeexecutions.GetRequest:
		return s.WorkflowAPI.NodeExecutions.Get(request.GetContext(), nil, body)

	default:
		return nil, ez.New("rest.Server.handleRequest", ez.EINVALID, "Unsupported request type", nil)
	}
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
