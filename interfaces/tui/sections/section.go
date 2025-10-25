package sections

import tea "github.com/charmbracelet/bubbletea"

// Section defines the behavior shared by every workspace section.
type Section interface {
	Init() tea.Cmd
	SetSize(width, height int)
	Update(msg tea.Msg) tea.Cmd
	View() string
	ShortHelp() string
}
