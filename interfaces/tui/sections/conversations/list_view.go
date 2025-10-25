package conversations

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanclief/agent-composer/core/resources/agents/conversations"
	"github.com/vanclief/agent-composer/interfaces/tui/sections/theme"
	"github.com/vanclief/agent-composer/models/agent"
)

type listView struct {
	table         table.Model
	items         []agent.Conversation
	response      *conversations.ListResponse
	request       conversations.ListRequest
	prevCursors   []string
	loading       bool
	err           error
	status        string
	pendingCursor string
	width         int
	height        int
}

var (
	selectedBackground = lipgloss.Color("#3e4451")
	selectedForeground = lipgloss.Color("#f0f0f0")
)

func newListView() listView {
	tbl := table.New(
		table.WithColumns(buildConversationColumns(120)),
		table.WithRows([]table.Row{}),
		table.WithHeight(12),
		table.WithWidth(120),
	)

	styles := table.DefaultStyles()
	styles.Header = theme.HighlightStyle.Copy().Bold(true)
	styles.Cell = theme.BodyStyle.Copy()
	styles.Selected = theme.BodyStyle.Copy().
		Foreground(selectedForeground).
		Background(selectedBackground)
	tbl.SetStyles(styles)
	tbl.Focus()

	req := conversations.ListRequest{}
	req.CursorRequest.Limit = conversationsPageSize

	return listView{
		table:   tbl,
		request: req,
		status:  "Loading conversations…",
	}
}

func (v *listView) SetSize(width, height int) {
	v.width = width
	v.height = height
	tableHeight := maxInt(height-6, 6)
	tableWidth := maxInt(width-2, 40)
	v.table.SetHeight(tableHeight)
	v.table.SetWidth(tableWidth)
	v.table.SetColumns(buildConversationColumns(tableWidth))
}

func (v *listView) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Conversations"))
	b.WriteString("\n\n")

	if v.loading {
		b.WriteString(loadingStyle.Render("Loading conversations…"))
		b.WriteString("\n\n")
	}

	if v.err != nil {
		b.WriteString(errorStyle.Render("Error: " + v.err.Error()))
		b.WriteString("\n\n")
	}

	b.WriteString(v.table.View())
	b.WriteString("\n\n")

	if status := strings.TrimSpace(v.status); status != "" {
		b.WriteString(statusStyle.Render(status))
		b.WriteString("\n")
	}

	b.WriteString(statusStyle.Render(v.statusLine()))
	return b.String()
}

func (v *listView) statusLine() string {
	parts := []string{fmt.Sprintf("%d row(s)", len(v.items))}
	if len(v.prevCursors) > 0 {
		parts = append(parts, "p previous page")
	}
	if resp := v.response; resp != nil && resp.HasNextPage {
		parts = append(parts, "n next page")
	}
	parts = append(parts, "r refresh", "enter view conversation")
	return strings.Join(parts, "  •  ")
}

func (v *listView) HandleTableKey(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

func (v *listView) SelectedConversation() (*agent.Conversation, bool) {
	if len(v.items) == 0 {
		return nil, false
	}
	cursor := v.table.Cursor()
	if cursor < 0 || cursor >= len(v.items) {
		return nil, false
	}
	return &v.items[cursor], true
}

func (v *listView) UpdateRows(items []agent.Conversation) {
	v.items = items
	rows := make([]table.Row, len(items))
	for i, conv := range items {
		rows[i] = table.Row{conv.Name, string(conv.Status), string(conv.Provider), conv.Model, conv.ID.String()}
	}
	v.table.SetRows(rows)
	if len(rows) > 0 {
		v.table.SetCursor(0)
		v.table.GotoTop()
	}
}
