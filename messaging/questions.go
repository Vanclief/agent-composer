package messaging

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Question is a message of kind question with its answer, if any: the
// first reply to it from someone other than the asker. The answer is
// computed on every read, never stored.
type Question struct {
	msgmodels.Message `bun:",extend"`

	AnswerID   *int64     `bun:"answer_id" json:"answer_id"`
	AnsweredBy *string    `bun:"answered_by" json:"answered_by"`
	AnsweredAt *time.Time `bun:"answered_at" json:"answered_at"`
}

// QuestionsFilter narrows Questions. Every field is optional.
type QuestionsFilter struct {
	// Channel keeps the questions posted in it.
	Channel string
	// Open keeps the unanswered questions.
	Open bool
	// For keeps the questions in the channels that person reads: their
	// inbox #<for> and whatever ChannelsFor adds.
	For string
}

// ChannelsFor returns the channels the person called name reads besides
// their inbox #<name>, e.g. #<task> for every task they manage or own.
type ChannelsFor func(ctx context.Context, name string) ([]string, error)

// SetChannelsFor sets what Questions adds to a person's inbox for a For
// filter. Without it a person reads only their inbox.
func (s *Service) SetChannelsFor(fn ChannelsFor) {
	s.mu.Lock()
	s.channelsFor = fn
	s.mu.Unlock()
}

// answerJoin is the first reply to the question q from anyone other than
// its asker, by participant and by name, so an asker who joined again is
// still the asker.
const answerJoin = `LEFT JOIN messaging_messages AS answer ON answer.id = (
	SELECT MIN(reply.id) FROM messaging_messages AS reply
	WHERE reply.reply_to = message.id
	AND reply.participant_id <> message.participant_id
	AND reply."from" <> message."from")`

// Questions lists questions newest first with their answers. A human sees
// every channel; a Task only the channels it may read.
func (s *Service) Questions(ctx context.Context, caller Caller, filter QuestionsFilter) ([]Question, error) {
	err := checkCaller(caller)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	query := s.db.NewSelect().
		Model((*Question)(nil)).
		ColumnExpr("message.*").
		ColumnExpr("answer.id AS answer_id, answer.\"from\" AS answered_by, answer.created_at AS answered_at").
		Join(answerJoin).
		Where("message.kind = ?", msgmodels.KindQuestion).
		Order("message.id DESC").
		Limit(maxReadLimit)

	if filter.Channel != "" {
		err = msgmodels.ValidateChannel(filter.Channel)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		err = authorize(caller, filter.Channel, false)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		query = query.Where("message.channel = ?", filter.Channel)
	}

	if !caller.Human {
		query = query.Where("message.channel IN (?)", bun.In(readable(caller)))
	}

	if filter.For != "" {
		channels, err := s.inboxes(ctx, filter.For)
		if err != nil {
			return nil, ez.Wrap(err)
		}

		query = query.Where("message.channel IN (?)", bun.In(channels))
	}

	if filter.Open {
		query = query.Where("answer.id IS NULL")
	}

	questions := []Question{}

	err = query.Scan(ctx, &questions)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return questions, nil
}

// readable is every channel a Task caller may read.
func readable(caller Caller) []string {
	channels := []string{TaskChannel(caller.Task)}
	for _, grant := range caller.Grants {
		channels = append(channels, grant.Channel)
	}

	return channels
}

// inboxes is #<name> plus the channels ChannelsFor adds for name.
func (s *Service) inboxes(ctx context.Context, name string) ([]string, error) {
	err := msgmodels.ValidateName(name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	channels := []string{"#" + name}

	s.mu.Lock()
	channelsFor := s.channelsFor
	s.mu.Unlock()

	if channelsFor == nil {
		return channels, nil
	}

	more, err := channelsFor(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return append(channels, more...), nil
}
