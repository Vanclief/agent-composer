package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /secrets
func (h *Handler) ListSecrets(c echo.Context) error {
	const op = "Handler.ListSecrets"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &tasks.ListSecretsRequest{})
}
