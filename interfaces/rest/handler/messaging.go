package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// humanCaller is who calls agc's REST API: a person.
func humanCaller(*http.Request) (messaging.Caller, error) {
	return messaging.Caller{Human: true}, nil
}

// GET /msg/images/:id/:name
//
// Serves a message's image as it was posted (ADR 0005 "Images"). A plain
// echo handler, not the request dispatch, so the bytes go out as they are.
func (h *Handler) GetMessageImage(c echo.Context) error {
	httpapi.ServeImage(c.Response(), c.Request(), h.server.Messaging, messaging.Caller{Human: true}, c.Param("id"), c.Param("name"))

	return nil
}

// Messaging serves the messaging API for humans, mounted at /api/msg. A
// human is bound by no channel rules (ADR 0005).
func (h *Handler) Messaging() http.Handler {
	return http.StripPrefix("/api/msg", httpapi.New(h.server.Messaging, humanCaller))
}
