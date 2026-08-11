package handler

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	workflowconversations "github.com/vanclief/agent-composer/core/resources/workflow/conversations"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/ez"
)

func (h *Handler) ListConversations(c echo.Context) error {
	const op = "Handler.ListConversations"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowconversations.ListRequest{}

	filter := strings.TrimSpace(c.QueryParam("node_execution_id"))
	if filter != "" {
		resourceID, err := uuid.Parse(filter)
		if err != nil {
			parseErr := ez.New(ez.EINVALID, "Could not parse query param node_execution_id to UUID", err)
			return h.ManageError(c, op, request, parseErr)
		}

		body.NodeExecutionID = resourceID
	}

	return h.JSONResponse(c, op, request, body)
}
