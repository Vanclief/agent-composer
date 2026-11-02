package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vanclief/ez"

	msgclient "github.com/vanclief/agent-composer/messaging/client"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

var homeT0 = time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

// homeHarness records every turn; a new session gets s<turn number>, a
// resumed one keeps its id. answers scripts turn N; failAt fails it.
type homeHarness struct {
	turns   []homeHarnessTurn
	answers map[int]string
	failAt  map[int]bool
}

type homeHarnessTurn struct {
	session string
	prompt  string
}

func (h *homeHarness) Run(ctx context.Context, prompt string, session string) (string, string, error) {
	h.turns = append(h.turns, homeHarnessTurn{session: session, prompt: prompt})
	n := len(h.turns)

	next := session
	if next == "" {
		next = fmt.Sprintf("s%d", n)
	}

	if h.failAt[n] {
		return "", next, ez.New(ez.EINTERNAL, "claude failed: out of credits", nil)
	}

	answer, ok := h.answers[n]
	if ok {
		return answer, next, nil
	}

	return fmt.Sprintf("answer %d", n), next, nil
}

type homePost struct {
	channel  string
	body     string
	replyTo  int64
	question bool
}

// homeMessenger fakes the home sandbox's socket. Wait serves the script
// one batch per call; when it runs out it cancels the loop, or, with
// idle set, advances the clock by the timeout and returns nothing.
type fakeHomeMessenger struct {
	agent    msgclient.AgentResponse
	script   [][]msgmodels.Message
	stored   map[int64]msgmodels.Message
	cancel   context.CancelFunc
	idle     bool
	clock    *time.Time
	posts    []homePost
	waits    []string
	timeouts []time.Duration
	left     []string
	slept    bool
	nextID   int64
}

func (m *fakeHomeMessenger) Join(ctx context.Context, name string) (*msgmodels.Participant, error) {
	return &msgmodels.Participant{ID: "me", Name: name}, nil
}

func (m *fakeHomeMessenger) Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error) {
	m.left = append(m.left, participantID)

	return &httpapi.LeaveResponse{ParticipantID: participantID, Left: true}, nil
}

func (m *fakeHomeMessenger) Post(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error) {
	return m.store(channel, body, replyTo, false), nil
}

func (m *fakeHomeMessenger) AskReply(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error) {
	return m.store(channel, body, replyTo, true), nil
}

func (m *fakeHomeMessenger) store(channel, body string, replyTo *int64, question bool) *msgmodels.Message {
	m.nextID++

	post := homePost{channel: channel, body: body, question: question}
	if replyTo != nil {
		post.replyTo = *replyTo
	}
	m.posts = append(m.posts, post)

	return &msgmodels.Message{ID: m.nextID, Channel: channel, ParticipantID: "me", Body: body, ReplyTo: replyTo}
}

func (m *fakeHomeMessenger) Read(ctx context.Context, channel string, after int64, limit int) (*httpapi.MessagesResponse, error) {
	found, ok := m.stored[after+1]
	if !ok || found.Channel != channel {
		return &httpapi.MessagesResponse{Messages: []msgmodels.Message{}}, nil
	}

	return &httpapi.MessagesResponse{Messages: []msgmodels.Message{found}}, nil
}

func (m *fakeHomeMessenger) WaitChannels(ctx context.Context, channels []string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error) {
	m.waits = append(m.waits, fmt.Sprintf("%s after %d", strings.Join(channels, ","), after))
	m.timeouts = append(m.timeouts, timeout)

	if len(m.script) == 0 {
		if m.idle {
			*m.clock = m.clock.Add(timeout)
			return &httpapi.MessagesResponse{Messages: []msgmodels.Message{}, Cursor: after}, nil
		}

		m.cancel()
		return nil, ez.New(ez.EUNAVAILABLE, "script is over", ctx.Err())
	}

	batch := m.script[0]
	m.script = m.script[1:]

	return &httpapi.MessagesResponse{Messages: batch}, nil
}

func (m *fakeHomeMessenger) Agent(ctx context.Context) (*msgclient.AgentResponse, error) {
	agent := m.agent

	return &agent, nil
}

func (m *fakeHomeMessenger) Sleep(ctx context.Context) error {
	m.slept = true

	return nil
}

func homeMsg(id int64, channel, from string, replyTo int64, body string) msgmodels.Message {
	m := msgmodels.Message{ID: id, Channel: channel, From: from, ParticipantID: "p-" + from, Body: body, Kind: msgmodels.KindMessage, Human: from == "franco", CreatedAt: homeT0}
	if replyTo != 0 {
		m.ReplyTo = &replyTo
	}

	return m
}

func newHomeLoop(t *testing.T, msg *fakeHomeMessenger, harness *homeHarness) (*homeLoop, *time.Time) {
	t.Helper()

	clock := homeT0
	msg.clock = &clock

	loop := &homeLoop{
		harness: harness,
		msg:     msg,
		name:    "reviewer-1",
		brief:   "# Reviewer\n",
		dir:     t.TempDir(),
		now: func() time.Time {
			return clock
		},
	}

	return loop, &clock
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		t.Fatal(err)
	}
}

func TestHomeLoopTakesOneTurnPerThread(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	own := homeMsg(13, "#b", "reviewer-1", 11, "my own note")
	own.ParticipantID = "me"
	old := homeMsg(9, "#dm", "franco", 0, "said long before the agent existed")
	old.CreatedAt = homeT0.Add(-time.Hour)

	msg := &fakeHomeMessenger{
		cancel: cancel,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm", "#b"}, Idle: "15m"},
		// 5 replies to 3, which starts the thread; neither is in a batch.
		stored: map[int64]msgmodels.Message{
			3: homeMsg(3, "#b", "alice", 0, "old root"),
			5: homeMsg(5, "#b", "bob", 3, "old reply"),
		},
		script: [][]msgmodels.Message{
			{
				old,
				homeMsg(10, "#dm", "franco", 0, "hello"),
				homeMsg(11, "#b", "alice", 0, "review my PR?"),
				homeMsg(12, "#b", "bob", 11, "line two\nof bob"),
				own,
				fromAgcIn(14, "#b", "Task fix-login created."),
				homeMsg(15, "#b", "carol", 5, "back to this"),
			},
			{
				homeMsg(20, "#b", "alice", 11, "any news?"),
				// A reply to the agent's own answer in #dm (id 101).
				homeMsg(21, "#dm", "franco", 101, "thanks"),
				fromAgcIn(22, "#b", "Rejected by franco: no tests"),
			},
		},
	}
	harness := &homeHarness{answers: map[int]string{3: "QUESTION: which commit?", 2: "DONE: reviewed"}}
	loop, _ := newHomeLoop(t, msg, harness)

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	wantPrompts := []homeHarnessTurn{
		{"", "# Reviewer\n\n\n---\n\n@franco in #dm [message] (10): hello\n\nRead more with: agc msg read '#dm' --after 9\n"},
		{"", "# Reviewer\n\n\n---\n\n@alice in #b [message] (11): review my PR?\n@bob in #b [message] (12): line two\n  of bob\n\nRead more with: agc msg read '#b' --after 10\n"},
		{"", "# Reviewer\n\n\n---\n\n@carol in #b [message] (15): back to this\n\nRead more with: agc msg read '#b' --after 2\n"},
		{"s2", "@alice in #b [message] (20): any news?\n\nRead more with: agc msg read '#b' --after 10\n"},
		{"s1", "@franco in #dm [message] (21): thanks\n\nRead more with: agc msg read '#dm' --after 9\n"},
		// agc's rejection starts no thread of its own, so it is a new one.
		{"", "# Reviewer\n\n\n---\n\n@agc in #b [message] (22): Rejected by franco: no tests\n\nRead more with: agc msg read '#b' --after 21\n"},
	}
	if !reflect.DeepEqual(harness.turns, wantPrompts) {
		t.Fatalf("turns:\n%#v", harness.turns)
	}

	// Each answer replies to its thread's root, DONE: is plain text, and
	// QUESTION: asks in the thread.
	wantPosts := []homePost{
		{channel: "#dm", body: "answer 1", replyTo: 10},
		{channel: "#b", body: "DONE: reviewed", replyTo: 11},
		{channel: "#b", body: "which commit?", replyTo: 3, question: true},
		{channel: "#b", body: "answer 4", replyTo: 11},
		{channel: "#dm", body: "answer 5", replyTo: 10},
		{channel: "#b", body: "answer 6", replyTo: 22},
	}
	if !reflect.DeepEqual(msg.posts, wantPosts) {
		t.Fatalf("posts = %+v", msg.posts)
	}

	sessions := map[string]string{}
	readJSONFile(t, filepath.Join(loop.dir, sessionsFile), &sessions)
	wantSessions := map[string]string{"#dm:10": "s1", "#b:11": "s2", "#b:3": "s3", "#b:22": "s6"}
	if !reflect.DeepEqual(sessions, wantSessions) {
		t.Fatalf("sessions = %v", sessions)
	}

	cursor := cursorState{}
	readJSONFile(t, filepath.Join(loop.dir, cursorFile), &cursor)
	if cursor.After != 22 {
		t.Fatalf("cursor = %d", cursor.After)
	}

	wantWaits := []string{"#dm,#b after 0", "#dm,#b after 15", "#dm,#b after 22"}
	if !reflect.DeepEqual(msg.waits, wantWaits) {
		t.Fatalf("waits = %q", msg.waits)
	}

	if !reflect.DeepEqual(msg.left, []string{"me"}) || msg.slept {
		t.Fatalf("left %v, slept %v", msg.left, msg.slept)
	}
}

func TestHomeLoopTurnsOnAgcPostsForTheManager(t *testing.T) {
	loop := &homeLoop{name: "reviewer-1", me: &msgmodels.Participant{ID: "me", Name: "reviewer-1"}}

	cases := map[string]bool{
		"Delivered: added tests":   true,
		"Accepted by franco.":      true,
		"Accepted by reviewer-1.":  false,
		"Rejected by franco: no":   true,
		"Task fix-login created.":  false,
		"Accepted by reviewer-10.": true,
	}
	for body, want := range cases {
		got := loop.isTurn(fromAgcIn(1, "#fix-login", body), time.Time{})
		if got != want {
			t.Errorf("isTurn(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestHomeLoopResumesKnownThreadsAfterAWake(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeHomeMessenger{
		cancel: cancel,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm"}, Idle: "15m"},
		script: [][]msgmodels.Message{
			// Older than the idle time, but after the saved cursor: kept.
			{homeMsg(31, "#dm", "franco", 10, "still there?")},
		},
	}
	msg.script[0][0].CreatedAt = homeT0.Add(-time.Hour)
	harness := &homeHarness{}
	loop, _ := newHomeLoop(t, msg, harness)
	loop.prior = &msgState{ParticipantID: "old-me", Name: "reviewer-1"}

	err := writeJSONFile(filepath.Join(loop.dir, sessionsFile), map[string]string{"#dm:10": "kept"})
	if err != nil {
		t.Fatal(err)
	}
	err = writeJSONFile(filepath.Join(loop.dir, cursorFile), cursorState{After: 30})
	if err != nil {
		t.Fatal(err)
	}

	err = loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(harness.turns) != 1 || harness.turns[0].session != "kept" || strings.Contains(harness.turns[0].prompt, "# Reviewer") {
		t.Fatalf("turns = %#v", harness.turns)
	}

	if msg.waits[0] != "#dm after 30" {
		t.Fatalf("waits = %q", msg.waits)
	}

	// The name an earlier run held is released before joining.
	if !reflect.DeepEqual(msg.left, []string{"old-me", "me"}) {
		t.Fatalf("left = %v", msg.left)
	}
}

func TestHomeLoopMainKeepsOneSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := &fakeHomeMessenger{
		cancel: cancel,
		nextID: 100,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "main-1", Role: "main"}, Channels: []string{"#a", "#b"}, Idle: "15m"},
		script: [][]msgmodels.Message{
			{homeMsg(1, "#a", "franco", 0, "one"), homeMsg(2, "#b", "alice", 0, "two")},
			{homeMsg(3, "#a", "franco", 1, "three")},
		},
	}
	harness := &homeHarness{}
	loop, _ := newHomeLoop(t, msg, harness)
	loop.name = "main-1"

	err := loop.run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	records := []AgentRecord{
		{ID: 1, Channel: "#a", From: "franco", Kind: "message", Human: true, Body: "one", CreatedAt: homeT0},
		{ID: 2, Channel: "#b", From: "alice", Kind: "message", Body: "two", CreatedAt: homeT0},
	}
	first := "# Reviewer\n\n\n---\n\n" + BatchPrompt(records) + "Your answer is posted in #b as a reply to message 2; post anywhere else with agc msg post.\n"

	if len(harness.turns) != 2 || harness.turns[0] != (homeHarnessTurn{"", first}) {
		t.Fatalf("turns = %#v", harness.turns)
	}
	if harness.turns[1].session != "s1" || strings.Contains(harness.turns[1].prompt, "# Reviewer") || !strings.Contains(harness.turns[1].prompt, "(3): three") {
		t.Fatalf("second turn = %#v", harness.turns[1])
	}

	sessions := map[string]string{}
	readJSONFile(t, filepath.Join(loop.dir, sessionsFile), &sessions)
	if !reflect.DeepEqual(sessions, map[string]string{"main": "s1"}) {
		t.Fatalf("sessions = %v", sessions)
	}

	if msg.posts[0].replyTo != 2 || msg.posts[1].replyTo != 1 || msg.posts[1].channel != "#a" {
		t.Fatalf("posts = %+v", msg.posts)
	}
}

func TestHomeLoopSleepsWhenIdleAndSurvivesAFailingTurn(t *testing.T) {
	msg := &fakeHomeMessenger{
		idle:   true,
		agent:  msgclient.AgentResponse{Agent: msgclient.AgentView{Name: "reviewer-1", Role: "reviewer"}, Channels: []string{"#dm"}, Idle: "90s"},
		script: [][]msgmodels.Message{{homeMsg(1, "#dm", "franco", 0, "hi")}},
	}
	harness := &homeHarness{failAt: map[int]bool{1: true}}
	loop, clock := newHomeLoop(t, msg, harness)

	err := loop.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(msg.posts) != 1 || msg.posts[0].replyTo != 1 || !strings.Contains(msg.posts[0].body, "The agent's harness failed: claude failed: out of credits") {
		t.Fatalf("posts = %+v", msg.posts)
	}

	// After the turn it waits out the idle time, then sleeps and exits.
	if !msg.slept || !clock.Equal(homeT0.Add(90*time.Second)) {
		t.Fatalf("slept %v at %s", msg.slept, clock)
	}
	wantTimeouts := []time.Duration{time.Minute, time.Minute, 30 * time.Second}
	if !reflect.DeepEqual(msg.timeouts, wantTimeouts) {
		t.Fatalf("timeouts = %v", msg.timeouts)
	}
}

func TestRenderRecords(t *testing.T) {
	got := BatchPrompt([]AgentRecord{
		{ID: 7, Channel: "#a", From: "franco", Kind: "question", Body: "square\nor round?"},
		{ID: 9, Channel: "#b", From: "bob", Body: "hi"},
	})

	want := "@franco in #a [question] (7): square\n  or round?\n@bob in #b [message] (9): hi\n\nRead more of a channel with: agc msg read '<channel>' --after <id>\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

// fromAgcIn is a message agc itself posted in channel.
func fromAgcIn(id int64, channel string, body string) msgmodels.Message {
	m := homeMsg(id, channel, "agc", 0, body)
	m.ParticipantID = "p-agc"

	return m
}
