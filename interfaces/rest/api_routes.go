package rest

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
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

	conversations := agents.Group("/conversations")
	conversations.GET("", h.ListConversations)
	conversations.POST("", h.CreateConversation)
	conversations.GET("/:id", h.GetConversation)
	conversations.POST("/:id/fork", h.ForkConversation)
	conversations.POST("/:id/resume", h.ResumeConversation)
	conversations.DELETE("/:id", h.DeleteConversation)

	// Hooks
	hooks := api.Group("/hooks")
	hooks.GET("", h.ListHooks)
	hooks.GET("/:id", h.GetHook)
	hooks.POST("", h.CreateHook)
	hooks.PUT("/:id", h.UpdateHook)
	hooks.DELETE("/:id", h.DeleteHook)
}
