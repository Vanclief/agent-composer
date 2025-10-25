package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanclief/agent-composer/core"
	tabscomponent "github.com/vanclief/agent-composer/interfaces/tui/components/tabs"
	"github.com/vanclief/agent-composer/interfaces/tui/sections"
	agentSpecs "github.com/vanclief/agent-composer/interfaces/tui/sections/agent_specs"
	"github.com/vanclief/agent-composer/interfaces/tui/sections/conversations"
	hooksui "github.com/vanclief/agent-composer/interfaces/tui/sections/hooks"
)

const (
	minWorkspaceWidth = 40
	defaultBodyHeight = 12
)

type tabKey string

const (
	tabKeyConversations tabKey = "conversations"
	tabKeyAgentSpecs    tabKey = "agent-specs"
	tabKeyHooks         tabKey = "hooks"
)

// Model represents the root Bubble Tea model responsible for orchestrating subcomponents.
type Model struct {
	ctx            context.Context
	stack          *core.Stack
	ready          bool
	width          int
	height         int
	bodyHeight     int
	workspaceWidth int
	activeTab      tabKey

	tabs     *tabscomponent.Component
	sections map[tabKey]sections.Section
	initCmd  tea.Cmd
}

// New creates a new TUI application model.
func New(ctx context.Context, stack *core.Stack) *Model {
	m := &Model{
		ctx:      ctx,
		stack:    stack,
		sections: make(map[tabKey]sections.Section),
	}

	m.setupSections()
	m.setupTabs()
	m.initCmd = m.collectInitCmds()

	return m
}

func (m *Model) setupSections() {
	conversationsSection := conversations.New(m.ctx, m.stack)
	agentSpecsSection := agentSpecs.New(m.ctx, m.stack)
	hooksSection := hooksui.New(m.ctx, m.stack)

	m.sections[tabKeyConversations] = conversationsSection
	m.sections[tabKeyAgentSpecs] = agentSpecsSection
	m.sections[tabKeyHooks] = hooksSection
}

func (m *Model) setupTabs() {
	items := []tabscomponent.Item{
		{Key: string(tabKeyConversations), Title: "Conversations"},
		{Key: string(tabKeyAgentSpecs), Title: "Agent Specs"},
		{Key: string(tabKeyHooks), Title: "Hooks"},
	}

	m.tabs = tabscomponent.New(items)
	m.setActiveTab(tabKeyConversations)
}

func (m *Model) collectInitCmds() tea.Cmd {
	var cmds []tea.Cmd
	for _, section := range m.sections {
		if cmd := section.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.initCmd
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowSize(message)
	case tea.KeyMsg:
		if handled, cmd := m.handleKeyMsg(message); handled {
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
	}

	if _, isKey := msg.(tea.KeyMsg); isKey {
		if section := m.activeSection(); section != nil {
			if cmd := section.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	} else {
		for _, section := range m.sections {
			if cmd := section.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	if !m.ready {
		m.ready = true
	}
	m.updateLayout()
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return true, tea.Quit
	case "left", "h":
		m.moveTab(-1)
		return true, nil
	case "right", "l":
		m.moveTab(1)
		return true, nil
	}
	return false, nil
}

func (m *Model) moveTab(delta int) {
	if m.tabs == nil {
		return
	}
	key := m.tabs.Move(delta)
	if key == "" {
		return
	}
	m.setActiveTab(tabKey(key))
}

func (m *Model) setActiveTab(key tabKey) {
	if _, ok := m.sections[key]; !ok {
		return
	}
	m.activeTab = key
	if m.tabs != nil {
		m.tabs.SetActiveByKey(string(key))
	}
	m.updateSectionSizes()
}

func (m *Model) updateLayout() {
	bodyHeight := m.height - 4
	if bodyHeight < defaultBodyHeight {
		bodyHeight = defaultBodyHeight
	}
	workspaceWidth := m.width - 2
	if workspaceWidth < minWorkspaceWidth {
		workspaceWidth = minWorkspaceWidth
	}
	m.bodyHeight = bodyHeight
	m.workspaceWidth = workspaceWidth
	m.updateSectionSizes()
}

func (m *Model) updateSectionSizes() {
	if m.bodyHeight == 0 || m.workspaceWidth == 0 {
		return
	}
	for _, section := range m.sections {
		section.SetSize(m.workspaceWidth, m.bodyHeight)
	}
}

// View implements tea.Model.
func (m *Model) View() string {
	if !m.ready {
		return "Initializing Agent Composer TUI…\n"
	}

	var b strings.Builder
	if m.tabs != nil {
		b.WriteString(m.tabs.View())
		b.WriteString(m.renderHelp())
		b.WriteRune('\n')
	}
	b.WriteString(m.renderBody())
	return b.String()
}

func (m *Model) renderHelp() string {
	help := []string{"←/→ switch tabs", "ctrl+c quit"}
	if section := m.activeSection(); section != nil {
		if extra := strings.TrimSpace(section.ShortHelp()); extra != "" {
			help = append(help, extra)
		}
	}
	return helpStyle.Render(strings.Join(help, "   "))
}

func (m *Model) renderBody() string {
	content := "No section available."
	if section := m.activeSection(); section != nil {
		content = section.View()
	}
	return workspacePaneStyle.Height(m.bodyHeight).Width(m.workspaceWidth).Render(content)
}

func (m *Model) activeSection() sections.Section {
	section, ok := m.sections[m.activeTab]
	if !ok {
		return nil
	}
	return section
}
