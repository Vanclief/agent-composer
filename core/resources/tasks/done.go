package tasks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

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

	err = record.MarkDone(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
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

	err = record.MarkDone(ctx, api.db)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot mark the task done on /done")
	}
}
