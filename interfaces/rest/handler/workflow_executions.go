package handler

import (
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	workflownodeexecutions "github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/ez"
)

func (h *Handler) ListWorkflowExecutions(c echo.Context) error {
	const op = "Handler.ListWorkflowExecutions"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowexecutions.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		WorkflowID: strings.TrimSpace(c.QueryParam("workflow_id")),
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) GetConfig(c echo.Context) error {
	return c.JSON(200, map[string]string{
		"shell_root": h.server.WorkflowAPI.DefaultShellRoot(),
	})
}

func (h *Handler) CreateWorkflowExecution(c echo.Context) error {
	const op = "Handler.CreateWorkflowExecution"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &workflowexecutions.CreateRequest{}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetWorkflowExecution(c echo.Context) error {
	const op = "Handler.GetWorkflowExecution"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	body := &workflowexecutions.GetRequest{
		WorkflowExecutionID: resourceID,
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) ListNodeExecutions(c echo.Context) error {
	const op = "Handler.ListNodeExecutions"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflownodeexecutions.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
	}

	filter := strings.TrimSpace(c.QueryParam("workflow_execution_id"))
	if filter != "" {
		resourceID, err := uuid.Parse(filter)
		if err != nil {
			parseErr := ez.New(op, ez.EINVALID, "Could not parse query param workflow_execution_id to UUID", err)
			return h.ManageError(c, op, request, parseErr)
		}

		body.WorkflowExecutionID = resourceID
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) GetNodeExecution(c echo.Context) error {
	const op = "Handler.GetNodeExecution"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	body := &workflownodeexecutions.GetRequest{
		NodeExecutionID: resourceID,
	}

	return h.JSONResponse(c, op, request, body)
}
