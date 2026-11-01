package handler

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/compose/components/rest/requests"
)

// taskRef reads the :id path segment, which is a task UUID or a task name.
// Task names can never parse as UUIDs, so the two cannot be confused.
func taskRef(c echo.Context) (uuid.UUID, string) {
	param := c.Param("id")

	id, err := uuid.Parse(param)
	if err == nil {
		return id, ""
	}

	return uuid.Nil, param
}

// GET /tasks
func (h *Handler) ListTasks(c echo.Context) error {
	const op = "Handler.ListTasks"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &tasks.ListRequest{})
}

// POST /tasks
func (h *Handler) CreateTask(c echo.Context) error {
	const op = "Handler.CreateTask"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.BindedJSONResponse(c, op, request, &tasks.CreateRequest{})
}

// GET /tasks/:id
func (h *Handler) GetTask(c echo.Context) error {
	const op = "Handler.GetTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.GetRequest{ID: id, Name: name})
}

// POST /tasks/:id/stop
func (h *Handler) StopTask(c echo.Context) error {
	const op = "Handler.StopTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.StopRequest{ID: id, Name: name})
}

// POST /tasks/:id/start
func (h *Handler) StartTask(c echo.Context) error {
	const op = "Handler.StartTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.StartRequest{ID: id, Name: name})
}

// DELETE /tasks/:id
func (h *Handler) DeleteTask(c echo.Context) error {
	const op = "Handler.DeleteTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.DeleteRequest{ID: id, Name: name})
}
