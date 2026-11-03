package tasks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type AcceptRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// By is who accepts; empty is the task's owner.
	By string `json:"by"`
}

func (r AcceptRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	return validateBy(r.By)
}

// Accept closes the task (ADR 0010 item 4): it sets done_at and
// accepted_by, posts "Accepted by <name>." in #<task>, closes the task's
// open agent questions (closeQuestions), then stops every
// running sandbox of the task, and every starting one as soon as it has
// booted. A sandbox that cannot be stopped is logged and does not fail
// the accept. Nothing is deleted. A
// task already done is returned as it is.
func (api *API) Accept(ctx context.Context, requester interface{}, request *AcceptRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	by, err := resolveBy(request.By, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.accept(ctx, record, by)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}

func (api *API) accept(ctx context.Context, record *taskmodels.Task, by string) error {
	accepted, err := api.markDone(ctx, record, by)
	if err != nil {
		return ez.Wrap(err)
	}

	if !accepted {
		return nil
	}

	api.post(ctx, record, "Accepted by "+by+".")
	api.closeQuestions(ctx, record, by)
	api.stopSandboxes(ctx, record)

	return nil
}

// markDone sets done_at and accepted_by, the one place that does, and
// reports whether this call did. The first time it publishes task.done
// with the task's summary (doneSummary); a task already done, even by an
// accept racing this one, publishes nothing.
func (api *API) markDone(ctx context.Context, record *taskmodels.Task, by string) (bool, error) {
	accepted, err := record.MarkAccepted(ctx, api.db, by)
	if err != nil {
		return false, ez.Wrap(err)
	}

	if !accepted || api.events == nil {
		return accepted, nil
	}

	view, err := api.view(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot read the task for task.done")
		return true, nil
	}

	summary, err := api.doneSummary(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot read the summary for task.done")
	}

	api.events.Publish(events.Event{Type: events.TaskDone, Data: DoneEvent{Task: view, Summary: summary}})

	return true, nil
}

// summaryLength caps the summary in task.done, in runes.
const summaryLength = 500

// summaryPage is how many messages doneSummary reads at a time.
const summaryPage = 50

// doneSummary is the summary task.done carries: the task's summary when
// it is delivered, otherwise the body of the last message in #<task> from
// neither a human nor agc, the agent's closing words, skipping /accept
// and /reject commands; cut to 500 runes, "" when there is none.
func (api *API) doneSummary(ctx context.Context, record *taskmodels.Task) (string, error) {
	if record.DeliveredAt != nil && record.Summary != "" {
		return cutSummary(record.Summary), nil
	}

	var before int64
	for {
		var messages []msgmodels.Message

		query := api.db.NewSelect().
			Model(&messages).
			Where("channel = ?", messaging.TaskChannel(record.Name)).
			Where("human = ?", false).
			Where(`"from" != ?`, msgmodels.SystemName).
			Order("id DESC").
			Limit(summaryPage)
		if before > 0 {
			query = query.Where("id < ?", before)
		}

		err := query.Scan(ctx)
		if err != nil {
			return "", ez.Wrap(err)
		}

		for _, m := range messages {
			_, command := ParseCommand(m.Body)
			if !command {
				return cutSummary(m.Body), nil
			}
		}

		if len(messages) < summaryPage {
			return "", nil
		}

		before = messages[len(messages)-1].ID
	}
}

// cutSummary cuts body to summaryLength runes.
func cutSummary(body string) string {
	runes := []rune(body)
	if len(runes) > summaryLength {
		runes = runes[:summaryLength]
	}

	return string(runes)
}

// closeQuestions closes, with agc's "Closed: <task> accepted by <by>."
// reply, the open agent questions in #<task> and the manager's accept
// questions about the task in the owner's inbox (ADR 0010 "Accept"). A
// failure is logged and does not fail the accept; the collector's tick
// closes what is left.
func (api *API) closeQuestions(ctx context.Context, record *taskmodels.Task, by string) {
	closing := messaging.Closing{Task: record.Name, Manager: record.Manager, Owner: record.Owner, By: by}

	closed, err := api.msg.CloseQuestions(ctx, []messaging.Closing{closing})
	for _, c := range closed {
		log.Info().Str("task", c.Task).Str("channel", c.Channel).Int64("question", c.Question).Str("from", c.From).Msg("messaging: closed question")
	}
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot close the task's questions")
	}
}

// stopSandboxes stops the task's running sandboxes the way
// POST /api/sandboxes/{id}/stop does; one still booting stops when its
// boot finishes (sandboxes.StopTask).
func (api *API) stopSandboxes(ctx context.Context, record *taskmodels.Task) {
	api.Sandboxes.StopTask(ctx, record)
}

// validateBy checks a given by is a messaging name; empty is allowed.
func validateBy(by string) error {
	trimmed := strings.TrimSpace(by)
	if trimmed == "" {
		return nil
	}

	return validateParticipant("by", trimmed)
}

// resolveBy is by, else the task's owner.
func resolveBy(by string, record *taskmodels.Task) (string, error) {
	resolved := firstNonEmpty(by, record.Owner)
	if resolved == "" {
		return "", ez.New(ez.EINVALID, "by is required: task "+record.Name+" has no owner to default to", nil)
	}

	return resolved, nil
}
