package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/server/resources/agents/sessions"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

func (h *Handler) ListAgentSessions(c echo.Context) error {
	const op = "Handler.ListAgentSessions"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &sessions.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		Search: c.QueryParam("search"),
	}

	if agentSpecIDStr := c.QueryParam("agent_spec_id"); agentSpecIDStr != "" {
		agentSpecID, err := uuid.Parse(agentSpecIDStr)
		if err != nil || agentSpecID == uuid.Nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid agent_spec_id", err))
		}
		requestBody.AgentSpecID = agentSpecID
	}

	if providerParam := c.QueryParam("provider"); providerParam != "" {
		prov := agent.LLMProvider(providerParam)
		if err := prov.Validate(); err != nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid provider", err))
		}
		requestBody.Provider = &prov
	}

	if statusStr := c.QueryParam("status"); statusStr != "" {
		status := agent.SessionStatus(statusStr)
		if err := status.Validate(); err != nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid status", err))
		}
		requestBody.Status = &status
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetAgentSession(c echo.Context) error {
	const op = "Handler.GetAgentSession"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &sessions.GetRequest{
		AgentSessionID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) DeleteAgentSession(c echo.Context) error {
	const op = "Handler.DeleteAgentSession"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &sessions.DeleteRequest{
		AgentSessionID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}
