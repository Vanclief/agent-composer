package server

import (
	"time"

	"github.com/rs/zerolog/log"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/agent-composer/core/resources/agents/conversations"
	"github.com/vanclief/agent-composer/core/resources/agents/specs"
	"github.com/vanclief/agent-composer/core/resources/hooks"
	"github.com/vanclief/agent-composer/models/user"
	"github.com/vanclief/compose/components/ratelimit"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

type Server struct {
	Ctrl        *controller.Controller
	RateLimiter *ratelimit.WindowCounter
	AgentsAPI   *agents.API
	HooksAPI    *hooks.API
}

func New(ctrl *controller.Controller, agentsAPI *agents.API, hooksAPI *hooks.API) *Server {
	limiter := ratelimit.NewWindowCounter(ctrl.Config.App.RateLimitWindow, ctrl.Config.App.RateLimit)

	return &Server{
		Ctrl:        ctrl,
		RateLimiter: limiter,
		AgentsAPI:   agentsAPI,
		HooksAPI:    hooksAPI,
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
	case *specs.ListRequest:
		return s.AgentsAPI.AgentSpecs.List(request.GetContext(), nil, body)
	case *specs.GetRequest:
		return s.AgentsAPI.AgentSpecs.Get(request.GetContext(), nil, body)
	case *specs.CreateRequest:
		return s.AgentsAPI.AgentSpecs.Create(request.GetContext(), nil, body)
	case *specs.UpdateRequest:
		return s.AgentsAPI.AgentSpecs.Update(request.GetContext(), nil, body)
	case *specs.DeleteRequest:
		return s.AgentsAPI.AgentSpecs.Delete(request.GetContext(), nil, body)

	case *conversations.ListRequest:
		return s.AgentsAPI.Conversations.List(request.GetContext(), nil, body)
	case *conversations.GetRequest:
		return s.AgentsAPI.Conversations.Get(request.GetContext(), nil, body)
	case *conversations.CreateRequest:
		return s.AgentsAPI.Conversations.Create(request.GetContext(), nil, body)
	case *conversations.ForkRequest:
		return s.AgentsAPI.Conversations.Fork(request.GetContext(), nil, body)
	case *conversations.ResumeRequest:
		return s.AgentsAPI.Conversations.Resume(request.GetContext(), nil, body)
	case *conversations.DeleteRequest:
		return s.AgentsAPI.Conversations.Delete(request.GetContext(), nil, body)

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
