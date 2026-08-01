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

func (h *Handler) ComposeWorkflow(c echo.Context) error {
	const op = "Handler.ComposeWorkflow"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowapi.ComposeRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) SaveWorkflowDraft(c echo.Context) error {
	const op = "Handler.SaveWorkflowDraft"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &workflowapi.SaveDraftRequest{
		WorkflowID: strings.TrimSpace(c.Param("id")),
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) DeleteWorkflowDraft(c echo.Context) error {
	const op = "Handler.DeleteWorkflowDraft"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &workflowapi.DeleteDraftRequest{
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
