package rest

import (
	"github.com/labstack/echo/v4"
	"github.com/vanclief/agent-composer/server/interfaces/rest/handler"
)

func addAPIRoutes(e *echo.Echo, h *handler.Handler) {
	// API
	api := e.Group("/api")

	// Agents
	agents := api.Group("/agents")

	specs := agents.Group("/specs")
	specs.GET("", h.ListAgentSpecs)
	specs.GET("/:id", h.GetAgentSpec)
	specs.POST("", h.CreateAgentSpec)
	specs.PUT("/:id", h.UpdateAgentSpec)
	specs.DELETE("/:id", h.DeleteAgentSpec)

	sessions := agents.Group("/sessions")
	sessions.GET("", h.ListAgentSessions)
	sessions.POST("", h.CreateAgentSession)
	sessions.GET("/:id", h.GetAgentSession)
	sessions.POST("/:id/fork", h.ForkAgentSession)
	sessions.POST("/:id/resume", h.ResumeAgentSession)
	sessions.DELETE("/:id", h.DeleteAgentSession)

	// Hooks
	hooks := api.Group("/hooks")
	hooks.GET("", h.ListHooks)
	hooks.GET("/:id", h.GetHook)
	hooks.POST("", h.CreateHook)
	hooks.PUT("/:id", h.UpdateHook)
	hooks.DELETE("/:id", h.DeleteHook)
}
