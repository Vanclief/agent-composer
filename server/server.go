package server

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/models/user"
	"github.com/vanclief/agent-composer/runtime/orchestra"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/agent-composer/server/resources/agents"
	"github.com/vanclief/agent-composer/server/resources/hooks"
	"github.com/vanclief/compose/components/ratelimit"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

type Server struct {
	Ctrl         *controller.Controller
	RateLimiter  *ratelimit.WindowCounter
	Orchestrator *orchestra.Orchestrator

	// Resources
	AgentsAPI *agents.API
	HooksAPI  *hooks.API
}

func New(ctrl *controller.Controller, orchestrator *orchestra.Orchestrator) *Server {
	// Initialize App
	limiter := ratelimit.NewWindowCounter(ctrl.Config.App.RateLimitWindow, ctrl.Config.App.RateLimit)

	agentsAPI := agents.NewAPI(ctrl, orchestrator)
	hooksAPI := hooks.NewAPI(ctrl, orchestrator)

	server := &Server{
		Ctrl:         ctrl,
		Orchestrator: orchestrator,

		RateLimiter: limiter,
		AgentsAPI:   agentsAPI,
		HooksAPI:    hooksAPI,
	}

	return server
}

func (a *Server) GetController() *controller.Controller {
	return a.Ctrl
}

func (server *Server) HandleRequest(request requests.Request) (interface{}, error) {
	const op = "Server.HandleRequest"

	var requester *user.User

	defer func() { logRequest(request, requester) }()

	// Step 2: Validate the request
	err := request.GetBody().Validate()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	// Step 3: Handle unauthenticated requests
	response, err := server.handleRequest(request)
	if err != nil {
		return response, ez.Wrap(op, err)
	} else if response != nil {
		return response, nil
	}
	//
	// // Step 4: Authenticate the user
	// requester, err = server.authenticateRequester(request)
	// if err != nil {
	// 	err = server.TranslateError(op, err, request, requester)
	// 	return nil, ez.Wrap(op, err)
	// }
	//
	// // Step 5: Handle authenticated requests
	// response, err = server.HandleAuthenticatedRequest(request, requester)
	// if err != nil {
	// 	return response, ez.Wrap(op, err)
	// }

	// return response, nil
	return nil, nil
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
