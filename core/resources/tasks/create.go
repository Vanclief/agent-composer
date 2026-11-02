package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/events"
	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// CreateRequest needs a title or a name. A missing name is the title's
// slug, a missing title is the name. Brief is required unless Sandbox is
// set, in which case the sandbox is allocated after the insert. Manager is
// the creator's messaging name and is required; an empty assignee, model
// or owner gets the configured default (ADR 0009 item 1).
type CreateRequest struct {
	Title    string `json:"title"`
	Name     string `json:"name"`
	Brief    string `json:"brief"`
	Assignee string `json:"assignee"`
	Model    string `json:"model"`
	Manager  string `json:"manager"`
	Owner    string `json:"owner"`
	For      string `json:"for"`
	// Notify asks the panel to notify when the task starts and when it is
	// done. Omitted means true.
	Notify  *bool         `json:"notify,omitempty"`
	Sandbox *task.Sandbox `json:"sandbox"`
}

func (r CreateRequest) Validate() error {
	_, _, err := r.resolve()
	if err != nil {
		return ez.Wrap(err)
	}

	if r.Sandbox == nil && strings.TrimSpace(r.Brief) == "" {
		return ez.New(ez.EINVALID, "brief is required for a task without a sandbox", nil)
	}

	manager := strings.TrimSpace(r.Manager)
	if manager == "" {
		return ez.New(ez.EINVALID, "manager is required: the messaging name of whoever creates and runs the task", nil)
	}

	err = validateParticipant("manager", manager)
	if err != nil {
		return ez.Wrap(err)
	}

	owner := strings.TrimSpace(r.Owner)
	if owner != "" {
		err = validateParticipant("owner", owner)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if r.Sandbox != nil {
		err = r.Sandbox.Validate()
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}

// resolve returns the title and the name the Task gets.
func (r CreateRequest) resolve() (string, string, error) {
	title := strings.TrimSpace(r.Title)
	name := strings.TrimSpace(r.Name)

	if title == "" && name == "" {
		return "", "", ez.New(ez.EINVALID, "title or name is required", nil)
	}

	if name == "" {
		slug, err := taskmodels.Slug(title)
		if err != nil {
			return "", "", ez.Wrap(err)
		}
		name = slug
	}

	if title == "" {
		title = name
	}

	err := taskmodels.ValidateTitle(title)
	if err != nil {
		return "", "", ez.Wrap(err)
	}

	err = taskmodels.ValidateName(name)
	if err != nil {
		return "", "", ez.Wrap(err)
	}

	return title, name, nil
}

// validateParticipant checks a manager or owner is a messaging name, so
// agc can address it and #<owner> is a channel.
func validateParticipant(field string, name string) error {
	err := msgmodels.ValidateName(name)
	if err != nil {
		return ez.New(ez.EINVALID, field+" "+ez.ErrorMessage(err), err)
	}

	return nil
}

// Applied is who works on a task, with which model, and who owns it, after
// the defaults.
type Applied struct {
	Assignee string `json:"assignee"`
	Model    string `json:"model"`
	Owner    string `json:"owner"`
}

// ApplyDefaults fills an empty assignee with the default harness, an empty
// model with the default model when the assignee is a harness, and an
// empty owner with the default owner. A model on a person's task is
// refused.
func ApplyDefaults(request *CreateRequest, defaults controller.DefaultsSettings) (Applied, error) {
	applied := Applied{
		Assignee: firstNonEmpty(request.Assignee, defaults.Harness),
		Model:    strings.TrimSpace(request.Model),
		Owner:    firstNonEmpty(request.Owner, defaults.Owner),
	}

	if sandboxes.IsHarness(applied.Assignee) && applied.Model == "" {
		applied.Model = defaults.Model
	}

	if !sandboxes.IsHarness(applied.Assignee) && applied.Model != "" {
		msg := fmt.Sprintf("model %s is set but assignee %s is a person, not a harness (claude_code, codex_cli, pi)", applied.Model, applied.Assignee)
		return applied, ez.New(ez.EINVALID, msg, nil)
	}

	if applied.Owner != "" {
		err := validateParticipant("owner", applied.Owner)
		if err != nil {
			return applied, ez.Wrap(err)
		}
	}

	return applied, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// Create inserts the task and returns at once. With a sandbox, it is
// allocated after the insert and boots in the background, so the task
// comes back with one starting sandbox. A sandbox that cannot be allocated
// (a missing secret) removes the task again.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	title, name, err := request.resolve()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	_, err = taskmodels.GetTaskByName(ctx, api.db, name)
	if err == nil {
		return nil, ez.New(ez.ECONFLICT, "task "+name+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	applied, err := ApplyDefaults(request, api.defaults)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record := &taskmodels.Task{
		Title:    title,
		Name:     name,
		Brief:    request.Brief,
		Assignee: applied.Assignee,
		Model:    applied.Model,
		Manager:  strings.TrimSpace(request.Manager),
		Owner:    applied.Owner,
		For:      strings.TrimSpace(request.For),
		Notify:   request.Notify == nil || *request.Notify,
	}

	err = record.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if request.Sandbox == nil {
		api.open(ctx, record, nil)
		return api.created(newView(record, []sandboxes.SandboxView{})), nil
	}

	// The allocation's own view is used, read before its boot started, so
	// the sandbox always comes back starting.
	allocated, err := api.Sandboxes.AllocateFor(ctx, record, *request.Sandbox)
	if err != nil {
		api.removeUnallocated(ctx, record)
		return nil, ez.Wrap(err)
	}

	api.open(ctx, record, allocated)

	return api.created(newView(record, []sandboxes.SandboxView{*allocated})), nil
}

// created publishes task.created with view and returns it.
func (api *API) created(view *TaskView) *TaskView {
	api.events.Publish(events.Event{Type: events.TaskCreated, Data: CreatedEvent{Task: view}})

	return view
}

// removeUnallocated deletes a task whose sandbox could not be allocated, so
// a failed create leaves nothing behind.
func (api *API) removeUnallocated(ctx context.Context, record *taskmodels.Task) {
	err := record.Delete(context.WithoutCancel(ctx), api.db)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot remove task after a failed allocation")
	}
}
