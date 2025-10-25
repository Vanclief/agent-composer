package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/interfaces/tui/app"
)

// Start launches the terminal UI using Bubble Tea.
func Start(ctx context.Context, stack *core.Stack) error {
	program := tea.NewProgram(
		app.New(ctx, stack),
		tea.WithContext(ctx),
		tea.WithAltScreen(),
	)

	_, err := program.Run()
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tea.ErrProgramKilled) {
		return err
	}

	return nil
}
