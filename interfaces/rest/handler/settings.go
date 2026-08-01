package handler

import (
	"github.com/labstack/echo/v4"

	settingsapi "github.com/vanclief/agent-composer/core/resources/settings"
	"github.com/vanclief/compose/components/rest/requests"
)

func (h *Handler) GetSettings(c echo.Context) error {
	const op = "Handler.GetSettings"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &settingsapi.GetRequest{}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) UpdateSettings(c echo.Context) error {
	const op = "Handler.UpdateSettings"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &settingsapi.UpdateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, body)
}
