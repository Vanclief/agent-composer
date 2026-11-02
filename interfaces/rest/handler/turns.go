package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/agents"
	"github.com/vanclief/compose/components/rest/requests"
)

// Turns are read from the harness session files on a volume (ADR 0016
// item 5): an agent's agc-<name>, or a task sandbox's.

// GET /agents/:name/turns?limit=&before=
func (h *Handler) ListAgentTurns(c echo.Context) error {
	const op = "Handler.ListAgentTurns"

	request := requests.New(c.Request().Header, c.RealIP())

	page, err := turnsPage(c)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, &agents.ListTurnsRequest{Name: c.Param("name"), TurnsPage: page})
}

// GET /agents/:name/turns/:n
func (h *Handler) GetAgentTurn(c echo.Context) error {
	const op = "Handler.GetAgentTurn"

	request := requests.New(c.Request().Header, c.RealIP())

	n, err := intParam(c.Param("n"), "n")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, &agents.GetTurnRequest{Name: c.Param("name"), N: n})
}

// GET /sandboxes/:id/turns?limit=&before=
func (h *Handler) ListSandboxTurns(c echo.Context) error {
	const op = "Handler.ListSandboxTurns"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	page, err := turnsPage(c)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, &agents.ListSandboxTurnsRequest{ID: id, Name: name, TurnsPage: page})
}

// GET /sandboxes/:id/turns/:n
func (h *Handler) GetSandboxTurn(c echo.Context) error {
	const op = "Handler.GetSandboxTurn"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	n, err := intParam(c.Param("n"), "n")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, &agents.GetSandboxTurnRequest{ID: id, Name: name, N: n})
}

func turnsPage(c echo.Context) (agents.TurnsPage, error) {
	limit, err := intParam(c.QueryParam("limit"), "limit")
	if err != nil {
		return agents.TurnsPage{}, err
	}

	before, err := intParam(c.QueryParam("before"), "before")
	if err != nil {
		return agents.TurnsPage{}, err
	}

	return agents.TurnsPage{Limit: limit, Before: before}, nil
}

// intParam reads a whole number from the path or query, 0 when empty.
func intParam(value string, name string) (int, error) {
	if value == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, ez.New(ez.EINVALID, name+" must be a whole number", err)
	}

	return n, nil
}
