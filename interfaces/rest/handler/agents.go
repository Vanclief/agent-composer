package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/compose/components/rest/requests"
)

// Persistent agents (ADR 0016). :name is the agent's name.

// GET /agents
func (h *Handler) ListAgents(c echo.Context) error {
	const op = "Handler.ListAgents"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &agents.ListRequest{})
}

// POST /agents answers 201 with the numbered agent, asleep, no home.
func (h *Handler) CreateAgent(c echo.Context) error {
	const op = "Handler.CreateAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &agents.CreateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	request.SetBody(body)

	response, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.JSON(http.StatusCreated, response)
}

// GET /agents/:name
func (h *Handler) GetAgent(c echo.Context) error {
	const op = "Handler.GetAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &agents.GetRequest{Name: c.Param("name")})
}

// POST /agents/:name/keep; the path names the agent, the body its new
// name.
func (h *Handler) KeepAgent(c echo.Context) error {
	const op = "Handler.KeepAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &agents.KeepRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.Agent = c.Param("name")

	return h.JSONResponse(c, op, request, body)
}

// POST /agents/:name/wake
func (h *Handler) WakeAgent(c echo.Context) error {
	const op = "Handler.WakeAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &agents.WakeRequest{Name: c.Param("name")})
}

// POST /agents/:name/sleep
func (h *Handler) SleepAgent(c echo.Context) error {
	const op = "Handler.SleepAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &agents.SleepRequest{Name: c.Param("name")})
}

// DELETE /agents/:name
func (h *Handler) DeleteAgent(c echo.Context) error {
	const op = "Handler.DeleteAgent"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &agents.DeleteRequest{Name: c.Param("name")})
}
