package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	"github.com/vanclief/agent-composer/runtime/llm"
	"github.com/vanclief/ez"
)

type Mux struct {
	clients      []*client.Client
	toolToClient map[string]int
	mergedTools  []llm.ToolDefinition
}

// NewMux starts initialized clients list (already started/initialized) and builds an index.
func NewMux(ctx context.Context, clients ...*client.Client) (*Mux, error) {
	const op = "mcp.NewMux"

	if len(clients) == 0 {
		return nil, ez.New(op, ez.EINVALID, "no MCP clients provided", nil)
	}

	mux := &Mux{
		clients:      clients,
		toolToClient: make(map[string]int),
	}

	err := mux.refreshTools(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return mux, nil
}

func (m *Mux) refreshTools(ctx context.Context) error {
	const op = "mcp.Mux.refreshTools"

	merged := make([]llm.ToolDefinition, 0, 16)
	toolToClient := make(map[string]int)

	for clientIndex, mc := range m.clients {
		result, err := mc.ListTools(ctx, mcpproto.ListToolsRequest{})
		if err != nil {
			return ez.Wrap(op, err)
		}
		for _, tool := range result.Tools {
			// Convert mcp.Tool -> llm.ToolDefinition
			var schemaMap map[string]any
			schemaBytes, marshalErr := json.Marshal(tool.InputSchema)
			if marshalErr != nil {
				return ez.Wrap(op, marshalErr)
			}
			unmarshalErr := json.Unmarshal(schemaBytes, &schemaMap)
			if unmarshalErr != nil {
				return ez.Wrap(op, unmarshalErr)
			}

			converted := llm.ToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				JSONSchema:  schemaMap,
			}

			// First writer wins (if duplicate tool names collide)
			_, exists := toolToClient[tool.Name]
			if !exists {
				toolToClient[tool.Name] = clientIndex
				merged = append(merged, converted)
			}
		}
	}

	m.mergedTools = merged
	m.toolToClient = toolToClient
	return nil
}

// ListTools surfaces merged tools in llm.ToolDefinition form.
func (m *Mux) ListTools(_ context.Context) ([]llm.ToolDefinition, error) {
	return m.mergedTools, nil
}

// CallTool routes a call by tool name to the owning MCP client and returns a text payload for your LLM transcript.
func (m *Mux) CallTool(ctx context.Context, call *llm.ToolCall) (string, error) {
	const op = "mcp.Mux.CallTool"

	if call == nil {
		return "", ez.New(op, ez.EINVALID, "nil tool call", nil)
	}

	clientIndex, exists := m.toolToClient[call.Name]
	if !exists {
		return "", ez.New(op, ez.ENOTFOUND, fmt.Sprintf("unknown tool: %s", call.Name), nil)
	}

	var argsMap map[string]any
	if len(call.Arguments) > 0 {
		unmarshalErr := json.Unmarshal([]byte(call.Arguments), &argsMap)
		if unmarshalErr != nil {
			return "", ez.Wrap(op, unmarshalErr)
		}
	}

	request := mcpproto.CallToolRequest{
		Params: mcpproto.CallToolParams{
			Name:      call.Name,
			Arguments: argsMap,
		},
	}

	result, err := m.clients[clientIndex].CallTool(ctx, request)
	if err != nil {
		return "", ez.Wrap(op, err)
	}

	// Prefer text content; fall back to structured.
	var combined string
	for i := range result.Content {
		textContent, ok := mcpproto.AsTextContent(result.Content[i])
		if ok {
			if len(combined) > 0 {
				combined += "\n"
			}
			combined += textContent.Text
		}
	}
	if len(combined) > 0 {
		return combined, nil
	}

	if result.StructuredContent != nil {
		bytesOut, marshalErr := json.Marshal(result.StructuredContent)
		if marshalErr != nil {
			return "", ez.Wrap(op, marshalErr)
		}
		return string(bytesOut), nil
	}

	// Nothing useful returned; still succeed with empty payload.
	return "", nil
}
