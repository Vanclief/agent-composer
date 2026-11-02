package cli

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// fakeHarness records every turn and answers "answer N" on session s1.
type fakeHarness struct {
	turns []string
}

func (h *fakeHarness) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	h.turns = append(h.turns, session+"|"+prompt)

	return fmt.Sprintf("answer %d", len(h.turns)), "s1", nil
}

// fakeMessenger serves scripted batches to Wait, one per call, and records
// joins, posts, waits and leaves. When the script runs out it cancels the
// loop's context.
type fakeMessenger struct {
	script  [][]msgmodels.Message
	cancel  context.CancelFunc
	taken   map[string]bool
	joined  []string
	posts   []string
	waits   []int64
	left    bool
	nextID  int64
	lastIDs map[string]int64
}

func (m *fakeMessenger) Join(ctx context.Context, name string) (*msgmodels.Participant, error) {
	m.joined = append(m.joined, name)
	if m.taken[name] {
		return nil, ez.New(ez.ECONFLICT, "name @"+name+" is already in use", nil)
	}

	return &msgmodels.Participant{ID: "me", Name: name, Task: "fix-login"}, nil
}

func (m *fakeMessenger) Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error) {
	m.left = true

	return &httpapi.LeaveResponse{ParticipantID: participantID, Left: true}, nil
}

func (m *fakeMessenger) Post(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error) {
	m.nextID++
	m.posts = append(m.posts, channel+" "+participantID+": "+body)

	return &msgmodels.Message{ID: m.nextID, Channel: channel, ParticipantID: participantID, Body: body}, nil
}

func (m *fakeMessenger) Wait(ctx context.Context, channel string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error) {
	m.waits = append(m.waits, after)

	if len(m.script) == 0 {
		m.cancel()
		return nil, ez.New(ez.EUNAVAILABLE, "script is over", ctx.Err())
	}

	batch := m.script[0]
	m.script = m.script[1:]

	return &httpapi.MessagesResponse{Messages: batch}, nil
}

func (m *fakeMessenger) Channels(ctx context.Context) (*httpapi.ChannelsResponse, error) {
	channels := []messaging.ChannelInfo{}
	for channel, lastID := range m.lastIDs {
		channels = append(channels, messaging.ChannelInfo{Channel: channel, LastID: lastID})
	}

	return &httpapi.ChannelsResponse{Channels: channels}, nil
}

func message(id int64, from string, participantID string, body string) msgmodels.Message {
	return msgmodels.Message{ID: id, Channel: "#fix-login", From: from, ParticipantID: participantID, Body: body}
}

func TestAgentLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeMessenger{
		cancel:  cancel,
		taken:   map[string]bool{"fix-login": true},
		nextID:  2,
		lastIDs: map[string]int64{"#fix-login": 2, "#franco": 9},
		script: [][]msgmodels.Message{
			// Its own first answer, then the manager's reply.
			{message(3, "fix-login-2", "me", "answer 1"), message(4, "orchestrator", "p-orch", "Please also add tests.")},
			// Its own answer, /done from someone who may not close the
			// task, then /done from the owner.
			{message(5, "fix-login-2", "me", "answer 2"), message(6, "tester", "p-test", "/done"), message(7, "franco", "p-franco", " /done\n")},
			{message(8, "orchestrator", "p-orch", "never read")},
		},
	}
	harness := &fakeHarness{}

	var joined *msgmodels.Participant
	loop := &agentLoop{
		harness: harness,
		msg:     msg,
		task:    "fix-login",
		name:    "fix-login",
		brief:   "# Fix login\n",
		closers: map[string]bool{"orchestrator": true, "franco": true},
		joined: func(p *msgmodels.Participant) error {
			joined = p
			return nil
		},
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// An earlier run still holds the sandbox's name.
	if !reflect.DeepEqual(msg.joined, []string{"fix-login", "fix-login-2"}) || joined == nil || joined.Name != "fix-login-2" {
		t.Fatalf("joined %q as %+v", msg.joined, joined)
	}

	wantTurns := []string{
		"|# Fix login\n",
		"s1|@orchestrator in #fix-login:\n\nPlease also add tests.",
		"s1|@tester in #fix-login:\n\n/done",
		"s1|@franco closed the task with /done. Write a short closing summary for #fix-login: what you changed, what you verified, where the work is (branch and commits), and what is left undone.",
	}
	if !reflect.DeepEqual(harness.turns, wantTurns) {
		t.Fatalf("turns:\n%s", strings.Join(harness.turns, "\n---\n"))
	}

	wantPosts := []string{
		"#fix-login me: answer 1",
		"#fix-login me: answer 2",
		"#fix-login me: answer 3",
		"#fix-login me: answer 4",
	}
	if !reflect.DeepEqual(msg.posts, wantPosts) {
		t.Fatalf("posts = %q", msg.posts)
	}

	// It waits after the channel's last message, then after each batch,
	// and stops at /done without reading the third batch.
	if !reflect.DeepEqual(msg.waits, []int64{2, 4}) || len(msg.script) != 1 {
		t.Fatalf("waits = %v, script left %d", msg.waits, len(msg.script))
	}

	if !msg.left {
		t.Fatal("the loop did not leave")
	}
}

func TestAgentLoopAnswersHarnessFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeMessenger{cancel: cancel}
	loop := &agentLoop{
		harness: failingHarness{},
		msg:     msg,
		task:    "fix-login",
		name:    "fix-login",
		brief:   "# Fix login\n",
	}

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(msg.posts) != 1 || msg.posts[0] != "#fix-login me: The agent's harness failed: claude failed: not logged in" {
		t.Fatalf("posts = %q", msg.posts)
	}
}

type failingHarness struct{}

func (failingHarness) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	return "", "", ez.New(ez.EINTERNAL, "claude failed: not logged in", nil)
}
