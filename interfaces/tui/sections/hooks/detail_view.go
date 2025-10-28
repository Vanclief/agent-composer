package hooks

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/models/hook"
)

type detailView struct {
	hook         *hook.Hook
	loading      bool
	err          error
	pendingID    uuid.UUID
	width        int
	height       int
	bodyViewport viewport.Model
}

func newDetailView() detailView {
	return detailView{
		bodyViewport: viewport.New(0, 0),
	}
}

func (v *detailView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.bodyViewport.Height = v.viewportHeight()
}

func (v *detailView) View() string {
	content := v.renderContent()
	width := maxInt(v.width, 40)
	style := lipgloss.NewStyle().Width(width)
	if v.height > 0 {
		style = style.MaxHeight(v.height)
	}
	return style.Render(content)
}

func (v *detailView) renderContent() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Hook Detail"))
	b.WriteString("\n\n")

	switch {
	case v.loading:
		b.WriteString(loadingStyle.Render("Loading hook detail…"))
	case v.err != nil:
		b.WriteString(errorStyle.Render("Error: " + v.err.Error()))
	case v.hook == nil:
		b.WriteString(bodyStyle.Render("No hook selected. Use esc or q to go back."))
	default:
		b.WriteString(v.renderHookLayout(v.hook))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc/q back   r refresh detail"))
	return b.String()
}

func (v *detailView) renderHookLayout(h *hook.Hook) string {
	if h == nil {
		return ""
	}

	const (
		desiredSidebarWidth = 36
		minSidebarWidth     = 24
		columnGap           = 4
		minBodyWidth        = 32
	)

	totalWidth := maxInt(v.width, desiredSidebarWidth+minBodyWidth+columnGap)
	if totalWidth <= 0 {
		totalWidth = desiredSidebarWidth + minBodyWidth + columnGap
	}

	sidebarWidth := desiredSidebarWidth
	if totalWidth < minBodyWidth+columnGap+sidebarWidth {
		sidebarWidth = totalWidth - minBodyWidth - columnGap
	}

	if sidebarWidth < minSidebarWidth {
		bodyContent := renderHookBody(h, totalWidth)
		v.prepareBodyViewport(totalWidth, bodyContent)
		sidebarContent := renderHookSidebar(h, totalWidth)
		left := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).Render(v.bodyViewport.View())
		right := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).MarginTop(1).Render(sidebarContent)
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	bodyWidth := totalWidth - sidebarWidth - columnGap
	if bodyWidth < minBodyWidth {
		bodyWidth = minBodyWidth
		sidebarWidth = totalWidth - bodyWidth - columnGap
	}

	bodyContent := renderHookBody(h, bodyWidth)
	v.prepareBodyViewport(bodyWidth, bodyContent)
	sidebarContent := renderHookSidebar(h, sidebarWidth)

	left := lipgloss.NewStyle().Width(bodyWidth).MaxWidth(bodyWidth).Render(v.bodyViewport.View())
	right := lipgloss.NewStyle().Width(sidebarWidth).MaxWidth(sidebarWidth).MarginLeft(columnGap).Render(sidebarContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *detailView) Reset() {
	v.hook = nil
	v.err = nil
	v.loading = false
	v.pendingID = uuid.UUID{}
	v.bodyViewport.SetYOffset(0)
	v.bodyViewport.SetContent("")
}

func (v *detailView) HandleMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.bodyViewport, cmd = v.bodyViewport.Update(msg)
	return cmd
}

func (v *detailView) viewportHeight() int {
	const chromePadding = 6
	minHeight := 5
	if v.height <= 0 {
		return maxInt(minHeight, 12)
	}
	available := v.height - chromePadding
	if available < minHeight {
		return minHeight
	}
	return available
}

func (v *detailView) resetViewportPosition() {
	v.bodyViewport.GotoTop()
}

func (v *detailView) prepareBodyViewport(contentWidth int, content string) {
	if contentWidth <= 0 {
		contentWidth = 40
	}
	if v.bodyViewport.Width != contentWidth {
		v.bodyViewport.Width = contentWidth
	}
	height := v.viewportHeight()
	if v.bodyViewport.Height != height {
		v.bodyViewport.Height = height
	}
	v.bodyViewport.SetContent(content)
}

func renderHookBody(h *hook.Hook, width int) string {
	if h == nil {
		return ""
	}
	if width <= 0 {
		width = 40
	}

	headerStyle := labelStyle.Copy().MaxWidth(width)
	contentStyle := bodyStyle.Copy().MaxWidth(width)

	var sections []string

	command := strings.TrimSpace(h.Command)
	if command == "" {
		command = "<no command>"
	}
	sections = append(sections, fmt.Sprintf("%s\n%s", headerStyle.Render("Command"), contentStyle.Render(wrapText(command, width))))

	argsHeader := "Arguments"
	if len(h.Args) == 0 {
		sections = append(sections, fmt.Sprintf("%s\n%s", headerStyle.Render(argsHeader), contentStyle.Render("<no args>")))
	} else {
		var lines []string
		for _, arg := range h.Args {
			text := fmt.Sprintf("- %s", strings.TrimSpace(arg))
			lines = append(lines, contentStyle.Render(wrapText(text, width)))
		}
		sections = append(sections, fmt.Sprintf("%s\n%s", headerStyle.Render(argsHeader), strings.Join(lines, "\n")))
	}

	return strings.Join(sections, "\n\n")
}

func renderHookSidebar(h *hook.Hook, width int) string {
	if h == nil {
		return ""
	}
	if width <= 0 {
		width = 24
	}

	valueStyleWrapped := valueStyle.Copy().MaxWidth(width)
	headerStyle := labelStyle.Copy().MaxWidth(width)

	var sections []string
	appendField := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		row := fmt.Sprintf("%s\n%s", headerStyle.Render(label), valueStyleWrapped.Render(value))
		sections = append(sections, row)
	}

	appendField("Event Type", string(h.EventType))
	appendField("Template", humanizeTemplate(h.AgentName))
	appendField("Status", boolLabel(h.Enabled))
	appendField("Hook ID", h.ID.String())

	return strings.Join(sections, "\n\n")
}
