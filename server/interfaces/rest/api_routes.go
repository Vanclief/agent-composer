package rest

import (
	"github.com/labstack/echo/v4"
	"github.com/vanclief/agent-composer/server/interfaces/rest/handler"
)

func addAPIRoutes(e *echo.Echo, h *handler.Handler) {
	// API
	api := e.Group("/api")

	// Parrots
	parrots := api.Group("/parrots")

	templates := parrots.Group("/templates")
	templates.GET("", h.ListParrotTemplatess)
	templates.GET("/:id", h.GetParrotTemplate)
	templates.POST("", h.CreateParrotTemplate)
	templates.PUT("/:id", h.UpdateParrotTemplate)
	templates.DELETE("/:id", h.DeleteParrotTemplate)
	templates.POST("/:id/run", h.RunParrotTemplate)

	runs := parrots.Group("/runs")
	runs.GET("", h.ListParrotRuns)
	runs.GET("/:id", h.GetParrotRun)
	runs.DELETE("/:id", h.DeleteParrotRun)

	// Hooks
	hooks := api.Group("/hooks")
	hooks.GET("", h.ListHooks)
	hooks.GET("/:id", h.GetHook)
	hooks.POST("", h.CreateHook)
	hooks.PUT("/:id", h.UpdateHook)
	hooks.DELETE("/:id", h.DeleteHook)
}
