package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /sandboxes?work=1
func (h *Handler) ListSandboxes(c echo.Context) error {
	const op = "Handler.ListSandboxes"

	request := requests.New(c.Request().Header, c.RealIP())
	work := c.QueryParam("work")

	return h.JSONResponse(c, op, request, &sandboxes.ListRequest{Work: work == "1" || work == "true"})
}

// GET /sandboxes/:id
func (h *Handler) GetSandbox(c echo.Context) error {
	const op = "Handler.GetSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.GetRequest{ID: id, Name: name})
}

// POST /sandboxes/:id/stop
func (h *Handler) StopSandbox(c echo.Context) error {
	const op = "Handler.StopSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.StopRequest{ID: id, Name: name})
}

// POST /sandboxes/:id/start
func (h *Handler) StartSandbox(c echo.Context) error {
	const op = "Handler.StartSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.StartRequest{ID: id, Name: name})
}

// DELETE /sandboxes/:id
func (h *Handler) DeleteSandbox(c echo.Context) error {
	const op = "Handler.DeleteSandbox"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &sandboxes.DeleteRequest{ID: id, Name: name})
}
