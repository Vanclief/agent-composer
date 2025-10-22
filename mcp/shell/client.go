package shell

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/vanclief/ez"
	"github.com/vanclief/agent-composer/mcp"
)

// NewClient returns an initialized MCP client backed by the in-process shell server.
func NewClient(ctx context.Context, root string, allowedWorkdirs []string, defaultWorkdir string, maxTimeout time.Duration) (*client.Client, error) {
	const op = "mcp.shell.NewClient"

	if ctx == nil {
		ctx = context.Background()
	}

	srv, err := NewServer(root, allowedWorkdirs, defaultWorkdir, maxTimeout)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	shellClient, err := mcp.NewInProcessClient(ctx, srv)
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return shellClient, nil
}
