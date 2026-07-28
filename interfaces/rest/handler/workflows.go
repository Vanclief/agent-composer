package handler

import (
	"strings"

	"github.com/labstack/echo/v4"

	workflowapi "github.com/vanclief/agent-composer/core/resources/workflow"
	"github.com/vanclief/compose/components/rest/requests"
)

func (h *Handler) ListWorkflows(c echo.Context) error {
	const op = "Handler.ListWorkflows"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &workflowapi.ListRequest{}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) GetWorkflow(c echo.Context) error {
	const op = "Handler.GetWorkflow"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &workflowapi.GetRequest{
		WorkflowID: strings.TrimSpace(c.Param("id")),
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) UpdateWorkflowNode(c echo.Context) error {
	const op = "Handler.UpdateWorkflowNode"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowapi.UpdateNodeRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.WorkflowID = strings.TrimSpace(c.Param("id"))
	body.Node = strings.TrimSpace(c.Param("node"))

	return h.JSONResponse(c, op, request, body)
}
