package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/vanclief/agent-composer/core/resources/agents/conversations"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

func (h *Handler) ListConversations(c echo.Context) error {
	const op = "Handler.ListConversations"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &conversations.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		Search: c.QueryParam("search"),
	}

	sessionID := c.QueryParam("session_id")
	if sessionID != "" {
		requestBody.SessionID = sessionID
	}

	agentSpecIDStr := c.QueryParam("agent_spec_id")
	if agentSpecIDStr != "" {
		agentSpecID, err := uuid.Parse(agentSpecIDStr)
		if err != nil || agentSpecID == uuid.Nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid agent_spec_id", err))
		}
		requestBody.AgentSpecID = agentSpecID
	}

	providerParam := c.QueryParam("provider")
	if providerParam != "" {
		prov := agent.LLMProvider(providerParam)
		if err := prov.Validate(); err != nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid provider", err))
		}
		requestBody.Provider = &prov
	}

	statusStr := c.QueryParam("status")
	if statusStr != "" {
		status := agent.ConversationStatus(statusStr)
		if err := status.Validate(); err != nil {
			return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid status", err))
		}
		requestBody.Status = &status
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetConversation(c echo.Context) error {
	const op = "Handler.GetConversation"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &conversations.GetRequest{
		ConversationID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) CreateConversation(c echo.Context) error {
	const op = "Handler.CreateConversation"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &conversations.CreateRequest{}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) DeleteConversation(c echo.Context) error {
	const op = "Handler.DeleteConversation"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &conversations.DeleteRequest{
		ConversationID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) ForkConversation(c echo.Context) error {
	const op = "Handler.ForkConversation"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &conversations.ForkRequest{
		ConversationID: resourceID,
	}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) ResumeConversation(c echo.Context) error {
	const op = "Handler.ResumeConversation"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &conversations.ResumeRequest{
		ConversationID: resourceID,
	}

	return h.BindedJSONResponse(c, op, request, requestBody)
}
