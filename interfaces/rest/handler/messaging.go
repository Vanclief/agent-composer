package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/messages"
	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
)

// humanCaller is who calls agc's REST API: a person.
func humanCaller(*http.Request) (messaging.Caller, error) {
	return messaging.Caller{Human: true}, nil
}

// GET /msg/images/:id/:name
//
// Serves a message's image as it was posted (ADR 0005 "Images"), read from
// the agc home. The request goes through the dispatch
// (messages.GetImageRequest); the handler streams the opened file as it is
// instead of answering JSON.
func (h *Handler) GetMessageImage(c echo.Context) error {
	const op = "Handler.GetMessageImage"

	request := requests.New(c.Request().Header, c.RealIP())

	id := c.Param("id")
	messageID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return h.ManageError(c, op, request, ez.New(ez.ENOTFOUND, "no message "+id, nil))
	}

	request.SetBody(&messages.GetImageRequest{MessageID: messageID, Name: c.Param("name")})

	response, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	opened, ok := response.(*messages.ImageResponse)
	if !ok {
		return h.ManageError(c, op, request, ez.New(ez.EINTERNAL, "the image request answered no image", nil))
	}
	defer opened.File.Close()

	err = httpapi.WriteImage(c.Response(), c.Request(), opened.File, &opened.Image)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return nil
}

// Messaging serves the messaging API for humans, mounted at /api/msg. A
// human is bound by no channel rules (ADR 0005).
func (h *Handler) Messaging() http.Handler {
	return http.StripPrefix("/api/msg", httpapi.New(h.server.Messaging, humanCaller))
}
