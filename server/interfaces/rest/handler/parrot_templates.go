package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
	"github.com/vanclief/agent-composer/server/resources/parrots/templates"
)

func (h *Handler) ListParrotTemplatess(c echo.Context) error {
	const op = "Handler.ListParrotTemplatess"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &templates.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		Search: c.QueryParam("search"),
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) GetParrotTemplate(c echo.Context) error {
	const op = "Handler.GetParrotTemplate"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &templates.GetRequest{
		ParrotTemplateID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) CreateParrotTemplate(c echo.Context) error {
	const op = "Handler.CreateParrotTemplate"

	request := requests.New(c.Request().Header, c.RealIP())

	requestBody := &templates.CreateRequest{}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) UpdateParrotTemplate(c echo.Context) error {
	const op = "Handler.UpdateParrotTemplate"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &templates.UpdateRequest{
		ParrotTemplateID: resourceID,
	}

	return h.BindedJSONResponse(c, op, request, requestBody)
}

func (h *Handler) DeleteParrotTemplate(c echo.Context) error {
	const op = "Handler.DeleteParrotTemplate"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &templates.DeleteRequest{
		ParrotTemplateID: resourceID,
	}

	return h.JSONResponse(c, op, request, requestBody)
}

func (h *Handler) RunParrotTemplate(c echo.Context) error {
	const op = "Handler.RunParrotTemplate"

	request := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	requestBody := &templates.RunRequest{
		ParrotTemplateID: resourceID,
	}

	return h.BindedJSONResponse(c, op, request, requestBody)
}
