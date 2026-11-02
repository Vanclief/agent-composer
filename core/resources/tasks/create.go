package tasks

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// CreateRequest needs a title or a name. A missing name is the title's
// slug, a missing title is the name. Brief is required unless Sandbox is
// set, in which case the sandbox is allocated after the insert.
type CreateRequest struct {
	Title    string        `json:"title"`
	Name     string        `json:"name"`
	Brief    string        `json:"brief"`
	Assignee string        `json:"assignee"`
	For      string        `json:"for"`
	Sandbox  *task.Sandbox `json:"sandbox"`
}

func (r CreateRequest) Validate() error {
	_, _, err := r.resolve()
	if err != nil {
		return ez.Wrap(err)
	}

	if r.Sandbox == nil && strings.TrimSpace(r.Brief) == "" {
		return ez.New(ez.EINVALID, "brief is required for a task without a sandbox", nil)
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

	record := &taskmodels.Task{
		Title:    title,
		Name:     name,
		Brief:    request.Brief,
		Assignee: strings.TrimSpace(request.Assignee),
		For:      strings.TrimSpace(request.For),
	}

	err = record.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if request.Sandbox == nil {
		return newView(record, []sandboxes.SandboxView{}), nil
	}

	// The allocation's own view is used, read before its boot started, so
	// the sandbox always comes back starting.
	allocated, err := api.Sandboxes.AllocateFor(ctx, record, *request.Sandbox)
	if err != nil {
		api.removeUnallocated(ctx, record)
		return nil, ez.Wrap(err)
	}

	return newView(record, []sandboxes.SandboxView{*allocated}), nil
}

// removeUnallocated deletes a task whose sandbox could not be allocated, so
// a failed create leaves nothing behind.
func (api *API) removeUnallocated(ctx context.Context, record *taskmodels.Task) {
	err := record.Delete(context.WithoutCancel(ctx), api.db)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot remove task after a failed allocation")
	}
}
