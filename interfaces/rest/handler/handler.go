package handler

import (
	"context"

	baseHandler "github.com/vanclief/compose/components/rest/handler"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/executor"
	restserver "github.com/vanclief/agent-composer/interfaces/rest/server"
)

// Handler is a struct with basic methods that should be extended to properly handle a HTTP Service.
type Handler struct {
	baseHandler.BaseHandler
	server *restserver.Server
	ctrl   *controller.Controller
	// ConnectTerminal backs the terminal WebSocket. Tests replace it.
	ConnectTerminal ConnectTerminal
}

func NewHandler(server *restserver.Server) *Handler {
	h := baseHandler.NewHandler(server)
	return &Handler{
		BaseHandler: *h,
		server:      server,
		ctrl:        server.Ctrl,
		ConnectTerminal: func(ctx context.Context, ref string) (executor.TerminalExecutor, func(), error) {
			id, name := parseTaskRef(ref)
			return server.TasksAPI.TerminalExecutor(ctx, id, name)
		},
	}
}
