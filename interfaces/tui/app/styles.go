package app

import "github.com/charmbracelet/lipgloss"

var (
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	workspacePaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(1, 2).
				BorderForeground(lipgloss.Color("240"))
)
