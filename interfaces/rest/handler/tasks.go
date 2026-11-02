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

// POST /tasks/:id/deliver
func (h *Handler) DeliverTask(c echo.Context) error {
	body := &tasks.DeliverRequest{}

	return h.taskBodyResponse(c, "Handler.DeliverTask", body, func(id uuid.UUID, name string) {
		body.ID, body.Name = id, name
	})
}

// POST /tasks/:id/accept
func (h *Handler) AcceptTask(c echo.Context) error {
	body := &tasks.AcceptRequest{}

	return h.taskBodyResponse(c, "Handler.AcceptTask", body, func(id uuid.UUID, name string) {
		body.ID, body.Name = id, name
	})
}

// POST /tasks/:id/reject
func (h *Handler) RejectTask(c echo.Context) error {
	body := &tasks.RejectRequest{}

	return h.taskBodyResponse(c, "Handler.RejectTask", body, func(id uuid.UUID, name string) {
		body.ID, body.Name = id, name
	})
}

// taskBodyResponse binds an optional JSON body into body, then sets the
// task from the path with setRef, so the path always wins over the body.
func (h *Handler) taskBodyResponse(c echo.Context, op string, body requests.Body, setRef func(id uuid.UUID, name string)) error {
	request := requests.New(c.Request().Header, c.RealIP())

	if c.Request().ContentLength != 0 {
		err := c.Bind(body)
		if err != nil {
			return h.ManageError(c, op, request, err)
		}
	}
	setRef(taskRef(c))

	request.SetBody(body)

	response, err := h.App.HandleRequest(request)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.JSON(http.StatusOK, response)
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
