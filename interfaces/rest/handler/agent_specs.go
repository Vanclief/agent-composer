package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/agents/specs"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
)

func (h *Handler) ListAgentSpecs(c echo.Context) error {
	const op = "Handler.ListAgentSpecs"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &specs.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		Search: c.QueryParam("search"),
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetAgentSpec(c echo.Context) error {
	const op = "Handler.GetAgentSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &specs.GetRequest{
		AgentSpecID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) CreateAgentSpec(c echo.Context) error {
	const op = "Handler.CreateAgentSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &specs.CreateRequest{}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) UpdateAgentSpec(c echo.Context) error {
	const op = "Handler.UpdateAgentSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &specs.UpdateRequest{
		AgentSpecID: resourceID,
	}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) DeleteAgentSpec(c echo.Context) error {
	const op = "Handler.DeleteAgentSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &specs.DeleteRequest{
		AgentSpecID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}
