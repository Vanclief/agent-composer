package handler

import (
	baseHandler "github.com/vanclief/compose/components/rest/handler"

	"github.com/vanclief/agent-composer/core/controller"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
)

// Handler is a struct with basic methods that should be extended to properly handle a HTTP Service.
type Handler struct {
	baseHandler.BaseHandler
	server *restserver.Server
	ctrl   *controller.Controller
}

func NewHandler(server *restserver.Server) *Handler {
	h := baseHandler.NewHandler(server)
	return &Handler{
		BaseHandler: *h,
		server:      server,
		ctrl:        server.GetController(),
	}
}
