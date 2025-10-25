package conversations

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/core"
	"github.com/vanclief/agent-composer/core/resources/agents/conversations"
	"github.com/vanclief/agent-composer/models/agent"
)

var apiTimeout = 10 * time.Second

type viewMode int

const (
	viewModeList viewMode = iota
	viewModeDetail
)

// Section owns the conversations workspace state.
type Section struct {
	ctx    context.Context
	stack  *core.Stack
	width  int
	height int
	mode   viewMode

	list   listView
	detail detailView
}

// New creates a conversations section.
func New(ctx context.Context, stack *core.Stack) *Section {
	return &Section{
		ctx:    ctx,
		stack:  stack,
		mode:   viewModeList,
		list:   newListView(),
		detail: newDetailView(),
	}
}

// Init implements sections.Section.
func (s *Section) Init() tea.Cmd {
	return s.loadConversationList(true)
}

// SetSize implements sections.Section.
func (s *Section) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetSize(width, height)
	s.detail.SetSize(width, height)
}

// Update implements sections.Section.
func (s *Section) Update(msg tea.Msg) tea.Cmd {
	switch message := msg.(type) {
	case tea.KeyMsg:
		return s.handleKeyMsg(message)
	case tea.MouseMsg:
		if s.mode == viewModeDetail {
			return s.detail.HandleMsg(message)
		}
	case conversationListLoadedMsg:
		s.handleConversationListLoaded(message)
	case conversationDetailLoadedMsg:
		s.handleConversationDetailLoaded(message)
	}
	return nil
}

// View implements sections.Section.
func (s *Section) View() string {
	switch s.mode {
	case viewModeDetail:
		return s.detail.View()
	default:
		return s.list.View()
	}
}

// ShortHelp implements sections.Section.
func (s *Section) ShortHelp() string {
	if s.mode == viewModeDetail {
		return "esc/q back   r reload detail"
	}
	return "enter open conversation   n/p pagination   r refresh"
}

func (s *Section) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch s.mode {
	case viewModeDetail:
		return s.handleDetailKeys(msg)
	default:
		return s.handleListKeys(msg)
	}
}

func (s *Section) handleListKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return s.openConversationFromSelection()
	case "esc":
		return nil
	case "r":
		return s.loadConversationList(true)
	case "n":
		return s.nextConversationPage()
	case "p":
		return s.previousConversationPage()
	}
	return s.list.HandleTableKey(msg)
}

func (s *Section) handleDetailKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q":
		s.returnToList()
		return nil
	case "r":
		return s.reloadDetail()
	}
	return s.detail.HandleMsg(msg)
}

func (s *Section) openConversationFromSelection() tea.Cmd {
	conv, ok := s.list.SelectedConversation()
	if !ok {
		return nil
	}
	id := conv.ID
	return s.showConversationDetail(id, true)
}

func (s *Section) reloadDetail() tea.Cmd {
	var id uuid.UUID
	switch {
	case s.detail.pendingID != (uuid.UUID{}):
		id = s.detail.pendingID
	case s.detail.conversation != nil:
		id = s.detail.conversation.ID
	default:
		return nil
	}
	return s.showConversationDetail(id, false)
}

func (s *Section) showConversationDetail(id uuid.UUID, switchView bool) tea.Cmd {
	if switchView {
		s.mode = viewModeDetail
	}
	s.detail.loading = true
	s.detail.err = nil
	s.detail.pendingID = id
	if switchView {
		s.detail.conversation = nil
	}
	return s.loadConversationDetail(id)
}

func (s *Section) returnToList() {
	s.detail.Reset()
	s.mode = viewModeList
}

func (s *Section) loadConversationList(resetCursor bool) tea.Cmd {
	if s.stack == nil || s.stack.AgentsAPI == nil || s.stack.AgentsAPI.Conversations == nil {
		s.list.err = fmt.Errorf("conversations API unavailable")
		return nil
	}

	if resetCursor {
		s.list.request.Cursor = ""
		s.list.prevCursors = nil
	}

	req := s.list.request
	s.list.loading = true
	s.list.err = nil
	s.list.status = "Loading conversations…"
	s.list.pendingCursor = req.Cursor

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(s.ctx, apiTimeout)
		defer cancel()

		resp, err := s.stack.AgentsAPI.Conversations.List(ctx, nil, &req)
		return conversationListLoadedMsg{cursor: req.Cursor, response: resp, err: err}
	}
}

func (s *Section) loadConversationDetail(id uuid.UUID) tea.Cmd {
	if s.stack == nil || s.stack.AgentsAPI == nil || s.stack.AgentsAPI.Conversations == nil {
		s.detail.loading = false
		s.detail.err = fmt.Errorf("conversations API unavailable")
		return nil
	}

	req := conversations.GetRequest{ConversationID: id}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(s.ctx, apiTimeout)
		defer cancel()

		resp, err := s.stack.AgentsAPI.Conversations.Get(ctx, nil, &req)
		return conversationDetailLoadedMsg{id: id, conversation: resp, err: err}
	}
}

func (s *Section) nextConversationPage() tea.Cmd {
	resp := s.list.response
	if resp == nil || !resp.HasNextPage || resp.NextCursor == "" {
		s.list.status = "Already at the last page."
		return nil
	}

	s.list.prevCursors = append(s.list.prevCursors, s.list.request.Cursor)
	s.list.request.Cursor = resp.NextCursor
	return s.loadConversationList(false)
}

func (s *Section) previousConversationPage() tea.Cmd {
	if len(s.list.prevCursors) == 0 {
		s.list.status = "Already at the first page."
		return nil
	}

	idx := len(s.list.prevCursors) - 1
	cursor := s.list.prevCursors[idx]
	s.list.prevCursors = s.list.prevCursors[:idx]
	s.list.request.Cursor = cursor
	return s.loadConversationList(false)
}

func (s *Section) handleConversationListLoaded(msg conversationListLoadedMsg) {
	if msg.cursor != s.list.pendingCursor {
		return
	}

	s.list.loading = false
	if msg.err != nil {
		s.list.err = msg.err
		s.list.status = "Unable to load conversations."
		return
	}

	s.list.err = nil
	s.list.response = msg.response
	if msg.response != nil {
		s.list.items = msg.response.Conversations
	} else {
		s.list.items = nil
	}

	s.list.UpdateRows(s.list.items)

	if len(s.list.items) == 0 {
		s.list.status = "No conversations matched your filters."
	} else {
		s.list.status = fmt.Sprintf("Loaded %d conversation(s).", len(s.list.items))
	}
}

func (s *Section) handleConversationDetailLoaded(msg conversationDetailLoadedMsg) {
	if msg.id != s.detail.pendingID {
		return
	}

	s.detail.loading = false
	if msg.err != nil {
		s.detail.err = msg.err
		return
	}

	s.detail.err = nil
	s.detail.conversation = msg.conversation
	s.detail.resetViewportPosition()
}

type conversationListLoadedMsg struct {
	cursor   string
	response *conversations.ListResponse
	err      error
}

type conversationDetailLoadedMsg struct {
	id           uuid.UUID
	conversation *agent.Conversation
	err          error
}
