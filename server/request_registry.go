package server

import (
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/server/resources/hooks"
	"github.com/vanclief/agent-composer/server/resources/parrots/runs"
	"github.com/vanclief/agent-composer/server/resources/parrots/templates"
)

// TODO: Add requester here
func (s *Server) handleRequest(request requests.Request) (interface{}, error) {
	// Step 2: Handle unauthenticated requests
	switch body := request.GetBody().(type) {

	// Parrots
	case *templates.ListRequest:
		return s.ParrotsAPI.Templates.List(request.GetContext(), nil, body)
	case *templates.GetRequest:
		return s.ParrotsAPI.Templates.Get(request.GetContext(), nil, body)
	case *templates.CreateRequest:
		return s.ParrotsAPI.Templates.Create(request.GetContext(), nil, body)
	case *templates.UpdateRequest:
		return s.ParrotsAPI.Templates.Update(request.GetContext(), nil, body)
	case *templates.DeleteRequest:
		return s.ParrotsAPI.Templates.Delete(request.GetContext(), nil, body)
	case *templates.RunRequest:
		return s.ParrotsAPI.Templates.Run(request.GetContext(), nil, body)

	case *runs.ListRequest:
		return s.ParrotsAPI.Runs.List(request.GetContext(), nil, body)
	case *runs.GetRequest:
		return s.ParrotsAPI.Runs.Get(request.GetContext(), nil, body)

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
