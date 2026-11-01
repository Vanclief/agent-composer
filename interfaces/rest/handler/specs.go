package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /specs
func (h *Handler) ListSpecs(c echo.Context) error {
	const op = "Handler.ListSpecs"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &tasks.ListSpecsRequest{})
}

// GET /specs/:name
func (h *Handler) GetSpec(c echo.Context) error {
	const op = "Handler.GetSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &tasks.GetSpecRequest{Name: c.Param("name")})
}

// PUT /specs/:name
func (h *Handler) SaveSpec(c echo.Context) error {
	const op = "Handler.SaveSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &tasks.SaveSpecRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}
	body.Name = c.Param("name")

	return h.JSONResponse(c, op, request, body)
}

// DELETE /specs/:name
func (h *Handler) DeleteSpec(c echo.Context) error {
	const op = "Handler.DeleteSpec"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &tasks.DeleteSpecRequest{Name: c.Param("name")})
}
