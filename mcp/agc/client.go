package agc

import (
	"context"
	"io"

	"github.com/mark3labs/mcp-go/client"
	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/mcp"
	"github.com/vanclief/ez"
)

// NewClient returns an initialized MCP client backed by the in-process AGC server.
func NewClient(ctx context.Context, defaultShellRoot string) (*client.Client, error) {
	const op = "mcp.agc.NewClient"

	if ctx == nil {
		ctx = context.Background()
	}

	stack, err := core.NewStack(ctx, core.StackOptions{
		ShellRoot: defaultShellRoot,
		LogWriter: io.Discard,
	})
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	go func() {
		<-ctx.Done()
		_ = stack.Controller.DB.Close()
	}()

	srv := NewServer(ctx, stack, defaultShellRoot)

	agcClient, err := mcp.NewInProcessClient(ctx, srv)
	if err != nil {
		_ = stack.Controller.DB.Close()
		return nil, ez.Wrap(op, err)
	}

	return agcClient, nil
}
