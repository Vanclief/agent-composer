package handler

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/core/resources/knowledge"
	"github.com/vanclief/compose/components/rest/requests"
)

// GET /knowledge?without=id1,id2
func (h *Handler) ListKnowledge(c echo.Context) error {
	const op = "Handler.ListKnowledge"

	request := requests.New(c.Request().Header, c.RealIP())

	without := []string{}
	for _, id := range strings.Split(c.QueryParam("without"), ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			without = append(without, id)
		}
	}

	return h.JSONResponse(c, op, request, &knowledge.ListRequest{Without: without})
}

// GET /knowledge/why/:id
func (h *Handler) KnowledgeWhy(c echo.Context) error {
	const op = "Handler.KnowledgeWhy"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &knowledge.WhyRequest{ID: c.Param("id")})
}

// GET /knowledge/doubt/:id
func (h *Handler) KnowledgeDoubt(c echo.Context) error {
	const op = "Handler.KnowledgeDoubt"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &knowledge.DoubtRequest{ID: c.Param("id")})
}

// GET /knowledge/conflicts
func (h *Handler) KnowledgeConflicts(c echo.Context) error {
	const op = "Handler.KnowledgeConflicts"

	request := requests.New(c.Request().Header, c.RealIP())

	return h.JSONResponse(c, op, request, &knowledge.ConflictsRequest{})
}
