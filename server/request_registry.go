package server

import (
	"github.com/vanclief/agent-composer/server/resources/agents/sessions"
	"github.com/vanclief/agent-composer/server/resources/agents/specs"
	"github.com/vanclief/agent-composer/server/resources/hooks"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

// TODO: Add requester here
func (s *Server) handleRequest(request requests.Request) (interface{}, error) {
	// Step 2: Handle unauthenticated requests
	switch body := request.GetBody().(type) {

	// Agents
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
	case *specs.SessionRequest:
		return s.AgentsAPI.AgentSpecs.StartSessions(request.GetContext(), nil, body)

	case *sessions.ListRequest:
		return s.AgentsAPI.Sessions.List(request.GetContext(), nil, body)
	case *sessions.GetRequest:
		return s.AgentsAPI.Sessions.Get(request.GetContext(), nil, body)
	case *sessions.DeleteRequest:
		return s.AgentsAPI.Sessions.Delete(request.GetContext(), nil, body)

		// Hooks
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
		return nil, ez.New("Server.handleRequest", ez.EINVALID, "Unsupported request type", nil)
	}
}
