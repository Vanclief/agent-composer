package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/task"
)

// maxStderrBytes keeps the tail of a failed clone or setup in the error.
const maxStderrBytes = 2000

// CreateRequest needs a title or a name. A missing name is the title's
// slug, a missing title is the name.
type CreateRequest struct {
	Title string    `json:"title"`
	Name  string    `json:"name"`
	Spec  task.Spec `json:"spec"`
}

func (r CreateRequest) Validate() error {
	_, _, err := r.resolve()
	if err != nil {
		return ez.Wrap(err)
	}

	err = r.Spec.Validate()
	if err != nil {
		return ez.Wrap(err)
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

// Create boots the Task's sandbox, clones its repo and runs its setup.
// The messaging listener starts first so the guest can use it at once.
// A failure after the sandbox exists removes the sandbox and says so.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	title, name, err := request.resolve()
	if err != nil {
		return nil, ez.Wrap(err)
	}
	spec := request.Spec

	_, err = taskmodels.GetTaskByName(ctx, api.db, name)
	if err == nil {
		return nil, ez.New(ez.ECONFLICT, "task "+name+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	bindings, err := api.resolveSecrets(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	sandboxSpec, err := spec.SandboxSpec(name, bindings, api.socketPath(name), msgVsockPort)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	encodedSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = api.startTaskListener(name, spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	sb, err := api.rt.Create(ctx, sandboxSpec)
	if err != nil {
		api.stopListener(name)
		return nil, ez.Wrap(err)
	}

	err = prepare(ctx, sb, spec)
	if err != nil {
		return nil, api.discard(ctx, name, sb, err)
	}

	record := &taskmodels.Task{Title: title, Name: name, Spec: encodedSpec}
	err = record.Insert(ctx, api.db)
	if err != nil {
		return nil, api.discard(ctx, name, sb, err)
	}

	err = sb.Detach(ctx)
	if err != nil {
		log.Warn().Err(err).Str("task", name).Msg("cannot release sandbox handle")
	}

	return api.view(ctx, record)
}

// prepare clones the repo and runs the setup script, as the sandbox user.
// Setup runs in the repo, or in the home directory when there is no repo.
func prepare(ctx context.Context, sb Sandbox, spec task.Spec) error {
	dir := ""

	if spec.Repo != "" {
		cloneURL, repoDir, err := spec.RepoURL()
		if err != nil {
			return ez.Wrap(err)
		}

		err = run(ctx, sb, executor.Command{Name: "git", Args: []string{"clone", cloneURL, repoDir}}, "git clone "+cloneURL)
		if err != nil {
			return ez.Wrap(err)
		}

		// checkout, not clone --branch, so ref may also be a commit sha.
		if spec.Ref != "" {
			err = run(ctx, sb, executor.Command{Name: "git", Args: []string{"-C", repoDir, "checkout", spec.Ref}}, "git checkout "+spec.Ref)
			if err != nil {
				return ez.Wrap(err)
			}
		}

		dir = repoDir
	}

	if spec.Setup != "" {
		err := run(ctx, sb, executor.Command{Name: "bash", Args: []string{"-lc", spec.Setup}, Dir: dir}, "setup")
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}

// run runs cmd and turns a non-zero exit into an error carrying the tail
// of its stderr.
func run(ctx context.Context, sb Sandbox, cmd executor.Command, what string) error {
	result, err := sb.Run(ctx, cmd)
	if err != nil {
		return ez.Wrap(err)
	}

	if result.ExitCode == 0 {
		return nil
	}

	msg := fmt.Sprintf("%s failed with exit code %d", what, result.ExitCode)
	stderr := strings.TrimSpace(string(result.Stderr))
	if len(stderr) > maxStderrBytes {
		stderr = "..." + stderr[len(stderr)-maxStderrBytes:]
	}
	if stderr != "" {
		msg += ": " + stderr
	}

	return ez.New(ez.EINTERNAL, msg, nil)
}

// discard undoes a half-made Task: stops and removes its sandbox and its
// listener. The returned error is cause, extended to say what happened to
// the sandbox.
func (api *API) discard(ctx context.Context, name string, sb Sandbox, cause error) error {
	cleanupCtx := context.WithoutCancel(ctx)

	_ = sb.Detach(cleanupCtx)
	api.stopListener(name)

	err := api.rt.Stop(cleanupCtx, name)
	if err == nil {
		err = api.rt.Remove(cleanupCtx, name)
	}

	msg := ez.ErrorMessage(cause) + " (sandbox " + name + " was removed)"
	if err != nil {
		msg = ez.ErrorMessage(cause) + " (sandbox " + name + " could not be removed: " + ez.ErrorMessage(err) + ")"
	}

	return ez.New(ez.ErrorCode(cause), msg, cause)
}
