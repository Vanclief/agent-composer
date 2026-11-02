package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

type RejectRequest struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// By is who rejects; empty is the task's owner.
	By  string `json:"by"`
	Why string `json:"why"`
}

func (r RejectRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	err = validateBy(r.By)
	if err != nil {
		return ez.Wrap(err)
	}

	why := strings.TrimSpace(r.Why)
	if why == "" {
		return ez.New(ez.EINVALID, "why is required: the agent takes it as its next turn", nil)
	}

	// "Rejected by <name>: " fits in what is left.
	limit := msgmodels.MaxBodyBytes - 100
	if len(why) > limit {
		return ez.New(ez.EINVALID, fmt.Sprintf("why is longer than %d bytes", limit), nil)
	}

	return nil
}

// Reject sends the task back (ADR 0010 item 5): it clears delivered_at,
// keeps summary, and posts "Rejected by <name>: <why>" in #<task>, which
// the agent takes as its next turn. A done task cannot be rejected.
func (api *API) Reject(ctx context.Context, requester interface{}, request *RejectRequest) (*TaskView, error) {
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

	err = api.reject(ctx, record, by, strings.TrimSpace(request.Why))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(ctx, record)
}

func (api *API) reject(ctx context.Context, record *taskmodels.Task, by string, why string) error {
	if record.DoneAt != nil {
		return ez.New(ez.ECONFLICT, "task "+record.Name+" is done: it was accepted by "+orUnknown(record.AcceptedBy), nil)
	}

	err := record.MarkRejected(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	api.post(ctx, record, "Rejected by "+by+": "+why)

	return nil
}
