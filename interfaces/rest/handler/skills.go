package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/skills"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /skills
func (h *Handler) ListSkills(c echo.Context) error {
	const op = "Handler.ListSkills"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &skills.ListRequest{})
}

// GET /skills/:name
//
// Serves the skill's SKILL.md as markdown. A plain echo handler, not the
// request dispatch, so the text goes out as it is.
func (h *Handler) GetSkill(c echo.Context) error {
	const op = "Handler.GetSkill"

	request := requests.New(c.Request().Header, c.RealIP())

	skill, err := h.server.SkillsAPI.Get(c.Request().Context(), nil, &skills.GetRequest{Name: c.Param("name")})
	if err != nil {
		return h.ManageError(c, op, request, err)
	}

	return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(skill.Text))
}
