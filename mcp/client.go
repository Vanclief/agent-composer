package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/client"
	mcclient "github.com/mark3labs/mcp-go/client"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vanclief/ez"
)

type Client struct {
	c *mcclient.Client
}

func (client *Client) Close() {
	if client.c != nil {
		client.c.Close()
	}
}

const protocolVersion = "2025-06-18" // MCP spec revision (client will negotiate) :contentReference[oaicite:0]{index=0}

// NewInProcessClient connects an in-process MCP server directly to a stdio subprocess.
func NewInProcessClient(ctx context.Context, srv *server.MCPServer) (*client.Client, error) {
	const op = "mcp.NewInProcessClient"

	if srv == nil {
		return nil, ez.New(op, ez.EINVALID, "nil MCP server", nil)
	}

	mcpClient, err := client.NewInProcessClient(srv)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	initReq := mcpproto.InitializeRequest{
		Params: mcpproto.InitializeParams{
			ProtocolVersion: protocolVersion,
			ClientInfo: mcpproto.Implementation{
				Name:    "agent-composer",
				Version: "0.1.0",
			},
			Capabilities: mcpproto.ClientCapabilities{},
		},
	}
	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}
	return mcpClient, nil
}

func StartStdioClient(ctx context.Context, command string, env []string, args ...string) (*client.Client, error) {
	const op = "mcp.StartStdioClient"

	mcpClient, err := client.NewStdioMCPClient(command, env, args...)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	err = mcpClient.Start(ctx)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	initReq := mcpproto.InitializeRequest{
		Params: mcpproto.InitializeParams{
			ProtocolVersion: protocolVersion,
			ClientInfo: mcpproto.Implementation{
				Name:    "agent-composer",
				Version: "0.1.0",
			},
			Capabilities: mcpproto.ClientCapabilities{},
		},
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return mcpClient, nil
}
