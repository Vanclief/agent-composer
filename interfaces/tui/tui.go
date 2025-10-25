package tui

import (
	"context"
	"fmt"

	"github.com/vanclief/agent-composer/core"
)

// Start launches the terminal UI.
func Start(ctx context.Context, stack *core.Stack) error {
	fmt.Println("Terminal UI not implemented yet. Press Ctrl+C to exit.")
	<-ctx.Done()
	return ctx.Err()
}
