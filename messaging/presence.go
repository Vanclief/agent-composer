package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// HumanOnlineWindow is how recently a human must have posted or read to
// count as online.
const HumanOnlineWindow = 5 * time.Minute

// Presence answers whether an agent's compute is up, so messaging never
// imports the sandboxes API. Core wires it at startup with SetPresence.
type Presence interface {
	// SandboxRunning reports whether the sandbox called name is running
	// right now. found is false when no sandbox has that name.
	SandboxRunning(ctx context.Context, name string) (running bool, found bool)
	// TaskRunning reports whether any sandbox of the task is running.
	TaskRunning(ctx context.Context, task string) bool
}

// ParticipantView is a participant with its computed presence. Neither
// field is stored.
type ParticipantView struct {
	msgmodels.Participant

	Online     bool       `json:"online"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

// SetPresence wires the sandbox status source. Without it an agent is
// online whenever it has not left.
func (s *Service) SetPresence(p Presence) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.presence = p
}

// MarkSeen records that the participant read, waited or posted at at.
// It is kept in memory only, so a restart forgets it.
func (s *Service) MarkSeen(participantID string, at time.Time) {
	if participantID == "" {
		return
	}

	s.seenMu.Lock()
	defer s.seenMu.Unlock()

	if at.After(s.seen[participantID]) {
		s.seen[participantID] = at
	}
}

// Seen marks a participant the caller owns as seen now. It is the read
// and wait path, where the id comes from the query: an unknown id, a
// left one or one the caller does not own is ignored.
func (s *Service) Seen(ctx context.Context, caller Caller, participantID string) {
	if participantID == "" {
		return
	}

	p, err := s.participant(ctx, participantID)
	if err != nil {
		return
	}

	if !owns(caller, p) || p.LeftAt != nil {
		return
	}

	s.MarkSeen(p.ID, s.now())
}

func (s *Service) seenAt(participantID string) (time.Time, bool) {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()

	at, ok := s.seen[participantID]
	return at, ok
}

// Participant returns one participant with its presence. Humans may see
// anyone. A Task may see its own participants and anyone who posted in a
// channel it may read.
func (s *Service) Participant(ctx context.Context, caller Caller, participantID string) (*ParticipantView, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	p, err := s.participant(ctx, participantID)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if !s.everywhere(caller) && !owns(caller, p) {
		channels := ownChannels(caller)
		for _, grant := range caller.Grants {
			channels = append(channels, grant.Channel)
		}

		visible, err := s.db.NewSelect().
			Model((*msgmodels.Message)(nil)).
			Where("participant_id = ?", p.ID).
			Where("channel IN (?)", bun.In(channels)).
			Exists(ctx)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		if !visible {
			// Same answer as a missing id, so ids cannot be probed.
			return nil, ez.New(ez.ENOTFOUND, fmt.Sprintf("participant %s not found", participantID), nil)
		}
	}

	views, err := s.views(ctx, []msgmodels.Participant{*p})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &views[0], nil
}

// views computes presence for participants: one query for their latest
// messages, plus one Presence call per active agent.
func (s *Service) views(ctx context.Context, participants []msgmodels.Participant) ([]ParticipantView, error) {
	views := make([]ParticipantView, len(participants))
	if len(participants) == 0 {
		return views, nil
	}

	ids := make([]string, len(participants))
	for i := range participants {
		ids[i] = participants[i].ID
	}

	latestIDs := s.db.NewSelect().
		Model((*msgmodels.Message)(nil)).
		ColumnExpr("MAX(id)").
		Where("participant_id IN (?)", bun.In(ids)).
		Group("participant_id")

	latest := []msgmodels.Message{}

	err := s.db.NewSelect().
		Model(&latest).
		Column("participant_id", "created_at").
		Where("id IN (?)", latestIDs).
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	lastPost := map[string]time.Time{}
	for _, m := range latest {
		lastPost[m.ParticipantID] = m.CreatedAt
	}

	s.mu.Lock()
	presence := s.presence
	s.mu.Unlock()

	now := s.now()

	for i := range participants {
		p := participants[i]
		view := ParticipantView{Participant: p}

		var last time.Time
		posted, ok := lastPost[p.ID]
		if ok {
			last = posted
		}

		if p.Human {
			seen, ok := s.seenAt(p.ID)
			if ok && seen.After(last) {
				last = seen
			}
		}

		if !last.IsZero() {
			at := last.UTC()
			view.LastSeenAt = &at
		}

		view.Online = s.online(ctx, presence, &p, last, now)
		views[i] = view
	}

	return views, nil
}

func (s *Service) online(ctx context.Context, presence Presence, p *msgmodels.Participant, last, now time.Time) bool {
	if p.LeftAt != nil || isSystem(p) {
		return false
	}

	if p.Human {
		return !last.IsZero() && now.Sub(last) <= HumanOnlineWindow
	}

	if presence == nil {
		return true
	}

	// A persistent agent is online while its home runs (ADR 0016).
	if p.Agent != "" {
		running, _ := presence.SandboxRunning(ctx, p.Agent)
		return running
	}

	// An agent joins as its sandbox's name (ADR 0009, $AGC_SANDBOX).
	running, found := presence.SandboxRunning(ctx, p.Name)
	if found {
		return running
	}

	return presence.TaskRunning(ctx, p.Task)
}
