package hooks

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/vanclief/agent-composer/core"
	hooksresource "github.com/vanclief/agent-composer/core/resources/hooks"
	"github.com/vanclief/agent-composer/models/hook"
)

var apiTimeout = 10 * time.Second

type viewMode int

const (
	viewModeList viewMode = iota
	viewModeDetail
)

// Section owns the hooks workspace state.
type Section struct {
	ctx    context.Context
	stack  *core.Stack
	width  int
	height int
	mode   viewMode

	list   listView
	detail detailView
}

// New creates a hooks section.
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
	return s.loadHookList(true)
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
	case hookListLoadedMsg:
		s.handleHookListLoaded(message)
	case hookDetailLoadedMsg:
		s.handleHookDetailLoaded(message)
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
	return "enter open hook   n/p pagination   r refresh"
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
		return s.openHookFromSelection()
	case "esc":
		return nil
	case "r":
		return s.loadHookList(true)
	case "n":
		return s.nextHookPage()
	case "p":
		return s.previousHookPage()
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

func (s *Section) openHookFromSelection() tea.Cmd {
	h, ok := s.list.SelectedHook()
	if !ok {
		return nil
	}
	id := h.ID
	return s.showHookDetail(id, true)
}

func (s *Section) reloadDetail() tea.Cmd {
	var id uuid.UUID
	switch {
	case s.detail.pendingID != (uuid.UUID{}):
		id = s.detail.pendingID
	case s.detail.hook != nil:
		id = s.detail.hook.ID
	default:
		return nil
	}
	return s.showHookDetail(id, false)
}

func (s *Section) showHookDetail(id uuid.UUID, switchView bool) tea.Cmd {
	if switchView {
		s.mode = viewModeDetail
	}
	s.detail.loading = true
	s.detail.err = nil
	s.detail.pendingID = id
	if switchView {
		s.detail.hook = nil
	}
	return s.loadHookDetail(id)
}

func (s *Section) returnToList() {
	s.detail.Reset()
	s.mode = viewModeList
}

func (s *Section) loadHookList(resetCursor bool) tea.Cmd {
	if s.stack == nil || s.stack.HooksAPI == nil {
		s.list.err = fmt.Errorf("hooks API unavailable")
		return nil
	}

	if resetCursor {
		s.list.request.Cursor = ""
		s.list.prevCursors = nil
	}

	req := s.list.request
	s.list.loading = true
	s.list.err = nil
	s.list.status = "Loading hooks…"
	s.list.pendingCursor = req.Cursor

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(s.ctx, apiTimeout)
		defer cancel()

		resp, err := s.stack.HooksAPI.List(ctx, nil, &req)
		return hookListLoadedMsg{cursor: req.Cursor, response: resp, err: err}
	}
}

func (s *Section) loadHookDetail(id uuid.UUID) tea.Cmd {
	if s.stack == nil || s.stack.HooksAPI == nil {
		s.detail.loading = false
		s.detail.err = fmt.Errorf("hooks API unavailable")
		return nil
	}

	req := hooksresource.GetRequest{HookID: id}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(s.ctx, apiTimeout)
		defer cancel()

		resp, err := s.stack.HooksAPI.Get(ctx, nil, &req)
		return hookDetailLoadedMsg{id: id, hook: resp, err: err}
	}
}

func (s *Section) nextHookPage() tea.Cmd {
	resp := s.list.response
	if resp == nil || !resp.HasNextPage || resp.NextCursor == "" {
		s.list.status = "Already at the last page."
		return nil
	}

	s.list.prevCursors = append(s.list.prevCursors, s.list.request.Cursor)
	s.list.request.Cursor = resp.NextCursor
	return s.loadHookList(false)
}

func (s *Section) previousHookPage() tea.Cmd {
	if len(s.list.prevCursors) == 0 {
		s.list.status = "Already at the first page."
		return nil
	}

	idx := len(s.list.prevCursors) - 1
	cursor := s.list.prevCursors[idx]
	s.list.prevCursors = s.list.prevCursors[:idx]
	s.list.request.Cursor = cursor
	return s.loadHookList(false)
}

func (s *Section) handleHookListLoaded(msg hookListLoadedMsg) {
	if msg.cursor != s.list.pendingCursor {
		return
	}

	s.list.loading = false
	if msg.err != nil {
		s.list.err = msg.err
		s.list.status = "Unable to load hooks."
		return
	}

	s.list.err = nil
	s.list.response = msg.response
	if msg.response != nil {
		s.list.items = msg.response.Hooks
	} else {
		s.list.items = nil
	}

	s.list.UpdateRows(s.list.items)

	if len(s.list.items) == 0 {
		s.list.status = "No hooks found."
	} else {
		s.list.status = fmt.Sprintf("Loaded %d hook(s).", len(s.list.items))
	}
}

func (s *Section) handleHookDetailLoaded(msg hookDetailLoadedMsg) {
	if msg.id != s.detail.pendingID {
		return
	}

	s.detail.loading = false
	if msg.err != nil {
		s.detail.err = msg.err
		return
	}

	s.detail.err = nil
	s.detail.hook = msg.hook
	s.detail.resetViewportPosition()
}

type hookListLoadedMsg struct {
	cursor   string
	response *hooksresource.ListResponse
	err      error
}

type hookDetailLoadedMsg struct {
	id   uuid.UUID
	hook *hook.Hook
	err  error
}
