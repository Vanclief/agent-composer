package handler

import (
	"strings"

	"github.com/labstack/echo/v4"

	workflowworktrees "github.com/vanclief/agent-composer/core/resources/workflow/worktrees"
	"github.com/vanclief/compose/components/rest/requests"
)

func (h *Handler) ListWorktrees(c echo.Context) error {
	const op = "Handler.ListWorktrees"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowworktrees.ListRequest{
		Repo:  strings.TrimSpace(c.QueryParam("repo")),
		Fetch: c.QueryParam("fetch") == "true",
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) CreateWorktree(c echo.Context) error {
	const op = "Handler.CreateWorktree"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowworktrees.CreateRequest{}
	err := c.Bind(body)
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return h.JSONResponse(c, op, request, body)
}

func (h *Handler) RemoveWorktree(c echo.Context) error {
	const op = "Handler.RemoveWorktree"

	request := requests.New(c.Request().Header, c.RealIP())

	body := &workflowworktrees.DeleteRequest{
		Repo:  strings.TrimSpace(c.QueryParam("repo")),
		Path:  strings.TrimSpace(c.QueryParam("path")),
		Force: c.QueryParam("force") == "true",
	}

	return h.JSONResponse(c, op, request, body)
}
