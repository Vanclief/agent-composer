package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"unicode"

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

// prepare clones the repo, writes the agent config and runs the setup
// script, as the sandbox user. Setup runs in the repo, or in the home
// directory when there is no repo, and can still change the agent config.
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

	trusted := dir
	if trusted == "" {
		trusted = task.HomeDir
	}

	script, err := agentConfigScript(trusted)
	if err != nil {
		return ez.Wrap(err)
	}

	err = run(ctx, sb, executor.Command{Name: "bash", Args: []string{"-c", script}}, "agent config")
	if err != nil {
		return ez.Wrap(err)
	}

	if spec.Setup != "" {
		err = run(ctx, sb, executor.Command{Name: "bash", Args: []string{"-lc", spec.Setup}, Dir: dir}, "setup")
		if err != nil {
			return ez.Wrap(err)
		}
	}

	return nil
}

// agentConfigTemplate merges first-run answers into the sandbox user's
// Claude Code and Codex config, keeping whatever is already there (a
// mounted volume may hold a harness home). Keys, from the shipped CLIs:
// Claude Code's global config (~/.claude.json, or $CLAUDE_CONFIG_DIR/
// .claude.json) holds hasCompletedOnboarding, theme,
// bypassPermissionsModeAccepted (read by older releases, moved to settings
// by newer ones) and projects[dir].hasTrustDialogAccepted; its user
// settings (~/.claude/settings.json) hold skipDangerousModePermissionPrompt,
// which current releases check before the bypass-permissions dialog.
// Codex reads projects."<dir>".trust_level from $CODEX_HOME/config.toml.
// {{dir}} is replaced by the trusted directory.
const agentConfigTemplate = `set -eu
dir='{{dir}}'
claude_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
claude_json="${CLAUDE_CONFIG_DIR:-$HOME}/.claude.json"
codex_dir="${CODEX_HOME:-$HOME/.codex}"
mkdir -p "$claude_dir" "$codex_dir"

merge() {
  file="$1"
  filter="$2"
  if [ ! -s "$file" ]
  then
    echo '{}' > "$file"
  fi
  jq --arg dir "$dir" "$filter" "$file" > "$file.agc"
  mv "$file.agc" "$file"
}

merge "$claude_json" '.hasCompletedOnboarding = true | .theme = "dark" | .bypassPermissionsModeAccepted = true | .projects[$dir].hasTrustDialogAccepted = true'
merge "$claude_dir/settings.json" '.skipDangerousModePermissionPrompt = true'

codex_config="$codex_dir/config.toml"
touch "$codex_config"
if ! grep -qxF "[projects.\"$dir\"]" "$codex_config"
then
  {
    echo
    echo "[projects.\"$dir\"]"
    echo 'trust_level = "trusted"'
  } >> "$codex_config"
fi
`

// agentConfigScript renders the bash script that marks Claude Code's
// onboarding done with the dark theme, accepts its bypass-permissions
// dialog and trusts dir in Claude Code and Codex, so an agent started in
// the Task begins working without a human answering first-run prompts.
// dir is written inside shell, JSON and TOML quotes, so a path with a
// quote, a backslash or a control character is rejected.
func agentConfigScript(dir string) (string, error) {
	if !path.IsAbs(dir) || strings.ContainsAny(dir, `"'\`) || strings.IndexFunc(dir, unicode.IsControl) >= 0 {
		msg := fmt.Sprintf("directory %q cannot be marked trusted: it must be absolute, without quotes, backslashes or control characters", dir)
		return "", ez.New(ez.EINVALID, msg, nil)
	}

	return strings.Replace(agentConfigTemplate, "{{dir}}", dir, 1), nil
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
