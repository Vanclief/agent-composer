package agent_specs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/models/agent"
)

type detailView struct {
	spec                 *agent.Spec
	loading              bool
	err                  error
	pendingID            uuid.UUID
	width                int
	height               int
	instructionsViewport viewport.Model
}

func newDetailView() detailView {
	return detailView{
		instructionsViewport: viewport.New(0, 0),
	}
}

func (v *detailView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.instructionsViewport.Height = v.viewportHeight()
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
	b.WriteString(titleStyle.Render("Agent Spec Detail"))
	b.WriteString("\n\n")

	switch {
	case v.loading:
		b.WriteString(loadingStyle.Render("Loading agent spec detail…"))
	case v.err != nil:
		b.WriteString(errorStyle.Render("Error: " + v.err.Error()))
	case v.spec == nil:
		b.WriteString(bodyStyle.Render("No agent spec selected. Use esc or q to go back."))
	default:
		b.WriteString(v.renderSpecLayout(v.spec))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc/q back   r refresh detail"))
	return b.String()
}

func (v *detailView) renderSpecLayout(spec *agent.Spec) string {
	if spec == nil {
		return ""
	}

	const (
		desiredSidebarWidth  = 36
		minSidebarWidth      = 24
		columnGap            = 4
		minInstructionsWidth = 32
	)

	totalWidth := maxInt(v.width, desiredSidebarWidth+minInstructionsWidth+columnGap)
	if totalWidth <= 0 {
		totalWidth = desiredSidebarWidth + minInstructionsWidth + columnGap
	}

	sidebarWidth := desiredSidebarWidth
	if totalWidth < minInstructionsWidth+columnGap+sidebarWidth {
		sidebarWidth = totalWidth - minInstructionsWidth - columnGap
	}

	if sidebarWidth < minSidebarWidth {
		instructionsContent := renderSpecInstructions(spec, totalWidth)
		v.prepareInstructionsViewport(totalWidth, instructionsContent)
		sidebarContent := renderSpecSidebar(spec, totalWidth)
		left := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).Render(v.instructionsViewport.View())
		right := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).MarginTop(1).Render(sidebarContent)
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	instructionsWidth := totalWidth - sidebarWidth - columnGap
	if instructionsWidth < minInstructionsWidth {
		instructionsWidth = minInstructionsWidth
		sidebarWidth = totalWidth - instructionsWidth - columnGap
	}

	instructionsContent := renderSpecInstructions(spec, instructionsWidth)
	v.prepareInstructionsViewport(instructionsWidth, instructionsContent)
	sidebarContent := renderSpecSidebar(spec, sidebarWidth)

	left := lipgloss.NewStyle().Width(instructionsWidth).MaxWidth(instructionsWidth).Render(v.instructionsViewport.View())
	right := lipgloss.NewStyle().Width(sidebarWidth).MaxWidth(sidebarWidth).MarginLeft(columnGap).Render(sidebarContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *detailView) Reset() {
	v.spec = nil
	v.err = nil
	v.loading = false
	v.pendingID = uuid.UUID{}
	v.instructionsViewport.SetYOffset(0)
	v.instructionsViewport.SetContent("")
}

func (v *detailView) HandleMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.instructionsViewport, cmd = v.instructionsViewport.Update(msg)
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
	v.instructionsViewport.GotoTop()
}

func (v *detailView) prepareInstructionsViewport(contentWidth int, content string) {
	if contentWidth <= 0 {
		contentWidth = 40
	}
	if v.instructionsViewport.Width != contentWidth {
		v.instructionsViewport.Width = contentWidth
	}
	height := v.viewportHeight()
	if v.instructionsViewport.Height != height {
		v.instructionsViewport.Height = height
	}
	v.instructionsViewport.SetContent(content)
}

func renderSpecInstructions(spec *agent.Spec, width int) string {
	if spec == nil {
		return ""
	}
	if width <= 0 {
		width = 40
	}

	header := labelStyle.Copy().MaxWidth(width).Render("Instructions:")
	contentStyle := bodyStyle.Copy().MaxWidth(width)

	instructions := strings.TrimSpace(spec.Instructions)
	if instructions == "" {
		instructions = "<no instructions provided>"
	} else {
		instructions = wrapText(instructions, width)
	}

	return fmt.Sprintf("%s\n%s", header, contentStyle.Render(instructions))
}

func renderSpecSidebar(spec *agent.Spec, width int) string {
	if spec == nil {
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

	appendField("Name", spec.Name)
	appendField("Provider", string(spec.Provider))
	appendField("Model", spec.Model)
	appendField("Reasoning Effort", string(spec.ReasoningEffort))
	appendField("Version", fmt.Sprintf("%d", spec.Version))
	appendField("Spec ID", spec.ID.String())

	return strings.Join(sections, "\n\n")
}
