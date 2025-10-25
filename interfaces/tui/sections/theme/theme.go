package theme

import "github.com/charmbracelet/lipgloss"

// Palette captures the primary colors used by the TUI. Leaving a field empty
// keeps the default color.
type Palette struct {
	Title     string
	Body      string
	Highlight string
	Error     string
	Loading   string
}

var (
	// DefaultPalette matches the original TUI colours.
	DefaultPalette = Palette{
		Title:     "#61afef",
		Body:      "#abb2bf",
		Highlight: "#e5c07b",
		Error:     "#e06c75",
		Loading:   "#56b6c2",
	}

	currentPalette = DefaultPalette

	TitleStyle     lipgloss.Style
	BodyStyle      lipgloss.Style
	HighlightStyle lipgloss.Style
	ErrorStyle     lipgloss.Style
	LoadingStyle   lipgloss.Style
)

func init() {
	ApplyPalette(DefaultPalette)
}

// ApplyPalette rebuilds the exported styles based on the provided palette.
func ApplyPalette(p Palette) {
	currentPalette = p

	TitleStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	if p.Title != "" {
		TitleStyle = TitleStyle.Foreground(lipgloss.Color(p.Title))
	}

	BodyStyle = lipgloss.NewStyle()
	if p.Body != "" {
		BodyStyle = BodyStyle.Foreground(lipgloss.Color(p.Body))
	}

	highlightColor := p.Highlight
	if highlightColor == "" {
		highlightColor = p.Body
	}
	HighlightStyle = BodyStyle
	if highlightColor != "" {
		HighlightStyle = HighlightStyle.Foreground(lipgloss.Color(highlightColor))
	}

	errorColor := p.Error
	if errorColor == "" {
		errorColor = DefaultPalette.Error
	}
	ErrorStyle = BodyStyle.Copy().Foreground(lipgloss.Color(errorColor))

	loadingColor := p.Loading
	if loadingColor == "" {
		loadingColor = DefaultPalette.Loading
	}
	LoadingStyle = BodyStyle.Copy().Foreground(lipgloss.Color(loadingColor))
}

// Current returns the palette currently in use.
func Current() Palette {
	return currentPalette
}
