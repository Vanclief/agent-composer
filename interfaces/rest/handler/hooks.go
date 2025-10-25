package handler

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/hooks"
	"github.com/vanclief/agent-composer/runtime/events"
	"github.com/vanclief/compose/components/rest/requests"
	"github.com/vanclief/compose/drivers/databases/relational/postgres/pagination"
)

func (h *Handler) ListHooks(c echo.Context) error {
	const op = "Handler.ListHooks"

	req := requests.New(c.Request().Header, c.RealIP())

	body := &hooks.ListRequest{
		CursorRequest: pagination.CursorRequest{
			Limit:  h.GetListLimit(c, 50),
			Cursor: c.QueryParam("cursor"),
		},
		Search: c.QueryParam("search"),
	}

	// Optional filters
	if v := strings.TrimSpace(c.QueryParam("event_type")); v != "" {
		et := events.EventType(v)
		body.EventType = &et
	}
	if v := strings.TrimSpace(c.QueryParam("template_name")); v != "" {
		body.TemplateName = &v
	}

	return h.JSONResponse(c, op, req, body)
}

// GET /hooks/:id
func (h *Handler) GetHook(c echo.Context) error {
	const op = "Handler.GetHook"

	req := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, req, err)
	}

	body := &hooks.GetRequest{
		HookID: resourceID,
	}

	return h.JSONResponse(c, op, req, body)
}

// POST /hooks
func (h *Handler) CreateHook(c echo.Context) error {
	const op = "Handler.CreateHook"

	req := requests.New(c.Request().Header, c.RealIP())

	body := &hooks.CreateRequest{}
	return h.BindedJSONResponse(c, op, req, body)
}

// PATCH /hooks/:id
func (h *Handler) UpdateHook(c echo.Context) error {
	const op = "Handler.UpdateHook"

	req := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, req, err)
	}

	body := &hooks.UpdateRequest{
		HookID: resourceID,
	}
	return h.BindedJSONResponse(c, op, req, body)
}

// DELETE /hooks/:id
func (h *Handler) DeleteHook(c echo.Context) error {
	const op = "Handler.DeleteHook"

	req := requests.New(c.Request().Header, c.RealIP())

	resourceID, err := h.GetParameterUUID(c, "id")
	if err != nil {
		return h.ManageError(c, op, req, err)
	}

	body := &hooks.DeleteRequest{
		HookID: resourceID,
	}
	return h.JSONResponse(c, op, req, body)
}
