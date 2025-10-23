package mcp

import (
	"context"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

func (client *Client) ListTools(ctx context.Context) ([]runtimetypes.ToolDefinition, error) {
	const op = "mcp.ListTools"

	listToolsResult, err := client.c.ListTools(ctx, mcpproto.ListToolsRequest{})
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	toolDefinitions := make([]runtimetypes.ToolDefinition, 0, len(listToolsResult.Tools))
	for _, tool := range listToolsResult.Tools {
		jsonSchema, err := extractToolSchema(tool)
		if err != nil {
			return nil, ez.Wrap(op, err)
		}

		toolDefinitions = append(toolDefinitions, runtimetypes.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			JSONSchema:  jsonSchema,
		})
	}
	return toolDefinitions, nil
}

func (client *Client) CallTool(ctx context.Context, toolCall *runtimetypes.ToolCall) (string, error) {
	const op = "mcp.CallTool"

	request := mcpproto.CallToolRequest{
		Params: mcpproto.CallToolParams{
			Name:      toolCall.Name,
			Arguments: toolCall.JSONArguments,
		},
	}

	res, err := client.c.CallTool(ctx, request)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	result := stringifyResult(res)

	return result, nil
}
