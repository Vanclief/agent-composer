package agc

import (
	"context"
	"testing"

	mcpproto "github.com/mark3labs/mcp-go/mcp"
)

func TestNewClientListsAgcTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := NewClient(ctx, "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	result, err := client.ListTools(context.Background(), mcpproto.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	if len(result.Tools) != 3 {
		t.Fatalf("unexpected tool count: %d", len(result.Tools))
	}

	toolNames := map[string]bool{}
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	if !toolNames[toolNameWorkflowList] {
		t.Fatalf("missing tool: %q", toolNameWorkflowList)
	}

	if !toolNames[toolNameWorkflowStart] {
		t.Fatalf("missing tool: %q", toolNameWorkflowStart)
	}

	if !toolNames[toolNameWorkflowGet] {
		t.Fatalf("missing tool: %q", toolNameWorkflowGet)
	}
}
