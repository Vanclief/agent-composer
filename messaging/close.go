package messaging

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// ClosedPrefix starts agc's reply that closes a done task's question.
const ClosedPrefix = "Closed: "

// Closing names a done task whose open questions agc closes: Task's
// channel, and Owner's inbox for Manager's accept questions about Task.
// By is who accepted it.
type Closing struct {
	Task    string
	Manager string
	Owner   string
	By      string
}

// Closed is one question agc closed.
type Closed struct {
	Task     string
	Channel  string
	Question int64
	From     string
}

// ClosedBody is agc's reply that closes a question of task, accepted by
// by: "Closed: <task> accepted by <by>.".
func ClosedBody(task, by string) string {
	if by == "" {
		by = "someone unrecorded"
	}

	return ClosedPrefix + task + " accepted by " + by + "."
}

// CloseQuestions closes the open questions of the done tasks in closings
// (ADR 0010 "Accept"): every open question in #<task>, and every open
// question in #<owner> from the task's manager whose body starts with
// "Accept <task>?" or "<task>:". agc replies in its thread with
// ClosedBody, which answers it for Questions. A person's question is
// never closed. A reply that fails stops the run; the ones before it
// stay closed.
func (s *Service) CloseQuestions(ctx context.Context, closings []Closing) ([]Closed, error) {
	closed := []Closed{}
	if len(closings) == 0 {
		return closed, nil
	}

	channels := []string{}
	for _, c := range closings {
		channels = append(channels, TaskChannel(c.Task))
		if c.Owner != "" {
			channels = append(channels, "#"+c.Owner)
		}
	}

	questions := []msgmodels.Message{}

	err := s.db.NewSelect().
		Model(&questions).
		Join(answerJoin).
		Where("message.kind = ?", msgmodels.KindQuestion).
		Where("message.human = ?", false).
		Where("message.channel IN (?)", bun.In(channels)).
		Where("answer.id IS NULL").
		Order("message.id ASC").
		Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for i := range questions {
		q := &questions[i]

		c, ok := closingFor(q, closings)
		if !ok {
			continue
		}

		_, err = s.replyAsSystem(ctx, q, ClosedBody(c.Task, c.By))
		if err != nil {
			return closed, ez.Wrap(err)
		}

		closed = append(closed, Closed{Task: c.Task, Channel: q.Channel, Question: q.ID, From: q.From})
	}

	return closed, nil
}

// closingFor is the closing that closes q: its task's channel, or its
// owner's inbox when the manager asked about the task there.
func closingFor(q *msgmodels.Message, closings []Closing) (Closing, bool) {
	for _, c := range closings {
		if q.Channel == TaskChannel(c.Task) {
			return c, true
		}

		if c.Owner == "" || c.Manager == "" || q.Channel != "#"+c.Owner || q.From != c.Manager {
			continue
		}

		if strings.HasPrefix(q.Body, "Accept "+c.Task+"?") || strings.HasPrefix(q.Body, c.Task+":") {
			return c, true
		}
	}

	return Closing{}, false
}

// replyAsSystem posts body as agc as a reply to q, the reply that
// answers it (answerJoin).
func (s *Service) replyAsSystem(ctx context.Context, q *msgmodels.Message, body string) (*msgmodels.Message, error) {
	p, err := s.systemParticipant(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	id := q.ID

	return s.insert(ctx, p, q.Channel, body, &id, msgmodels.KindMessage)
}
