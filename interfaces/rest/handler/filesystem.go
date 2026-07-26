package handler

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/filesystem"
	"github.com/vanclief/compose/components/rest/requests"
)

func (h *Handler) BrowseDirectories(c echo.Context) error {
	const op = "Handler.BrowseDirectories"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &filesystem.BrowseRequest{
		Path: strings.TrimSpace(c.QueryParam("path")),
	}

	return h.JSONResponse(c, op, request, body)
}
