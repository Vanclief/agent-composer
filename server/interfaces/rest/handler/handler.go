package handler

import (
	"github.com/vanclief/agent-composer/server"
	"github.com/vanclief/agent-composer/server/controller"

	baseHandler "github.com/vanclief/compose/components/rest/handler"
)

// Handler is a struct with basic methods that should be extended to properly handle a HTTP Service.
type Handler struct {
	baseHandler.BaseHandler
	server *server.Server
	ctrl   *controller.Controller
}

func NewHandler(server *server.Server) *Handler {
	h := baseHandler.NewHandler(server)
	return &Handler{
		BaseHandler: *h,
		server:      server,
		ctrl:        server.GetController(),
	}
}
