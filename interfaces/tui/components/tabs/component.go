package tabs

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Item represents a single tab entry.
type Item struct {
	Key   string
	Title string
}

// Component renders the tab strip and tracks the active tab.
type Component struct {
	items  []Item
	active int
}

var (
	activeTabStyle   = lipgloss.NewStyle().Padding(0, 2).Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Bold(true)
	inactiveTabStyle = lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("246"))
	tabDividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("239")).SetString("│")
)

// New creates a new tab component.
func New(items []Item) *Component {
	c := &Component{items: items, active: 0}
	if len(items) == 0 {
		c.active = -1
	}
	return c
}

// SetActiveByKey selects the tab matching the key.
func (c *Component) SetActiveByKey(key string) bool {
	for idx, item := range c.items {
		if item.Key == key {
			c.active = idx
			return true
		}
	}
	return false
}

// Move updates the active tab by delta and returns the new tab key.
func (c *Component) Move(delta int) string {
	if len(c.items) == 0 {
		return ""
	}
	count := len(c.items)
	c.active = (c.active + delta + count) % count
	return c.items[c.active].Key
}

// ActiveItem returns the currently selected tab item.
func (c *Component) ActiveItem() (Item, bool) {
	if c.active < 0 || c.active >= len(c.items) {
		return Item{}, false
	}
	return c.items[c.active], true
}

// View renders the tab strip.
func (c *Component) View() string {
	if len(c.items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" ")
	for idx, item := range c.items {
		style := inactiveTabStyle
		if idx == c.active {
			style = activeTabStyle
		}
		b.WriteString(style.Render(item.Title))
		if idx < len(c.items)-1 {
			b.WriteString(tabDividerStyle.Render(" │ "))
		}
	}
	b.WriteString(" ")
	return b.String()
}
