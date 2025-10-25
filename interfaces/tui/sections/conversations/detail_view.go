package conversations

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/models/agent"
	runtimetypes "github.com/vanclief/agent-composer/runtime/types"
)

type detailView struct {
	conversation     *agent.Conversation
	loading          bool
	err              error
	pendingID        uuid.UUID
	width            int
	height           int
	messagesViewport viewport.Model
}

func newDetailView() detailView {
	return detailView{
		messagesViewport: viewport.New(0, 0),
	}
}

func (v *detailView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.messagesViewport.Height = v.viewportHeight()
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
	b.WriteString(titleStyle.Render("Conversation Detail"))
	b.WriteString("\n\n")

	switch {
	case v.loading:
		b.WriteString(loadingStyle.Render("Loading conversation details…"))
	case v.err != nil:
		b.WriteString(errorStyle.Render("Error: " + v.err.Error()))
	case v.conversation == nil:
		b.WriteString(bodyStyle.Render("No conversation selected. Use esc or q to go back."))
	default:
		b.WriteString(v.renderConversationLayout(v.conversation))
	}

	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render("esc/q back   r refresh detail"))
	return b.String()
}

func (v *detailView) renderConversationLayout(conv *agent.Conversation) string {
	if conv == nil {
		return ""
	}

	const (
		desiredSidebarWidth = 36
		minSidebarWidth     = 24
		columnGap           = 4
		minMessagesWidth    = 32
	)

	totalWidth := maxInt(v.width, desiredSidebarWidth+minMessagesWidth+columnGap)
	if totalWidth <= 0 {
		totalWidth = desiredSidebarWidth + minMessagesWidth + columnGap
	}

	sidebarWidth := desiredSidebarWidth
	if totalWidth < minMessagesWidth+columnGap+sidebarWidth {
		sidebarWidth = totalWidth - minMessagesWidth - columnGap
	}

	if sidebarWidth < minSidebarWidth {
		messagesContent := renderConversationMessages(conv, totalWidth)
		v.prepareMessagesViewport(totalWidth, messagesContent)
		sidebarContent := renderConversationSidebar(conv, totalWidth)
		left := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).Render(v.messagesViewport.View())
		right := lipgloss.NewStyle().Width(totalWidth).MaxWidth(totalWidth).MarginTop(1).Render(sidebarContent)
		return lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	messagesWidth := totalWidth - sidebarWidth - columnGap
	if messagesWidth < minMessagesWidth {
		messagesWidth = minMessagesWidth
		sidebarWidth = totalWidth - messagesWidth - columnGap
	}

	messagesContent := renderConversationMessages(conv, messagesWidth)
	v.prepareMessagesViewport(messagesWidth, messagesContent)
	sidebarContent := renderConversationSidebar(conv, sidebarWidth)

	left := lipgloss.NewStyle().Width(messagesWidth).MaxWidth(messagesWidth).Render(v.messagesViewport.View())
	right := lipgloss.NewStyle().Width(sidebarWidth).MaxWidth(sidebarWidth).MarginLeft(columnGap).Render(sidebarContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *detailView) Reset() {
	v.conversation = nil
	v.err = nil
	v.loading = false
	v.pendingID = uuid.UUID{}
	v.messagesViewport.SetYOffset(0)
	v.messagesViewport.SetContent("")
}

func (v *detailView) HandleMsg(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.messagesViewport, cmd = v.messagesViewport.Update(msg)
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
	v.messagesViewport.GotoTop()
}

func (v *detailView) prepareMessagesViewport(contentWidth int, content string) {
	if contentWidth <= 0 {
		contentWidth = 40
	}
	if v.messagesViewport.Width != contentWidth {
		v.messagesViewport.Width = contentWidth
	}
	height := v.viewportHeight()
	if v.messagesViewport.Height != height {
		v.messagesViewport.Height = height
	}
	v.messagesViewport.SetContent(content)
}

func renderConversationMessages(conv *agent.Conversation, width int) string {
	if width <= 0 {
		width = 40
	}
	contentStyle := bodyStyle.Copy().MaxWidth(width)
	headerStyle := valueStyle.Copy().Bold(true).MaxWidth(width)

	if conv == nil || len(conv.Messages) == 0 {
		return contentStyle.Render("No messages recorded yet.")
	}

	var b strings.Builder
	for idx, message := range conv.Messages {
		if idx > 0 {
			b.WriteString("\n")
		}
		role := humanizeRole(string(message.Role))
		content, alreadyStyled := formatMessageContent(message, width)
		header := fmt.Sprintf("%s:", role)
		b.WriteString(headerStyle.Render(header))
		b.WriteString("\n")
		if alreadyStyled {
			b.WriteString(content)
		} else {
			b.WriteString(contentStyle.Render(content))
		}
		b.WriteString("\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func renderConversationSidebar(conv *agent.Conversation, width int) string {
	if conv == nil {
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

	appendField("Name", conv.Name)
	appendField("Status", string(conv.Status))
	appendField("Provider", string(conv.Provider))
	appendField("Model", conv.Model)
	appendField("Reasoning", string(conv.ReasoningEffort))
	appendField("Spec ID", conv.AgentSpecID.String())
	appendField("Conversation ID", conv.ID.String())

	return strings.Join(sections, "\n\n")
}

type toolMessagePayload struct {
	ExitCode     int    `json:"exit_code"`
	DurationMS   int64  `json:"duration_ms"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	TimedOut     bool   `json:"timed_out"`
	EffectiveDir string `json:"effective_dir"`
	CommandEcho  string `json:"command_echo"`
}

func formatMessageContent(message runtimetypes.Message, width int) (string, bool) {
	if message.Role == runtimetypes.MessageRoleTool {
		if formatted, ok := formatToolMessageContent(message, width); ok {
			return formatted, true
		}
	}
	return wrapText(strings.TrimSpace(message.Content), width), false
}

func formatToolMessageContent(message runtimetypes.Message, width int) (string, bool) {
	var payload toolMessagePayload
	if err := json.Unmarshal([]byte(message.Content), &payload); err != nil {
		return "", false
	}

	var b strings.Builder

	command := strings.TrimSpace(payload.CommandEcho)
	if command == "" && message.Name != "" {
		command = message.Name
	}
	if command != "" {
		b.WriteString(valueStyle.Copy().Bold(true).Render("Ran"))
		b.WriteString("\n")
		b.WriteString(bodyStyle.Render(wrapText(command, width)))
	}

	outputHeader := "Stdout:"
	outputStyle := bodyStyle
	output := strings.TrimSpace(payload.Stdout)

	if payload.ExitCode != 0 || payload.TimedOut || strings.TrimSpace(payload.Stderr) != "" {
		outputHeader = "Stderr:"
		outputStyle = errorStyle
		if strings.TrimSpace(payload.Stderr) != "" {
			output = strings.TrimSpace(payload.Stderr)
		} else if output == "" {
			output = strings.TrimSpace(payload.Stdout)
		}
	}

	if output == "" {
		output = "<no output>"
	}

	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(valueStyle.Copy().Bold(true).Render(outputHeader))
	b.WriteString("\n")
	b.WriteString(outputStyle.Render(wrapText(output, width)))

	if payload.ExitCode != 0 || payload.TimedOut {
		var statusParts []string
		statusParts = append(statusParts, fmt.Sprintf("exit code %d", payload.ExitCode))
		if payload.TimedOut {
			statusParts = append(statusParts, "timed out")
		}
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(strings.Join(statusParts, ", ")))
	}

	return b.String(), true
}
