package rest

import (
	"github.com/labstack/echo/v4"

	"github.com/vanclief/agent-composer/interfaces/rest/handler"
)

func addAPIRoutes(e *echo.Echo, h *handler.Handler) {
	// API
	api := e.Group("/api")
	api.GET("/config", h.GetConfig)
	api.GET("/events", h.Events)

	// Hooks
	hooks := api.Group("/hooks")
	hooks.GET("", h.ListHooks)
	hooks.GET("/:id", h.GetHook)
	hooks.POST("", h.CreateHook)
	hooks.PUT("/:id", h.UpdateHook)
	hooks.DELETE("/:id", h.DeleteHook)

	workflows := api.Group("/workflows")
	workflows.GET("", h.ListWorkflows)
	workflows.POST("", h.CreateWorkflow)
	workflows.POST("/compose", h.ComposeWorkflow)
	workflows.GET("/:slug", h.GetWorkflow)
	workflows.PUT("/:slug/nodes/:node", h.UpdateWorkflowNode)
	workflows.PUT("/:slug", h.RenameWorkflow)
	workflows.DELETE("/:slug", h.DeleteWorkflow)
	workflows.POST("/:slug/save", h.SaveWorkflowDraft)
	workflows.DELETE("/:slug/draft", h.DeleteWorkflowDraft)

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

	api.GET("/defaults", h.GetDefaults)

	tasks := api.Group("/tasks")
	tasks.GET("", h.ListTasks)
	tasks.POST("", h.CreateTask)
	tasks.GET("/:id", h.GetTask)
	tasks.POST("/:id/deliver", h.DeliverTask)
	tasks.POST("/:id/accept", h.AcceptTask)
	tasks.POST("/:id/reject", h.RejectTask)
	tasks.POST("/:id/sandboxes", h.AllocateSandbox)
	tasks.DELETE("/:id", h.DeleteTask)

	sandboxes := api.Group("/sandboxes")
	sandboxes.GET("", h.ListSandboxes)
	sandboxes.GET("/:id", h.GetSandbox)
	sandboxes.POST("/:id/stop", h.StopSandbox)
	sandboxes.POST("/:id/start", h.StartSandbox)
	sandboxes.DELETE("/:id", h.DeleteSandbox)
	sandboxes.GET("/:id/terminals", h.ListTerminals)
	sandboxes.POST("/:id/terminals", h.StartTerminal)
	sandboxes.GET("/:id/terminals/:job/ws", h.TerminalSocket)
	sandboxes.GET("/:id/transcripts", h.ListTranscripts)
	sandboxes.GET("/:id/transcripts/*", h.GetTranscript)

	schedules := api.Group("/schedules")
	schedules.GET("", h.ListSchedules)
	schedules.POST("", h.CreateSchedule)
	schedules.GET("/:id", h.GetSchedule)
	schedules.PUT("/:id", h.UpdateSchedule)
	schedules.DELETE("/:id", h.DeleteSchedule)
	schedules.POST("/:id/run", h.RunSchedule)
	schedules.GET("/:id/items", h.ListScheduleItems)

	specs := api.Group("/specs")
	specs.GET("", h.ListSpecs)
	specs.GET("/:name", h.GetSpec)
	specs.PUT("/:name", h.SaveSpec)
	specs.DELETE("/:name", h.DeleteSpec)

	api.GET("/secrets", h.ListSecrets)

	knowledge := api.Group("/knowledge")
	knowledge.GET("", h.ListKnowledge)
	knowledge.GET("/why/:id", h.KnowledgeWhy)
	knowledge.GET("/doubt/:id", h.KnowledgeDoubt)
	knowledge.GET("/conflicts", h.KnowledgeConflicts)

	api.Any("/msg/*", echo.WrapHandler(h.Messaging()))
}
