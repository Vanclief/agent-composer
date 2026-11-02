package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vanclief/ez"

	msgclient "github.com/vanclief/agent-composer/messaging/client"
	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

const (
	// mainRole keeps one session for everything (ADR 0016 item 7).
	mainRole = "main"
	// defaultAgentIdle is the idle time when GET /agent gives none.
	defaultAgentIdle = 15 * time.Minute
	// defaultHarnessDir is the kept volume of the home sandbox (ADR 0008
	// "Disk and transcripts"): what is written there survives the VM.
	defaultHarnessDir = "/home/agent/.harness"
	// cursorFile holds the last message id the loop handled; the sessions
	// are in sessionsFile (sessions.go).
	cursorFile = "agc-cursor.json"
	// maxThreadDepth bounds following reply_to to a thread's root.
	maxThreadDepth = 100
)

// AgentRecord is one message as a persistent agent's turn sees it (ADR
// 0016 item 6).
type AgentRecord struct {
	ID        int64     `json:"id"`
	Channel   string    `json:"channel"`
	From      string    `json:"from"`
	Kind      string    `json:"kind"`
	Human     bool      `json:"human"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func newAgentRecord(m msgmodels.Message) AgentRecord {
	return AgentRecord{
		ID:        m.ID,
		Channel:   m.Channel,
		From:      m.From,
		Kind:      m.Kind,
		Human:     m.Human,
		Body:      m.Body,
		CreatedAt: m.CreatedAt,
	}
}

// RenderRecords prints records one per line as
// `@from in #channel [kind] (id): body`, a multi-line body indented under
// its first line.
func RenderRecords(records []AgentRecord) string {
	var b strings.Builder
	for _, r := range records {
		kind := r.Kind
		if kind == "" {
			kind = msgmodels.KindMessage
		}

		body := strings.ReplaceAll(strings.TrimSpace(r.Body), "\n", "\n  ")
		fmt.Fprintf(&b, "@%s in %s [%s] (%d): %s\n", r.From, r.Channel, kind, r.ID, body)
	}

	return b.String()
}

// BatchPrompt is the prompt the main role gets for a batch of new
// messages across its channels. It is the one place to change how main
// sees a batch (ADR 0015); the loop only calls it.
func BatchPrompt(records []AgentRecord) string {
	return RenderRecords(records) + "\nRead more of a channel with: agc msg read '<channel>' --after <id>\n"
}

// threadPrompt is the prompt for a turn in one thread: the new messages
// and how to read the thread from its root.
func threadPrompt(records []AgentRecord, channel string, root int64) string {
	return RenderRecords(records) + fmt.Sprintf("\nRead more with: agc msg read '%s' --after %d\n", channel, root-1)
}

// homeMessenger is the part of the messaging client the home loop uses.
type homeMessenger interface {
	Join(ctx context.Context, name string) (*msgmodels.Participant, error)
	Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error)
	Post(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error)
	AskReply(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error)
	Read(ctx context.Context, channel string, after int64, limit int) (*httpapi.MessagesResponse, error)
	WaitChannels(ctx context.Context, channels []string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error)
	Agent(ctx context.Context) (*msgclient.AgentResponse, error)
	Sleep(ctx context.Context) error
}

// homeLoop is `agc agent run --agent <name>` (ADR 0016 items 5–7): the
// loop of a persistent agent inside its home sandbox. It waits on every
// channel the agent reads, takes one turn per thread root with the new
// messages of that thread, answers in the thread, and puts the VM to sleep
// when nothing has needed a turn for the agent's idle time.
type homeLoop struct {
	harness agentHarness
	msg     homeMessenger
	name    string
	brief   string
	// dir holds the sessions and cursor files: the kept volume.
	dir string
	log io.Writer
	// prior is the participant an earlier run saved, released before
	// joining so the agent keeps its name after a hard stop.
	prior *msgState
	// joined is called with the participant once joined.
	joined func(p *msgmodels.Participant) error
	// now is the clock, swapped in tests.
	now func() time.Time
	// backoff waits after a failed wait; nil waits for real, tests record.
	backoff func(ctx context.Context, d time.Duration)

	me       *msgmodels.Participant
	role     string
	sessions *sessionStore
	// parents maps a message id to the id it replies to, 0 for none.
	parents map[int64]int64
	// heads keeps who posted each message seen (its body dropped), to
	// tell the threads the agent started.
	heads map[int64]msgmodels.Message
	// reading holds the channels the agent reads this cycle; in the
	// others it waits on (GET /agent's asked) only replies in its own
	// threads are turns.
	reading map[string]bool
	// dropped holds the channels already logged as left out of a wait.
	dropped map[string]bool
	// pending holds, per session key, other agents' plain posts that were
	// no turn, shown with the next turn in that thread.
	pending map[string][]AgentRecord
}

// homeTurn is the work of one turn: the new messages of one thread.
type homeTurn struct {
	key     string
	channel string
	root    int64
	records []AgentRecord
}

func (l *homeLoop) logf(format string, args ...any) {
	if l.log != nil {
		fmt.Fprintf(l.log, "agc agent: "+format+"\n", args...)
	}
}

func (l *homeLoop) clock() time.Time {
	if l.now != nil {
		return l.now()
	}

	return time.Now()
}

func (l *homeLoop) run(ctx context.Context) error {
	info, err := l.msg.Agent(ctx)
	if err != nil {
		return err
	}

	l.role = info.Agent.Role
	idle := parseIdle(info.Idle)

	sessions, err := loadSessionStore(l.dir)
	if err != nil {
		l.logf("%s", ez.ErrorMessage(err))
	}
	l.sessions = sessions
	l.parents = map[int64]int64{}
	l.heads = map[int64]msgmodels.Message{}
	l.dropped = map[string]bool{}

	if l.prior != nil && l.prior.Name == l.name && l.prior.ParticipantID != "" {
		// A run stopped with the VM still holds the name; it is ours.
		_, _ = l.msg.Leave(ctx, l.prior.ParticipantID)
	}

	// The home joins as the agent's name, never <name>-2: who wakes whom
	// (msgmodels.TurnFor), mentions and the manager's filters all use it.
	// Messaging hands it a name a stale participant holds (its home gone
	// or stopped); a running home's or a human's it refuses, and so does
	// this loop.
	p, err := l.msg.Join(ctx, l.name)
	if ez.ErrorCode(err) == ez.ECONFLICT {
		return ez.New(ez.ECONFLICT, "cannot join as @"+l.name+": "+ez.ErrorMessage(err), err)
	}
	if err != nil {
		return err
	}
	l.me = p
	defer l.leave()

	if l.joined != nil {
		err = l.joined(p)
		if err != nil {
			return err
		}
	}

	cursor, found, err := l.loadCursor()
	if err != nil {
		return err
	}

	// The first run ever has no cursor: answer what was said within the
	// idle time before it started (the message that woke it), not the
	// channels' whole history.
	var since time.Time
	if !found {
		since = l.clock().Add(-idle)
	}

	l.logf("joined as @%s (role %s), reading %s after %d", p.Name, l.role, strings.Join(info.Channels, ", "), cursor)
	if len(info.Asked) > 0 {
		l.logf("and the replies to its threads in %s", strings.Join(info.Asked, ", "))
	}

	lastTurn := l.clock()
	lastFailure := ""
	delay := homeRetryMin

	for {
		if ctx.Err() != nil {
			return nil
		}

		fresh, err := l.msg.Agent(ctx)
		if err != nil {
			l.logf("cannot read the agent, keeping its channels: %s", ez.ErrorMessage(err))
		} else {
			info = fresh
			idle = parseIdle(info.Idle)
		}

		quiet := l.clock().Sub(lastTurn)
		if quiet >= idle {
			return l.sleep(ctx, quiet)
		}

		channels := l.waitChannels(info)
		if len(channels) == 0 {
			l.logf("the agent reads no channel")
			return l.sleep(ctx, quiet)
		}

		timeout := min(agentWaitTimeout, idle-quiet)

		res, err := l.wait(ctx, channels, cursor, timeout)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			// Log a failure once, not on every retry, and wait longer
			// each time up to homeRetryMax.
			failure := ez.ErrorMessage(err)
			if failure != lastFailure {
				l.logf("wait failed, retrying with a back-off: %s", failure)
				lastFailure = failure
			}
			l.pauseFor(ctx, delay)
			delay = min(delay*2, homeRetryMax)
			continue
		}

		if lastFailure != "" {
			l.logf("wait works again")
			lastFailure = ""
		}
		delay = homeRetryMin

		if len(res.Messages) == 0 {
			continue
		}

		turns := l.plan(ctx, res.Messages, since)
		for _, t := range turns {
			l.take(ctx, t)
			lastTurn = l.clock()
		}

		cursor = res.Messages[len(res.Messages)-1].ID
		err = l.saveCursor(cursor)
		if err != nil {
			l.logf("cannot save the cursor: %s", ez.ErrorMessage(err))
		}
	}
}

func (l *homeLoop) sleep(ctx context.Context, quiet time.Duration) error {
	l.logf("no turn for %s, going to sleep", quiet.Round(time.Second))

	err := l.msg.Sleep(ctx)
	if err != nil {
		l.logf("cannot ask agc to stop the sandbox: %s", ez.ErrorMessage(err))
	}

	return nil
}

func (l *homeLoop) leave() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := l.msg.Leave(ctx, l.me.ID)
	if err != nil {
		l.logf("cannot leave: %s", ez.ErrorMessage(err))
	}
}

// waitChannels is the channels the agent reads, which it notes in
// reading, then the ones where it only started threads (asked).
func (l *homeLoop) waitChannels(info *msgclient.AgentResponse) []string {
	l.reading = map[string]bool{}
	channels := []string{}
	for _, channel := range info.Channels {
		l.reading[channel] = true
		channels = append(channels, channel)
	}

	for _, channel := range info.Asked {
		if l.reading[channel] {
			continue
		}

		channels = append(channels, channel)
	}

	return channels
}

// reads reports whether the agent reads channel; before the first wait
// (tests calling isTurn directly) it reads every channel.
func (l *homeLoop) reads(channel string) bool {
	return l.reading == nil || l.reading[channel]
}

// isTurn reports whether m is for the agent (msgmodels.TurnFor: a
// person's post, another agent's question or post naming it, agc's
// rejection, delivery, accept by someone else or question naming it,
// never its own) and not older than since.
func (l *homeLoop) isTurn(m msgmodels.Message, since time.Time) bool {
	if !since.IsZero() && m.CreatedAt.Before(since) {
		return false
	}

	return l.reads(m.Channel) && msgmodels.TurnFor(&m, l.me)
}

// isAnswer reports whether m, not older than since, is a reply by
// someone else in a thread the agent started, root being its first
// message: a turn in any channel (msgmodels.AnswerFor).
func (l *homeLoop) isAnswer(m msgmodels.Message, root int64, since time.Time) bool {
	if !since.IsZero() && m.CreatedAt.Before(since) {
		return false
	}

	head, ok := l.heads[root]
	if !ok {
		return false
	}

	return msgmodels.AnswerFor(&m, &head, l.me)
}

// remember keeps m's parent and who posted it.
func (l *homeLoop) remember(m msgmodels.Message) {
	l.parents[m.ID] = parentOf(m)

	m.Body = ""
	l.heads[m.ID] = m
}

// isContext reports whether m, not a turn, is another agent's plain post
// the agent is shown with the next turn in its thread.
func (l *homeLoop) isContext(m msgmodels.Message, since time.Time) bool {
	if !since.IsZero() && m.CreatedAt.Before(since) {
		return false
	}

	if m.ParticipantID == l.me.ID || m.From == l.me.Name {
		return false
	}

	return l.reads(m.Channel) && msgmodels.FromAgent(&m)
}

// plan groups the messages that are turns by thread: one turn per root,
// in the order the roots first appear, or, for main, one turn for all.
// Another agent's plain post is no turn (ADR 0016 item 6); it is kept as
// context and shown with the next turn in its thread. A reply in a thread
// the agent started is a turn in any channel (isAnswer).
func (l *homeLoop) plan(ctx context.Context, messages []msgmodels.Message, since time.Time) []homeTurn {
	if l.heads == nil {
		l.heads = map[int64]msgmodels.Message{}
	}

	for _, m := range messages {
		l.remember(m)
	}

	if l.pending == nil {
		l.pending = map[string][]AgentRecord{}
	}

	turns := []homeTurn{}
	index := map[string]int{}

	for _, m := range messages {
		turn := l.isTurn(m, since)
		if !turn && !l.isContext(m, since) && m.ReplyTo == nil {
			continue
		}

		root := l.root(ctx, m)
		if !turn {
			turn = l.isAnswer(m, root, since)
		}
		if !turn && !l.isContext(m, since) {
			continue
		}

		key := threadSessionKey(m.Channel, root)
		if l.role == mainRole {
			key = mainSessionKey
		}

		if !turn {
			l.pending[key] = append(l.pending[key], newAgentRecord(m))
			continue
		}

		i, ok := index[key]
		if !ok {
			i = len(turns)
			index[key] = i
			turns = append(turns, homeTurn{key: key})
		}

		// main answers in the thread of the newest message it was given.
		turns[i].channel = m.Channel
		turns[i].root = root
		turns[i].records = append(turns[i].records, l.pending[key]...)
		turns[i].records = append(turns[i].records, newAgentRecord(m))
		delete(l.pending, key)
	}

	return turns
}

func parentOf(m msgmodels.Message) int64 {
	if m.ReplyTo == nil {
		return 0
	}

	return *m.ReplyTo
}

// root follows reply_to from m to the message that starts its thread,
// reading a parent it has not seen from the channel (a reply is always
// in its parent's channel). A parent it cannot read is taken as the root.
func (l *homeLoop) root(ctx context.Context, m msgmodels.Message) int64 {
	id := m.ID
	for depth := 0; depth < maxThreadDepth; depth++ {
		parent, ok := l.parents[id]
		if !ok {
			parent, ok = l.fetchParent(ctx, m.Channel, id)
			if !ok {
				return id
			}
		}

		if parent == 0 {
			return id
		}

		id = parent
	}

	return id
}

func (l *homeLoop) fetchParent(ctx context.Context, channel string, id int64) (int64, bool) {
	res, err := l.msg.Read(ctx, channel, id-1, 1)
	if err != nil || len(res.Messages) == 0 || res.Messages[0].ID != id {
		return 0, false
	}

	parent := parentOf(res.Messages[0])
	l.remember(res.Messages[0])

	return parent, true
}

// take runs one turn and posts the answer as a reply to the thread's
// root. A new thread (or main's first turn ever) starts a session on the
// brief; a known one resumes. A failing harness is reported in the
// thread.
func (l *homeLoop) take(ctx context.Context, t homeTurn) {
	session := l.sessions.Get(t.key)

	var prompt string
	if l.role == mainRole {
		prompt = BatchPrompt(t.records)
		prompt += fmt.Sprintf("Your answer is posted in %s as a reply to message %d; post anywhere else with agc msg post.\n", t.channel, t.root)
	} else {
		prompt = threadPrompt(t.records, t.channel, t.root)
	}

	if session == "" {
		prompt = l.brief + "\n\n---\n\n" + prompt
	}

	l.logf("turn on %s with %d new message(s)", t.key, len(t.records))

	answer, next, err := l.harness.Run(ctx, prompt, session)
	if next != "" && next != session {
		saveErr := l.sessions.Set(t.key, next)
		if saveErr != nil {
			l.logf("cannot save the sessions: %s", ez.ErrorMessage(saveErr))
		}
	}

	body := strings.TrimSpace(answer)
	if err != nil {
		body = "The agent's harness failed: " + ez.ErrorMessage(err)
	}
	if body == "" {
		body = "(the agent answered with an empty message)"
	}
	if len(body) > msgmodels.MaxBodyBytes {
		body = body[:msgmodels.MaxBodyBytes-100] + "\n\n(cut: the answer was longer than a message allows)"
	}

	root := t.root
	question := strings.TrimSpace(strings.TrimPrefix(body, questionPrefix))
	if err == nil && strings.HasPrefix(body, questionPrefix) && question != "" {
		m, askErr := l.msg.AskReply(ctx, l.me.ID, t.channel, question, &root)
		if askErr == nil {
			l.remember(*m)
			return
		}

		l.logf("cannot ask, posting the answer: %s", ez.ErrorMessage(askErr))
	}

	m, err := l.msg.Post(ctx, l.me.ID, t.channel, body, &root)
	if err != nil {
		l.logf("cannot post the answer: %s", ez.ErrorMessage(err))
		return
	}

	l.remember(*m)
}

func parseIdle(value string) time.Duration {
	idle, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || idle <= 0 {
		return defaultAgentIdle
	}

	return idle
}

type cursorState struct {
	After int64 `json:"after"`
}

func (l *homeLoop) loadCursor() (int64, bool, error) {
	data, err := os.ReadFile(filepath.Join(l.dir, cursorFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, ez.Wrap(err)
	}

	state := cursorState{}

	err = json.Unmarshal(data, &state)
	if err != nil {
		return 0, false, ez.New(ez.EINVALID, "cursor file "+filepath.Join(l.dir, cursorFile)+" is unreadable", err)
	}

	return state.After, true, nil
}

func (l *homeLoop) saveCursor(after int64) error {
	return writeJSONFile(filepath.Join(l.dir, cursorFile), cursorState{After: after})
}

// writeJSONFile replaces path with value as JSON, through a temporary
// file so a stop mid-write leaves the old one.
func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil {
		return ez.Wrap(err)
	}

	tmp := path + ".tmp"

	err = os.WriteFile(tmp, encoded, 0o600)
	if err != nil {
		return ez.Wrap(err)
	}

	err = os.Rename(tmp, path)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// wait is WaitChannels on channels, without the channels messaging
// rejects as invalid: on an EINVALID naming one of them it leaves that
// channel out of this cycle's list and asks again, instead of failing the
// wait on every channel. Each dropped channel is logged once per run.
func (l *homeLoop) wait(ctx context.Context, channels []string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error) {
	for {
		res, err := l.msg.WaitChannels(ctx, channels, after, timeout)
		if err == nil || ctx.Err() != nil {
			return res, err
		}

		bad := invalidChannel(err, channels)
		if bad == "" {
			return nil, err
		}

		if !l.dropped[bad] {
			l.logf("leaving %s out of the wait: %s", bad, ez.ErrorMessage(err))
			l.dropped[bad] = true
		}

		channels = withoutChannel(channels, bad)
		if len(channels) == 0 {
			return nil, err
		}
	}
}

// invalidChannel is the channel of channels an EINVALID error names
// (messaging quotes it), "" for any other error.
func invalidChannel(err error, channels []string) string {
	if ez.ErrorCode(err) != ez.EINVALID {
		return ""
	}

	failure := ez.ErrorMessage(err)
	for _, channel := range channels {
		if strings.Contains(failure, strconv.Quote(channel)) {
			return channel
		}
	}

	return ""
}

func withoutChannel(channels []string, drop string) []string {
	out := []string{}
	for _, channel := range channels {
		if channel != drop {
			out = append(out, channel)
		}
	}

	return out
}

// pauseFor waits d, or until ctx is done.
func (l *homeLoop) pauseFor(ctx context.Context, d time.Duration) {
	if l.backoff != nil {
		l.backoff(ctx, d)
		return
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// pause waits agentRetryDelay or until ctx is done.
func pause(ctx context.Context) {
	timer := time.NewTimer(agentRetryDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
