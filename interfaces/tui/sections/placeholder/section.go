package placeholder

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanclief/agent-composer/interfaces/tui/sections/theme"
)

// Section is a minimal placeholder for unfinished tabs.
type Section struct {
	title string
}

// New creates a placeholder section with the provided title.
func New(title string) *Section {
	return &Section{title: title}
}

// Init implements sections.Section.
func (s *Section) Init() tea.Cmd { return nil }

// SetSize implements sections.Section.
func (s *Section) SetSize(_, _ int) {}

// Update implements sections.Section.
func (s *Section) Update(msg tea.Msg) tea.Cmd { return nil }

// View implements sections.Section.
func (s *Section) View() string {
	return fmt.Sprintf("%s\n\n%s", theme.TitleStyle.Render(s.title), theme.BodyStyle.Render("This section isn't implemented yet."))
}

// ShortHelp implements sections.Section.
func (s *Section) ShortHelp() string { return "" }
