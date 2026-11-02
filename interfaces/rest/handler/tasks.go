package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/compose/components/rest/requests"
)

// taskRef reads the :id path segment, which is a UUID or a name. Task and
// sandbox names can never parse as UUIDs, so the two cannot be confused.
func taskRef(c echo.Context) (uuid.UUID, string) {
	return parseTaskRef(c.Param("id"))
}

func parseTaskRef(param string) (uuid.UUID, string) {
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

// POST /tasks/:id/done
func (h *Handler) DoneTask(c echo.Context) error {
	const op = "Handler.DoneTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.DoneRequest{ID: id, Name: name})
}

// POST /tasks/:id/sandboxes answers 201 with the sandbox starting.
func (h *Handler) AllocateSandbox(c echo.Context) error {
	const op = "Handler.AllocateSandbox"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &sandboxes.AllocateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.TaskID, body.TaskName = taskRef(c)

	request.SetBody(body)

	response, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.JSON(http.StatusCreated, response)
}

// DELETE /tasks/:id
func (h *Handler) DeleteTask(c echo.Context) error {
	const op = "Handler.DeleteTask"

	request := requests.New(c.Request().Header, c.RealIP())
	id, name := taskRef(c)

	return h.JSONResponse(c, op, request, &tasks.DeleteRequest{ID: id, Name: name})
}
