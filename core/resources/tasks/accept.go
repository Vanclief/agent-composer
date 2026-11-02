package tasks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/vanclief/ez"

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
// accepted_by, posts "Accepted by <name>." in #<task>, then stops every
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
	if record.DoneAt != nil {
		return nil
	}

	err := record.MarkAccepted(ctx, api.db, by)
	if err != nil {
		return ez.Wrap(err)
	}

	api.post(ctx, record, "Accepted by "+by+".")
	api.stopSandboxes(ctx, record)

	return nil
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
