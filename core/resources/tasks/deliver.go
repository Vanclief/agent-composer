package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// deliveredPrefix starts the message a delivery posts in #<task>.
const deliveredPrefix = "Delivered: "

type DeliverRequest struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Summary string    `json:"summary"`
}

func (r DeliverRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	summary := strings.TrimSpace(r.Summary)
	if summary == "" {
		return ez.New(ez.EINVALID, "summary is required: say what was done and where the work is", nil)
	}

	limit := msgmodels.MaxBodyBytes - len(deliveredPrefix)
	if len(summary) > limit {
		return ez.New(ez.EINVALID, fmt.Sprintf("summary is longer than %d bytes", limit), nil)
	}

	return nil
}

// Deliver sets delivered_at and summary and posts "Delivered: <summary>"
// in #<task> as agc (ADR 0010 item 2). The sandboxes are left as they are.
// A task can be delivered again; the last summary wins. A done task
// cannot be delivered.
func (api *API) Deliver(ctx context.Context, requester interface{}, request *DeliverRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if record.DoneAt != nil {
		return nil, ez.New(ez.ECONFLICT, "task "+record.Name+" is done: it was accepted by "+orUnknown(record.AcceptedBy), nil)
	}

	summary := strings.TrimSpace(request.Summary)

	err = record.MarkDelivered(ctx, api.db, summary)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.post(ctx, record, deliveredPrefix+summary)

	return api.view(ctx, record)
}

// deliverFromSandbox is agc task deliver on a sandbox's socket, which
// speaks for its own task only.
func (api *API) deliverFromSandbox(ctx context.Context, taskName string, summary string) (any, error) {
	return api.Deliver(ctx, nil, &DeliverRequest{Name: taskName, Summary: summary})
}

// post posts body in #<task> as agc. The change it reports is already
// stored, so a failure is logged, not returned.
func (api *API) post(ctx context.Context, record *taskmodels.Task, body string) {
	_, err := api.msg.PostAsSystem(ctx, messaging.TaskChannel(record.Name), body)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot post in the task's channel")
	}
}

func orUnknown(name string) string {
	if name == "" {
		return "someone unrecorded"
	}

	return name
}
