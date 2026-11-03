// Package messaging implements the agc message space (ADR 0005):
// participants join with a name, post to channels, and read or wait
// from a cursor they own.
package messaging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
// request body. Agent is set instead of Task for a persistent agent's home
// sandbox (ADR 0016): the home sandbox's name, which is the agent's first
// name and never changes.
type Caller struct {
	Task   string
	Agent  string
	Grants []Grant
	Human  bool
}

// MainAgent reports whether the persistent agent whose home sandbox is
// called home has the role main, which may read, wait on and post in
// every channel (ADR 0016 item 9). Core wires it with SetMainAgent so
// messaging does not import the agents API.
type MainAgent func(home string) bool

// ChannelInfo is a channel the caller may read.
type ChannelInfo struct {
	Channel  string `json:"channel"`
	ReadOnly bool   `json:"read_only"`
	LastID   int64  `json:"last_id"`
	Count    int    `json:"count"`
	// ArchivedAt is when the channel was archived (ADR 0005 "Archived
	// channels"); only archived channels have it, and they are listed only
	// when asked for.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
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

	// mainAgent says which agent callers read everything, nil for none.
	mainAgent MainAgent

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

	// imagesDir keeps messages' images, empty refuses them.
	imagesDir string
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

// SetMainAgent wires the check for the main role (ADR 0016 item 9).
// Without it no agent caller is main.
func (s *Service) SetMainAgent(fn MainAgent) {
	s.mu.Lock()
	s.mainAgent = fn
	s.mu.Unlock()
}

// everywhere reports whether the caller may use every channel: a human, or
// the persistent agent with the role main.
func (s *Service) everywhere(caller Caller) bool {
	if caller.Human {
		return true
	}

	if caller.Agent == "" {
		return false
	}

	s.mu.Lock()
	fn := s.mainAgent
	s.mu.Unlock()

	return fn != nil && fn(caller.Agent)
}

// TaskChannel is the channel every Task gets without declaring it.
func TaskChannel(task string) string {
	return "#" + task
}

// authorize is the single access rule. Humans, and the agent with the
// role main (ADR 0016 item 9), may do anything. A Task may read and post
// in its own channel, read any granted channel, and post in a granted
// channel unless it is read-only. Any other persistent agent has only its
// grants, which its home's transport computes per request.
func (s *Service) authorize(caller Caller, channel string, post bool) error {
	if s.everywhere(caller) {
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
			msg := fmt.Sprintf("channel %s is read-only for %s", channel, callerName(caller))
			return ez.New(ez.ENOTAUTHORIZED, msg, nil)
		}

		return nil
	}

	msg := fmt.Sprintf("channel %s is not available to %s", channel, callerName(caller))
	return ez.New(ez.ENOTAUTHORIZED, msg, nil)
}

// callerName names a non-human caller in errors.
func callerName(caller Caller) string {
	if caller.Agent != "" {
		return "agent " + caller.Agent
	}

	return "task " + caller.Task
}

func checkCaller(caller Caller) error {
	if !caller.Human && caller.Task == "" && caller.Agent == "" {
		return ez.New(ez.ENOTAUTHORIZED, "caller is neither a task, an agent nor a human", nil)
	}

	if !caller.Human && caller.Task != "" && caller.Agent != "" {
		return ez.New(ez.ENOTAUTHORIZED, "caller is both a task and an agent", nil)
	}

	return nil
}

// ownChannels is the caller's own channel, #<task>, for a task; none for
// a human or an agent.
func ownChannels(caller Caller) []string {
	if caller.Task == "" {
		return []string{}
	}

	return []string{TaskChannel(caller.Task)}
}

// owns reports whether the participant was joined by this caller.
func owns(caller Caller, p *msgmodels.Participant) bool {
	return p.Human == caller.Human && p.Task == caller.Task && p.Agent == caller.Agent
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
		p.Agent = caller.Agent
	}

	err = p.Insert(ctx, s.db)
	if err == nil {
		return p, nil
	}

	joinErr := s.joinError(ctx, name, err)
	if caller.Human || caller.Agent == "" || ez.ErrorCode(joinErr) != ez.ECONFLICT {
		return nil, joinErr
	}

	// A persistent agent's home joins as the agent's name whatever the
	// home is called (ADR 0016 item 6), so it takes the name over from a
	// participant left behind by a home that is gone or stopped.
	took, err := s.takeStale(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	if !took {
		return nil, joinErr
	}

	err = p.Insert(ctx, s.db)
	if err != nil {
		return nil, s.joinError(ctx, name, err)
	}

	return p, nil
}

// takeStale releases name from the active participant holding it when
// that participant is an agent presence says is absent: its sandbox is
// gone or not running. A human's name, agc's and a running agent's are
// never taken; without presence nothing is.
func (s *Service) takeStale(ctx context.Context, name string) (bool, error) {
	s.mu.Lock()
	presence := s.presence
	s.mu.Unlock()

	if presence == nil {
		return false, nil
	}

	holder := &msgmodels.Participant{}

	err := s.db.NewSelect().Model(holder).Where("active_name = ?", name).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, ez.Wrap(err)
	}

	if holder.Human || isSystem(holder) {
		return false, nil
	}

	now := s.now()
	if s.online(ctx, presence, holder, now, now) {
		return false, nil
	}

	err = holder.Leave(ctx, s.db)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return true, nil
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
		return ez.New(ez.ENOTAUTHORIZED, fmt.Sprintf("participant %s does not belong to %s", participantID, callerName(caller)), nil)
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
	return s.PostImages(ctx, caller, participantID, channel, body, replyTo, kind, nil)
}

// PostImages is Post with images (ADR 0005 "Images"): at most four, each a
// PNG, JPEG, GIF or WebP of at most 5 MB, kept under the images directory.
// A message with images may have an empty body.
func (s *Service) PostImages(ctx context.Context, caller Caller, participantID, channel, body string, replyTo *int64, kind string, uploads []msgmodels.ImageUpload) (*msgmodels.Message, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	pending, err := s.prepareImages(uploads)
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

	err = s.authorize(caller, channel, true)
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

	err = s.inboxPost(ctx, caller, p, channel, replyTo, kind)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if p.LeftAt != nil {
		return nil, ez.New(ez.EINVALID, fmt.Sprintf("participant @%s has left, join again", p.Name), nil)
	}

	if !isSystem(p) {
		archived, err := msgmodels.ChannelArchived(ctx, s.db, channel)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if archived {
			return nil, ez.New(ez.ENOTAUTHORIZED, "channel archived", nil)
		}
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

	return s.insert(ctx, p, channel, body, replyTo, kind, pending)
}

// inboxPost is the inbox rule (ADR 0005 "Inbox"): in an inbox #<name>, a
// channel named after a human participant or a persistent agent (ADR
// 0016), a caller other than a person or that agent may post only a
// question or a reply in a thread. Notes go in #<task>, so the inbox holds
// only what needs its holder. A person's posts and agc's (PostAsSystem)
// never come here.
func (s *Service) inboxPost(ctx context.Context, caller Caller, p *msgmodels.Participant, channel string, replyTo *int64, kind string) error {
	if caller.Human {
		return nil
	}

	if p.Agent != "" && channel == "#"+p.Name {
		return nil
	}

	if kind == msgmodels.KindQuestion || replyTo != nil {
		return nil
	}

	inbox, err := s.isInbox(ctx, channel)
	if err != nil {
		return ez.Wrap(err)
	}

	if !inbox {
		return nil
	}

	return ez.New(ez.ENOTAUTHORIZED, "only questions and answers go in a person's inbox; post notes in #<task>", nil)
}

// isInbox reports whether channel is #<name> for a human participant or a
// persistent agent's (agent set), present or departed. A task's
// participant has task set and agent empty, so #<task> is never an inbox.
func (s *Service) isInbox(ctx context.Context, channel string) (bool, error) {
	name, found := strings.CutPrefix(channel, "#")
	if !found {
		return false, nil
	}

	exists, err := s.db.NewSelect().Model((*msgmodels.Participant)(nil)).Where("name = ?", name).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("human = ?", true).WhereOr("agent <> ''")
		}).
		Exists(ctx)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return exists, nil
}

// insert stores a message from p with its images, wakes the waiters and
// runs the hooks. The image files are written before anyone hears of the
// message, so a reader can always fetch them.
func (s *Service) insert(ctx context.Context, p *msgmodels.Participant, channel, body string, replyTo *int64, kind string, pending []pendingImage) (*msgmodels.Message, error) {
	m := &msgmodels.Message{
		Channel:       channel,
		From:          p.Name,
		ParticipantID: p.ID,
		Task:          p.Task,
		Agent:         p.Agent,
		Human:         p.Human,
		ReplyTo:       replyTo,
		Body:          body,
		Kind:          kind,
	}

	for _, image := range pending {
		m.Images = append(m.Images, image.image)
	}

	err := m.Insert(ctx, s.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	m.FillImageURLs()

	err = s.writeImages(m.ID, pending)
	if err != nil {
		_, _ = s.db.NewDelete().Model((*msgmodels.Message)(nil)).Where("id = ?", m.ID).Exec(ctx)
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

// ArchivedEvent is the data of channel.archived.
type ArchivedEvent struct {
	Channel    string    `json:"channel"`
	ArchivedAt time.Time `json:"archived_at"`
}

// Archive archives channel at at (ADR 0005 "Archived channels") and
// publishes channel.archived. It answers false, publishing nothing, when
// the channel was already archived. Nothing un-archives a channel.
func (s *Service) Archive(ctx context.Context, channel string, at time.Time) (bool, error) {
	added, err := msgmodels.ArchiveChannel(ctx, s.db, channel, at)
	if err != nil {
		return false, ez.Wrap(err)
	}

	if !added {
		return false, nil
	}

	s.mu.Lock()
	bus := s.events
	s.mu.Unlock()

	bus.Publish(events.Event{Type: events.ChannelArchived, Data: ArchivedEvent{Channel: channel, ArchivedAt: at.UTC()}})

	return true, nil
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
	return p.Name == msgmodels.SystemName && !p.Human && p.Task == "" && p.Agent == ""
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

	return s.insert(ctx, p, channel, body, nil, msgmodels.KindMessage, nil)
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
	return s.ReadChannels(ctx, caller, []string{channel}, afterID, limit)
}

// ReadChannels is Read over several channels at once: messages from any of
// them with id > afterID, in id order. Message ids are global, so one
// cursor serves every channel. The caller must be allowed to read each.
func (s *Service) ReadChannels(ctx context.Context, caller Caller, channels []string, afterID int64, limit int) ([]msgmodels.Message, error) {
	channels, err := s.checkRead(caller, channels)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return s.read(ctx, channels, afterID, limit)
}

// checkRead validates channels, drops repeats and authorizes reading each.
func (s *Service) checkRead(caller Caller, channels []string) ([]string, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(channels) == 0 {
		return nil, ez.New(ez.EINVALID, "name at least one channel", nil)
	}

	seen := map[string]bool{}
	unique := []string{}
	for _, channel := range channels {
		err = msgmodels.ValidateChannel(channel)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		err = s.authorize(caller, channel, false)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if seen[channel] {
			continue
		}

		seen[channel] = true
		unique = append(unique, channel)
	}

	return unique, nil
}

func (s *Service) read(ctx context.Context, channels []string, afterID int64, limit int) ([]msgmodels.Message, error) {
	if limit <= 0 {
		limit = defaultReadLimit
	}

	if limit > maxReadLimit {
		limit = maxReadLimit
	}

	messages := []msgmodels.Message{}

	query := s.db.NewSelect().Model(&messages)
	if len(channels) == 1 {
		query = query.Where("channel = ?", channels[0])
	} else {
		query = query.Where("channel IN (?)", bun.In(channels))
	}

	err := query.
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
	return s.WaitChannels(ctx, caller, []string{channel}, afterID, timeout)
}

// WaitChannels is Wait over several channels at once (ADR 0016 item 6):
// it returns as soon as any of them has a message with id > afterID, with
// every such message from all of them in id order.
func (s *Service) WaitChannels(ctx context.Context, caller Caller, channels []string, afterID int64, timeout time.Duration) ([]msgmodels.Message, error) {
	channels, err := s.checkRead(caller, channels)
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

		messages, err := s.read(ctx, channels, afterID, maxReadLimit)
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
// channel that has messages. Archived channels are left out unless
// archived, then they carry ArchivedAt.
func (s *Service) Channels(ctx context.Context, caller Caller, archived bool) ([]ChannelInfo, error) {
	channels, err := s.channels(ctx, caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	archives, err := msgmodels.ArchivedChannels(ctx, s.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	listed := []ChannelInfo{}
	for _, channel := range channels {
		at, ok := archives[channel.Channel]
		if ok && !archived {
			continue
		}

		if ok {
			channel.ArchivedAt = &at
		}

		listed = append(listed, channel)
	}

	return listed, nil
}

// channels is Channels before archived channels are marked or left out.
func (s *Service) channels(ctx context.Context, caller Caller) ([]ChannelInfo, error) {
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

	if s.everywhere(caller) {
		err = query.Scan(ctx, &stats)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if stats == nil {
			stats = []ChannelInfo{}
		}

		return stats, nil
	}

	channels := []ChannelInfo{}
	names := ownChannels(caller)
	for _, name := range names {
		channels = append(channels, ChannelInfo{Channel: name})
	}
	for _, grant := range caller.Grants {
		if caller.Task != "" && grant.Channel == TaskChannel(caller.Task) {
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

	err = s.authorize(caller, channel, false)
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
