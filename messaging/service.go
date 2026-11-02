// Package messaging implements the agc message space (ADR 0005):
// participants join with a name, post to channels, and read or wait
// from a cursor they own.
package messaging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/events"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

const (
	defaultReadLimit = 50
	maxReadLimit     = 500
	defaultWait      = 30 * time.Second
)

// Grant is a channel listed in the Task spec.
type Grant struct {
	Channel  string `json:"channel"`
	ReadOnly bool   `json:"read_only"`
}

// Caller is who is asking, resolved by the transport, never by the
// request body.
type Caller struct {
	Task   string
	Grants []Grant
	Human  bool
}

// ChannelInfo is a channel the caller may read.
type ChannelInfo struct {
	Channel  string `json:"channel"`
	ReadOnly bool   `json:"read_only"`
	LastID   int64  `json:"last_id"`
	Count    int    `json:"count"`
}

// PostHook runs after every message is stored, with the stored message.
type PostHook func(ctx context.Context, m *msgmodels.Message)

type Service struct {
	db bun.IDB

	// posted is closed and replaced on every post, waking all waiters.
	mu     sync.Mutex
	posted chan struct{}
	hooks  []PostHook

	// presence answers whether an agent's sandbox runs, nil when unwired.
	presence Presence

	// channelsFor names the channels a person reads besides their inbox.
	channelsFor ChannelsFor

	// system serializes taking the reserved name agc.
	system sync.Mutex

	// seen is when each participant last read, waited or posted over
	// HTTP, in memory only.
	seenMu sync.Mutex
	seen   map[string]time.Time

	// now is the clock, swapped in tests.
	now func() time.Time

	// events gets message.posted for every stored message, nil drops them.
	events *events.Bus
}

// PostedEvent is the data of message.posted.
type PostedEvent struct {
	Message *msgmodels.Message `json:"message"`
}

// SetEvents publishes message.posted on bus for every message stored from
// now on, before the post hooks run.
func (s *Service) SetEvents(bus *events.Bus) {
	s.mu.Lock()
	s.events = bus
	s.mu.Unlock()
}

func NewService(db bun.IDB) *Service {
	return &Service{db: db, posted: make(chan struct{}), seen: map[string]time.Time{}, now: time.Now}
}

// TaskChannel is the channel every Task gets without declaring it.
func TaskChannel(task string) string {
	return "#" + task
}

// authorize is the single access rule. Humans may do anything. A Task
// may read and post in its own channel, read any granted channel, and
// post in a granted channel unless it is read-only.
func authorize(caller Caller, channel string, post bool) error {
	if caller.Human {
		return nil
	}

	if caller.Task != "" && channel == TaskChannel(caller.Task) {
		return nil
	}

	for _, grant := range caller.Grants {
		if grant.Channel != channel {
			continue
		}

		if post && grant.ReadOnly {
			msg := fmt.Sprintf("channel %s is read-only for task %s", channel, caller.Task)
			return ez.New(ez.ENOTAUTHORIZED, msg, nil)
		}

		return nil
	}

	msg := fmt.Sprintf("channel %s is not available to task %s", channel, caller.Task)
	return ez.New(ez.ENOTAUTHORIZED, msg, nil)
}

func checkCaller(caller Caller) error {
	if !caller.Human && caller.Task == "" {
		return ez.New(ez.ENOTAUTHORIZED, "caller is neither a task nor a human", nil)
	}

	return nil
}

// owns reports whether the participant was joined by this caller.
func owns(caller Caller, p *msgmodels.Participant) bool {
	return p.Human == caller.Human && p.Task == caller.Task
}

func (s *Service) Join(ctx context.Context, caller Caller, name string) (*msgmodels.Participant, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = msgmodels.ValidateName(name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if name == msgmodels.SystemName {
		return nil, ez.New(ez.ECONFLICT, fmt.Sprintf("name @%s is reserved for agc itself", name), nil)
	}

	p := &msgmodels.Participant{Name: name, Human: caller.Human}
	if !caller.Human {
		p.Task = caller.Task
	}

	err = p.Insert(ctx, s.db)
	if err != nil {
		return nil, s.joinError(ctx, name, err)
	}

	return p, nil
}

// joinError tells a taken name from any other insert failure. The
// unique index on active_name is the arbiter, so look again after it
// refused rather than checking before inserting.
func (s *Service) joinError(ctx context.Context, name string, cause error) error {
	taken, err := s.db.NewSelect().Model((*msgmodels.Participant)(nil)).Where("active_name = ?", name).Exists(ctx)
	if err == nil && taken {
		return ez.New(ez.ECONFLICT, fmt.Sprintf("name @%s is already in use", name), cause)
	}

	return ez.Wrap(cause)
}

func (s *Service) participant(ctx context.Context, id string) (*msgmodels.Participant, error) {
	p := &msgmodels.Participant{}

	err := s.db.NewSelect().Model(p).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("participant %s not found", id), err)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return p, nil
}

// Leave releases the participant's name. Only the caller that joined
// may leave, except a human, who may release any name. Leaving twice
// is a no-op.
func (s *Service) Leave(ctx context.Context, caller Caller, participantID string) error {
	err := checkCaller(caller)
	if err != nil {
		return ez.Wrap(err)
	}

	p, err := s.participant(ctx, participantID)
	if err != nil {
		return ez.Wrap(err)
	}

	if !caller.Human && !owns(caller, p) {
		return ez.New(ez.ENOTAUTHORIZED, fmt.Sprintf("participant %s does not belong to task %s", participantID, caller.Task), nil)
	}

	if isSystem(p) {
		return ez.New(ez.ENOTAUTHORIZED, fmt.Sprintf("participant @%s is agc itself and cannot leave", p.Name), nil)
	}

	if p.LeftAt != nil {
		return nil
	}

	err = p.Leave(ctx, s.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Post appends a message of kind, message or question; empty is message.
// Sender fields come from the participant row, never from the input.
func (s *Service) Post(ctx context.Context, caller Caller, participantID, channel, body string, replyTo *int64, kind string) (*msgmodels.Message, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if kind == "" {
		kind = msgmodels.KindMessage
	}

	err = msgmodels.ValidateKind(kind)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = msgmodels.ValidateChannel(channel)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = authorize(caller, channel, true)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	p, err := s.participant(ctx, participantID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if !owns(caller, p) {
		return nil, ez.New(ez.ENOTAUTHORIZED, fmt.Sprintf("participant %s does not belong to this caller", participantID), nil)
	}

	if p.LeftAt != nil {
		return nil, ez.New(ez.EINVALID, fmt.Sprintf("participant @%s has left, join again", p.Name), nil)
	}

	if replyTo != nil {
		exists, err := s.db.NewSelect().Model((*msgmodels.Message)(nil)).Where("id = ?", *replyTo).Where("channel = ?", channel).Exists(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if !exists {
			return nil, ez.New(ez.EINVALID, fmt.Sprintf("reply_to %d is not a message in %s", *replyTo, channel), nil)
		}
	}

	return s.insert(ctx, p, channel, body, replyTo, kind)
}

// insert stores a message from p, wakes the waiters and runs the hooks.
func (s *Service) insert(ctx context.Context, p *msgmodels.Participant, channel, body string, replyTo *int64, kind string) (*msgmodels.Message, error) {
	m := &msgmodels.Message{
		Channel:       channel,
		From:          p.Name,
		ParticipantID: p.ID,
		Task:          p.Task,
		Human:         p.Human,
		ReplyTo:       replyTo,
		Body:          body,
		Kind:          kind,
	}

	err := m.Insert(ctx, s.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	s.mu.Lock()
	close(s.posted)
	s.posted = make(chan struct{})
	hooks := s.hooks
	bus := s.events
	s.mu.Unlock()

	bus.Publish(events.Event{Type: events.MessagePosted, Data: PostedEvent{Message: m}})

	for _, hook := range hooks {
		hook(ctx, m)
	}

	return m, nil
}

// OnPost registers hook to run after every stored message, in the
// poster's request, before the post returns.
func (s *Service) OnPost(hook PostHook) {
	s.mu.Lock()
	s.hooks = append(s.hooks, hook)
	s.mu.Unlock()
}

// isSystem reports whether p is agc's own participant.
func isSystem(p *msgmodels.Participant) bool {
	return p.Name == msgmodels.SystemName && !p.Human && p.Task == ""
}

// PostAsSystem posts body in channel as the reserved participant agc
// (ADR 0009): task empty, human false. agc keeps the name for good, so no
// one else can join with it.
func (s *Service) PostAsSystem(ctx context.Context, channel, body string) (*msgmodels.Message, error) {
	err := msgmodels.ValidateChannel(channel)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	p, err := s.systemParticipant(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return s.insert(ctx, p, channel, body, nil, msgmodels.KindMessage)
}

// systemParticipant returns agc's active participant, joining it the first
// time.
func (s *Service) systemParticipant(ctx context.Context) (*msgmodels.Participant, error) {
	s.system.Lock()
	defer s.system.Unlock()

	p := &msgmodels.Participant{}

	err := s.db.NewSelect().Model(p).Where("active_name = ?", msgmodels.SystemName).Scan(ctx)
	if err == nil && isSystem(p) {
		return p, nil
	}
	if err == nil {
		msg := fmt.Sprintf("name @%s is held by participant %s, not by agc: release it with agc msg leave", p.Name, p.ID)
		return nil, ez.New(ez.ECONFLICT, msg, nil)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, ez.Wrap(err)
	}

	p = &msgmodels.Participant{Name: msgmodels.SystemName}

	err = p.Insert(ctx, s.db)
	if err != nil {
		return nil, s.joinError(ctx, p.Name, err)
	}

	return p, nil
}

// Read returns messages with id > afterID, oldest first.
func (s *Service) Read(ctx context.Context, caller Caller, channel string, afterID int64, limit int) ([]msgmodels.Message, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = msgmodels.ValidateChannel(channel)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = authorize(caller, channel, false)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return s.read(ctx, channel, afterID, limit)
}

func (s *Service) read(ctx context.Context, channel string, afterID int64, limit int) ([]msgmodels.Message, error) {
	if limit <= 0 {
		limit = defaultReadLimit
	}

	if limit > maxReadLimit {
		limit = maxReadLimit
	}

	messages := []msgmodels.Message{}

	err := s.db.NewSelect().
		Model(&messages).
		Where("channel = ?", channel).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return messages, nil
}

// Wait is Read that blocks until a message with id > afterID exists or
// the timeout passes, in which case it returns an empty slice. Every
// Post wakes all waiters, which then query again.
func (s *Service) Wait(ctx context.Context, caller Caller, channel string, afterID int64, timeout time.Duration) ([]msgmodels.Message, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = msgmodels.ValidateChannel(channel)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = authorize(caller, channel, false)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if timeout <= 0 {
		timeout = defaultWait
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		// Take the signal before querying so a post in between is not lost.
		s.mu.Lock()
		posted := s.posted
		s.mu.Unlock()

		messages, err := s.read(ctx, channel, afterID, maxReadLimit)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if len(messages) > 0 {
			return messages, nil
		}

		select {
		case <-posted:
		case <-timer.C:
			return []msgmodels.Message{}, nil
		case <-ctx.Done():
			return nil, ez.Wrap(ctx.Err())
		}
	}
}

// Channels lists the channels the caller may read. A Task sees its own
// channel and its grants, even before anyone posts. A human sees every
// channel that has messages.
func (s *Service) Channels(ctx context.Context, caller Caller) ([]ChannelInfo, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	var stats []ChannelInfo

	query := s.db.NewSelect().
		Model((*msgmodels.Message)(nil)).
		ColumnExpr("channel, MAX(id) AS last_id, COUNT(*) AS count").
		Group("channel").
		Order("channel ASC")

	if caller.Human {
		err = query.Scan(ctx, &stats)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if stats == nil {
			stats = []ChannelInfo{}
		}

		return stats, nil
	}

	channels := []ChannelInfo{{Channel: TaskChannel(caller.Task)}}
	names := []string{TaskChannel(caller.Task)}
	for _, grant := range caller.Grants {
		if grant.Channel == TaskChannel(caller.Task) {
			continue
		}

		channels = append(channels, ChannelInfo{Channel: grant.Channel, ReadOnly: grant.ReadOnly})
		names = append(names, grant.Channel)
	}

	err = query.Where("channel IN (?)", bun.In(names)).Scan(ctx, &stats)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for i := range channels {
		for _, stat := range stats {
			if stat.Channel == channels[i].Channel {
				channels[i].LastID = stat.LastID
				channels[i].Count = stat.Count
			}
		}
	}

	return channels, nil
}

// Participants lists everyone who has posted in channel: active ones
// first, then those who left, each by name, with their presence. Access
// is the same as Read.
func (s *Service) Participants(ctx context.Context, caller Caller, channel string) ([]ParticipantView, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = msgmodels.ValidateChannel(channel)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = authorize(caller, channel, false)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	posters := s.db.NewSelect().
		Model((*msgmodels.Message)(nil)).
		Column("participant_id").
		Where("channel = ?", channel)

	participants := []msgmodels.Participant{}

	err = s.db.NewSelect().
		Model(&participants).
		Where("id IN (?)", posters).
		OrderExpr("left_at IS NOT NULL").
		Order("name ASC", "joined_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return s.views(ctx, participants)
}

// AllParticipants lists everyone who ever joined, in any channel or none:
// active ones first, then those who left, each by name, with their
// presence. Only humans may list them; an agent sees the participants of
// the channels it may read.
func (s *Service) AllParticipants(ctx context.Context, caller Caller) ([]ParticipantView, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if !caller.Human {
		msg := fmt.Sprintf("listing every participant is for humans, not task %s", caller.Task)
		return nil, ez.New(ez.ENOTAUTHORIZED, msg, nil)
	}

	participants := []msgmodels.Participant{}

	err = s.db.NewSelect().
		Model(&participants).
		OrderExpr("left_at IS NOT NULL").
		Order("name ASC", "joined_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return s.views(ctx, participants)
}
