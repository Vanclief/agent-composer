package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/schedules"
	"github.com/vanclief/compose/components/rest/requests"
)

// The :id of a schedule route is a UUID or a name, as for tasks.

// GET /schedules
func (h *Handler) ListSchedules(c echo.Context) error {
	const op = "Handler.ListSchedules"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &schedules.ListRequest{})
}

// POST /schedules
func (h *Handler) CreateSchedule(c echo.Context) error {
	const op = "Handler.CreateSchedule"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.BindedJSONResponse(c, op, request, &schedules.CreateRequest{})
}

// GET /schedules/:id
func (h *Handler) GetSchedule(c echo.Context) error {
	const op = "Handler.GetSchedule"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &schedules.GetRequest{ID: id, Name: name})
}

// PUT /schedules/:id replaces the schedule; the path wins over the body.
func (h *Handler) UpdateSchedule(c echo.Context) error {
	const op = "Handler.UpdateSchedule"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &schedules.UpdateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.ID, body.Name = taskRef(c)

	return h.JSONResponse(c, op, request, body)
}

// DELETE /schedules/:id
func (h *Handler) DeleteSchedule(c echo.Context) error {
	const op = "Handler.DeleteSchedule"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &schedules.DeleteRequest{ID: id, Name: name})
}

// POST /schedules/:id/run
func (h *Handler) RunSchedule(c echo.Context) error {
	const op = "Handler.RunSchedule"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &schedules.RunRequest{ID: id, Name: name})
}

// GET /schedules/:id/items
func (h *Handler) ListScheduleItems(c echo.Context) error {
	const op = "Handler.ListScheduleItems"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &schedules.ListItemsRequest{ID: id, Name: name})
}
