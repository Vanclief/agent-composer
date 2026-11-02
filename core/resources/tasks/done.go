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

type DoneRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (r DoneRequest) Validate() error {
	return validateRef(r.ID, r.Name)
}

// Done sets done_at, the only thing that does. Its sandboxes are left as
// they are. A task already done keeps its first done_at.
func (api *API) Done(ctx context.Context, requester interface{}, request *DoneRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.markDone(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}

// markDone sets done_at, the one place that does, and publishes task.done
// when the task was not done before.
func (api *API) markDone(ctx context.Context, record *taskmodels.Task) error {
	wasDone := record.DoneAt != nil

	err := record.MarkDone(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	if wasDone || api.events == nil {
		return nil
	}

	view, err := api.view(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot read the task for task.done")
		return nil
	}

	summary, err := api.Summary(ctx, record.Name)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot read the summary for task.done")
	}

	api.events.Publish(events.Event{Type: events.TaskDone, Data: DoneEvent{Task: view, Summary: summary}})

	return nil
}

// summaryLength caps the summary in task.done, in characters.
const summaryLength = 500

// Summary is the body of the last message in #<task> from neither a human
// nor agc, the agent's closing words, cut to 500 characters, or "" when there
// is none. A bare /done is a command, not a summary, and is skipped.
func (api *API) Summary(ctx context.Context, name string) (string, error) {
	var messages []msgmodels.Message

	err := api.db.NewSelect().
		Model(&messages).
		Where("channel = ?", messaging.TaskChannel(name)).
		Where("human = ?", false).
		Where(`"from" != ?`, msgmodels.SystemName).
		Where("TRIM(body) != ?", DoneCommand).
		Order("id DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return "", ez.Wrap(err)
	}

	if len(messages) == 0 {
		return "", nil
	}

	body := []rune(messages[0].Body)
	if len(body) > summaryLength {
		body = body[:summaryLength]
	}

	return string(body), nil
}

// DoneCommand closes a task when its manager or owner posts it in the
// task's channel (ADR 0009 item 5).
const DoneCommand = "/done"

// closeOnDone sets done_at when the manager or the owner posts /done in
// #<task>, the same effect as Done. From anyone else, including the
// task's own sandboxes, it is an ordinary message.
func (api *API) closeOnDone(ctx context.Context, m *msgmodels.Message) {
	if strings.TrimSpace(m.Body) != DoneCommand {
		return
	}

	name := strings.TrimPrefix(m.Channel, "#")
	if m.Task == name {
		return
	}

	record, err := taskmodels.GetTaskByName(ctx, api.db, name)
	if ez.ErrorCode(err) == ez.ENOTFOUND {
		return
	}
	if err != nil {
		log.Warn().Err(err).Str("channel", m.Channel).Msg("cannot read the task for /done")
		return
	}

	if m.From != record.Manager && m.From != record.Owner {
		return
	}

	err = api.markDone(ctx, record)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot mark the task done on /done")
	}
}
