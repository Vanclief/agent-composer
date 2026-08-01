package rest

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
)

func addAPIRoutes(e *echo.Echo, h *handler.Handler) {
	// API
	api := e.Group("/api")
	api.GET("/config", h.GetConfig)

	// Hooks
	hooks := api.Group("/hooks")
	hooks.GET("", h.ListHooks)
	hooks.GET("/:id", h.GetHook)
	hooks.POST("", h.CreateHook)
	hooks.PUT("/:id", h.UpdateHook)
	hooks.DELETE("/:id", h.DeleteHook)

	workflows := api.Group("/workflows")
	workflows.GET("", h.ListWorkflows)
	workflows.POST("/compose", h.ComposeWorkflow)
	workflows.GET("/:id", h.GetWorkflow)
	workflows.PUT("/:id/nodes/:node", h.UpdateWorkflowNode)
	workflows.POST("/:id/save", h.SaveWorkflowDraft)
	workflows.DELETE("/:id/draft", h.DeleteWorkflowDraft)

	api.GET("/settings", h.GetSettings)
	api.PUT("/settings", h.UpdateSettings)

	workflow := api.Group("/workflow")
	workflowExecutions := workflow.Group("/executions")
	workflowExecutions.GET("", h.ListWorkflowExecutions)
	workflowExecutions.POST("", h.CreateWorkflowExecution)
	workflowExecutions.GET("/:id", h.GetWorkflowExecution)

	workflowNodeExecutions := workflow.Group("/node-executions")
	workflowNodeExecutions.GET("", h.ListNodeExecutions)
	workflowNodeExecutions.GET("/:id", h.GetNodeExecution)

	workflowConversations := workflow.Group("/conversations")
	workflowConversations.GET("", h.ListConversations)

	api.GET("/filesystem/directories", h.BrowseDirectories)
	api.GET("/harnesses", h.ListHarnesses)

	worktrees := api.Group("/worktrees")
	worktrees.GET("", h.ListWorktrees)
	worktrees.POST("", h.CreateWorktree)
	worktrees.DELETE("", h.RemoveWorktree)
}
