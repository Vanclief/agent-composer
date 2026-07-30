package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/harnessinfo"
	"github.com/vanclief/compose/components/rest/requests"
)

func (h *Handler) ListHarnesses(c echo.Context) error {
	const op = "Handler.ListHarnesses"

	request := requests.New(c.Request().Header, c.RealIP())
	body := &harnessinfo.ListRequest{}

	return h.JSONResponse(c, op, request, body)
}
