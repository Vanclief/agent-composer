package agc

import (
	"context"

	"github.com/google/uuid"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vanclief/agent-composer/core"
	workflowexecutions "github.com/vanclief/agent-composer/core/resources/workflow/executions"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/ez"
)

const (
	toolNameWorkflowList  = "agc_workflow_list"
	toolNameWorkflowStart = "agc_workflow_start"
	toolNameWorkflowGet   = "agc_workflow_get"
)

type workflowStartArgs struct {
	Slug      string         `json:"slug" jsonschema_description:"Workflow slug from the installed AGC registry"`
	File      string         `json:"file" jsonschema_description:"Path to a workflow spec YAML file on disk"`
	Input     map[string]any `json:"input" jsonschema:"required" jsonschema_description:"Workflow input object keyed by workflow input name"`
	ShellRoot string         `json:"shell_root" jsonschema_description:"Optional shell root passed through to the workflow executor"`
}

type workflowGetArgs struct {
	ExecutionID string `json:"execution_id" jsonschema:"required" jsonschema_description:"Workflow execution id returned by agc_workflow_start"`
}

type WorkflowStartResult = workflowexecutions.CreateResponse
type WorkflowGetResult = workflowexecutions.StatusResponse

type workflowListArgs struct{}

type WorkflowListResult struct {
	Workflows []workflowruntime.WorkflowSummary `json:"workflows"`
}

func NewServer(rootCtx context.Context, stack *core.Stack, defaultShellRoot string) *server.MCPServer {
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	if stack == nil {
		panic("stack reference is nil")
	}

	srv := server.NewMCPServer("AGC MCP", "0.1.0")

	listTool := mcpproto.NewTool(
		toolNameWorkflowList,
		mcpproto.WithDescription("List installed Agent Composer workflows with their descriptions, inputs, and outputs"),
		mcpproto.WithInputSchema[workflowListArgs](),
		mcpproto.WithOutputSchema[WorkflowListResult](),
	)

	startTool := mcpproto.NewTool(
		toolNameWorkflowStart,
		mcpproto.WithDescription("Start an Agent Composer workflow by slug or YAML file path and return immediately with an execution id"),
		mcpproto.WithInputSchema[workflowStartArgs](),
		mcpproto.WithOutputSchema[WorkflowStartResult](),
	)

	getTool := mcpproto.NewTool(
		toolNameWorkflowGet,
		mcpproto.WithDescription("Get the current status, output, or failure details for an Agent Composer workflow execution"),
		mcpproto.WithInputSchema[workflowGetArgs](),
		mcpproto.WithOutputSchema[WorkflowGetResult](),
	)

	srv.AddTool(listTool, mcpproto.NewStructuredToolHandler(func(
		toolCtx context.Context,
		_ mcpproto.CallToolRequest,
		_ workflowListArgs,
	) (WorkflowListResult, error) {
		workflows, err := stack.WorkflowAPI.Registry.List(toolCtx)
		if err != nil {
			return WorkflowListResult{}, ez.Wrap(err)
		}

		return WorkflowListResult{
			Workflows: workflows,
		}, nil
	}))

	srv.AddTool(startTool, mcpproto.NewStructuredToolHandler(func(
		_ context.Context,
		_ mcpproto.CallToolRequest,
		args workflowStartArgs,
	) (WorkflowStartResult, error) {
		shellRoot := args.ShellRoot
		if shellRoot == "" {
			shellRoot = defaultShellRoot
		}

		response, err := stack.WorkflowAPI.Executions.Create(rootCtx, nil, &workflowexecutions.CreateRequest{
			WorkflowSlug: args.Slug,
			File:         args.File,
			Input:        args.Input,
			ShellRoot:    shellRoot,
		})
		if err != nil {
			return WorkflowStartResult{}, ez.Wrap(err)
		}

		return *response, nil
	}))

	srv.AddTool(getTool, mcpproto.NewStructuredToolHandler(func(
		ctx context.Context,
		_ mcpproto.CallToolRequest,
		args workflowGetArgs,
	) (WorkflowGetResult, error) {
		executionID, err := uuid.Parse(args.ExecutionID)
		if err != nil {
			return WorkflowGetResult{}, ez.New(ez.EINVALID, "invalid execution_id", err)
		}

		response, err := stack.WorkflowAPI.Executions.Status(ctx, nil, &workflowexecutions.GetRequest{
			WorkflowExecutionID: executionID,
		})
		if err != nil {
			return WorkflowGetResult{}, ez.Wrap(err)
		}

		return *response, nil
	}))

	return srv
}
