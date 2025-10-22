package handler

import (
    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "github.com/vanclief/compose/components/rest/requests"
    "github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
    "github.com/vanclief/ez"
    "github.com/vanclief/agent-composer/models/parrot"
    "github.com/vanclief/agent-composer/server/resources/parrots/runs"
)

func (h *Handler) ListParrotRuns(c echo.Context) error {
	const op = "Handler.ListParrotRuns"

	request := requests.New(c.Request().Header, c.RealIP())

    requestBody := &runs.ListRequest{
        CursorRequest: pagination.CursorRequest{
            Limit:  h.GetListLimit(c, 50),
            Cursor: c.QueryParam("cursor"),
        },
        Search: c.QueryParam("search"),
    }

    templateIDStr := c.QueryParam("template_id")
    if templateIDStr != "" {
        templateID, err := uuid.Parse(templateIDStr)
        if err != nil || templateID == uuid.Nil {
            return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid template_id", err))
        }

        requestBody.TemplateID = templateID
    }

    // Optional status filter
    statusStr := c.QueryParam("status")
    if statusStr != "" {
        s := parrot.RunStatus(statusStr)
        if err := s.Validate(); err != nil {
            return h.ManageError(c, op, request, ez.New(op, ez.EINVALID, "invalid status", err))
        }
        requestBody.Status = &s
    }

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetParrotRun(c echo.Context) error {
	const op = "Handler.GetParrotRun"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &runs.GetRequest{
		ParrotRunID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) DeleteParrotRun(c echo.Context) error {
	const op = "Handler.DeleteParrotRun"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &runs.DeleteRequest{
		ParrotRunID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}
