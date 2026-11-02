package sandboxes

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/models/agent"
	taskmodels "github.com/vanclief/agent-composer/models/task"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

// maxStderrBytes keeps the tail of a failed clone or setup in the error.
const maxStderrBytes = 2000

// BriefPath is where agc writes the instructions (TASK.md) in a sandbox.
const BriefPath = task.HomeDir + "/TASK.md"

// AllocateRequest allocates a sandbox to the task named by TaskID or
// TaskName, which come from the path. The body is the sandbox: the
// environment fields inline plus command.
type AllocateRequest struct {
	TaskID   uuid.UUID `json:"-"`
	TaskName string    `json:"-"`
	task.Sandbox
}

func (r AllocateRequest) Validate() error {
	err := validateRef(r.TaskID, r.TaskName)
	if err != nil {
		return ez.Wrap(err)
	}

	err = r.Sandbox.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Allocate inserts a sandbox for the task and returns it as starting. The
// boot runs in the background (see AllocateFor).
func (api *API) Allocate(ctx context.Context, requester interface{}, request *AllocateRequest) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	var owner *taskmodels.Task
	if request.TaskID != uuid.Nil {
		owner, err = taskmodels.GetTaskByID(ctx, api.db, request.TaskID)
	} else {
		owner, err = taskmodels.GetTaskByName(ctx, api.db, request.TaskName)
	}
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.AllocateFor(ctx, owner, request.Sandbox)
}

// AllocateFor inserts a sandbox for owner and returns it as starting.
// Missing secrets fail here, before anything is inserted. A goroutine with
// its own context then creates the VM, clones the repo, writes the agent
// config and TASK.md, runs setup, sets booted_at and starts the command as
// a terminal. A failure sets boot_error and discards the VM.
func (api *API) AllocateFor(ctx context.Context, owner *taskmodels.Task, request task.Sandbox) (*SandboxView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	spec := request.Spec

	bindings, err := api.resolveSecrets(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	encodedSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	command := strings.TrimSpace(request.Command)
	if command == "" {
		command = AgentCommand(owner)
	}

	record := &taskmodels.Sandbox{TaskID: owner.ID, Spec: encodedSpec, Command: command, Instructions: Instructions(owner, spec)}

	err = api.insert(ctx, owner.Name, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.startBoot(ctx, record, owner, spec, bindings)
}

// startBoot publishes the new sandbox as starting and boots it in the
// background.
func (api *API) startBoot(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task, spec task.Spec, bindings []secrets.Binding) (*SandboxView, error) {
	// The view is read before the boot starts, so it always says starting.
	view, err := api.view(ctx, record, owner)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.publish(view)

	bootCtx, cancel := context.WithTimeout(api.rootCtx, bootTimeout)
	done := make(chan struct{})

	api.mu.Lock()
	api.booting[record.Name] = &boot{cancel: cancel, done: done}
	api.mu.Unlock()

	api.boots.Add(1)
	go func() {
		defer api.boots.Done()
		defer close(done)
		defer cancel()

		booted := api.boot(bootCtx, record, owner, spec, bindings)

		// The boot leaves the booting set before the task is read, so an
		// accept either sees it booting and leaves it here, or sees it
		// booted and stops it itself.
		api.endBoot(record.Name)

		if booted && record.Agent == "" {
			api.stopIfTaskDone(context.WithoutCancel(bootCtx), record, owner)
		}
	}()

	return view, nil
}

// insert names the sandbox and inserts record. The first sandbox of a task
// (or the home of an agent) is named after base, later ones <base>-2,
// <base>-3: the first free name. A name is taken while a sandbox has it or
// while a deleted sandbox's volume (task.VolumeName) is still on the host,
// so a new sandbox never inherits another's transcripts.
func (api *API) insert(ctx context.Context, base string, record *taskmodels.Sandbox) error {
	api.naming.Lock()
	defer api.naming.Unlock()

	name := ""
	for n := 1; name == ""; n++ {
		candidate := base
		if n > 1 {
			candidate = fmt.Sprintf("%s-%d", base, n)
		}

		taken, err := taskmodels.SandboxNameTaken(ctx, api.db, candidate)
		if err != nil {
			return ez.Wrap(err)
		}

		if !taken {
			taken, err = api.volumeExists(task.VolumeName(candidate))
			if err != nil {
				return ez.Wrap(err)
			}
		}

		if !taken {
			name = candidate
		}
	}

	record.Name = name
	record.Volume = task.VolumeName(name)

	err := record.Insert(ctx, api.db)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// boot is one sandbox's boot in progress, cancelled by Delete.
type boot struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// isBooting reports whether this process is booting the sandbox.
func (api *API) isBooting(name string) bool {
	api.mu.Lock()
	defer api.mu.Unlock()

	_, ok := api.booting[name]
	return ok
}

func (api *API) endBoot(name string) {
	api.mu.Lock()
	delete(api.booting, name)
	api.mu.Unlock()
}

// cancelBoot cancels the sandbox's boot, if one is in progress, and waits
// for it to discard what it made.
func (api *API) cancelBoot(name string) {
	api.mu.Lock()
	b := api.booting[name]
	api.mu.Unlock()

	if b == nil {
		return
	}

	b.cancel()
	<-b.done
}

// boot runs the allocation, records a failure as boot_error, and reports
// whether the sandbox booted.
func (api *API) boot(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task, spec task.Spec, bindings []secrets.Binding) bool {
	err := api.bootSandbox(ctx, record, owner, spec, bindings)
	if err == nil {
		api.publishCurrent(context.WithoutCancel(ctx), record, owner)
		return true
	}

	msg := ez.ErrorMessage(err)
	record.BootError = &msg

	err = record.Update(context.WithoutCancel(ctx), api.db, "boot_error")
	if err != nil {
		log.Error().Err(err).Str("sandbox", record.Name).Str("boot_error", msg).Msg("cannot record sandbox boot error")
	}

	api.publishCurrent(context.WithoutCancel(ctx), record, owner)

	return false
}

func (api *API) bootSandbox(ctx context.Context, record *taskmodels.Sandbox, owner *taskmodels.Task, spec task.Spec, bindings []secrets.Binding) error {
	name := record.Name

	sandboxSpec, err := spec.SandboxSpec(owner.Name, name, bindings, api.socketPath(name), msgVsockPort)
	if err != nil {
		return ez.Wrap(err)
	}

	// The listener starts first so the guest can use it at once.
	err = api.startSandboxListener(record, owner, spec)
	if err != nil {
		return ez.Wrap(err)
	}

	sb, err := api.rt.Create(ctx, sandboxSpec)
	if err != nil {
		api.stopListener(name)
		return ez.Wrap(err)
	}

	err = prepare(ctx, sb, spec, briefPath(record), record.Instructions)
	if err != nil {
		return api.discard(ctx, name, sb, err)
	}

	now := time.Now().UTC()
	record.BootedAt = &now

	err = record.Update(ctx, api.db, "booted_at")
	if err != nil {
		return api.discard(ctx, name, sb, err)
	}

	if record.Command != "" {
		_, err = sb.StartTerminal(ctx, jobCommand(record, spec))
		if err != nil {
			return api.discard(ctx, name, sb, jobError(record, err))
		}
	}

	err = sb.Detach(ctx)
	if err != nil {
		log.Warn().Err(err).Str("sandbox", name).Msg("cannot release sandbox handle")
	}

	return nil
}

// jobCommand is the sandbox's job: its command, run in workDir.
func jobCommand(record *taskmodels.Sandbox, spec task.Spec) executor.Command {
	fields := strings.Fields(record.Command)

	return executor.Command{Name: fields[0], Args: fields[1:], Dir: workDir(spec)}
}

// jobError says the sandbox's job could not start.
func jobError(record *taskmodels.Sandbox, err error) error {
	return ez.New(ez.ErrorCode(err), "cannot start "+record.Command+": "+ez.ErrorMessage(err), err)
}

// workDir is where setup and the command run: the repo, or the home
// directory when there is no repo.
func workDir(spec task.Spec) string {
	if spec.Repo == "" {
		return task.HomeDir
	}

	_, repoDir, err := spec.RepoURL()
	if err != nil {
		return task.HomeDir
	}

	return repoDir
}

// Instructions is the TASK.md agc writes in every sandbox and keeps on the
// sandbox row, byte for byte: the task's title and brief, "Accepted when"
// with one bullet per criterion when it has any (ADR 0010 item 1), then
// the standing instructions of ADR 0009 item 3 filled in, ending with how
// to deliver. Without an owner it says "no owner is set" and drops the
// escalation line; without a repo it says so.
func Instructions(owner *taskmodels.Task, spec task.Spec) string {
	var b strings.Builder

	b.WriteString("# " + owner.Title + "\n\n")

	brief := strings.TrimSpace(owner.Brief)
	if brief != "" {
		b.WriteString(brief + "\n\n")
	}

	if len(owner.Accept) > 0 {
		b.WriteString("## Accepted when\n\n")
		for _, criterion := range owner.Accept {
			b.WriteString("- " + criterion + "\n")
		}
		b.WriteString("\n")
	}

	worker := owner.Assignee
	if owner.Model != "" {
		worker += " (" + owner.Model + ")"
	}

	name := owner.Name

	b.WriteString("## How this task works\n\n")
	b.WriteString("You are " + worker + " working for the task `" + name + "` inside a sandbox.\n")

	if owner.Owner != "" {
		b.WriteString("Your manager is " + owner.Manager + "; the owner is " + owner.Owner + ".\n")
	} else {
		b.WriteString("Your manager is " + owner.Manager + "; no owner is set.\n")
	}

	_, repoDir, err := spec.RepoURL()
	if spec.Repo != "" && err == nil {
		b.WriteString("The repository is cloned at " + repoDir + ".\n")
	} else {
		b.WriteString("There is no repository.\n")
	}

	b.WriteString("Work counts only once it leaves this sandbox:\n\n")
	b.WriteString("- Commit on a branch named `agc/" + name + "` and push it.\n")
	b.WriteString("- Post what you changed and what you verified in `#" + name + "` with\n")
	b.WriteString("  `agc msg post '#" + name + "' <text>`; your manager reads that channel and\n")
	b.WriteString("  answers there. Ask your manager there too, with\n")
	b.WriteString("  `agc msg ask '#" + name + "' <question>` (or start your answer with\n")
	b.WriteString("  `QUESTION:`) so the question shows up as waiting on them, then keep\n")
	b.WriteString("  working on what does not depend on the answer.\n")

	if owner.Owner != "" {
		b.WriteString("- If something needs a decision only " + owner.Owner + " can make, ask it in\n")
		b.WriteString("  `#" + owner.Owner + "` (their inbox) with `agc msg ask '#" + owner.Owner + "' <question>` so it\n")
		b.WriteString("  shows up as waiting on them, and say so in `#" + name + "`; do not wait idle.\n")
	}

	b.WriteString("- Do not modify files outside the repository, and never print secret values.\n")

	acceptors := "your manager"
	if owner.Owner != "" && owner.Owner != owner.Manager {
		acceptors = "your manager or " + owner.Owner
	}

	b.WriteString("- When the work is finished, deliver it as \"Delivering\" below says. The\n")
	b.WriteString("  task is done only when " + acceptors + " accepts it; a rejection comes\n")
	b.WriteString("  back in `#" + name + "` with the reason.\n\n")

	writeDelivering(&b, owner)

	return b.String()
}

// writeDelivering is TASK.md's "Delivering" section: write report.json
// with the fields of ADR 0010's "Delivery report", one criterion per
// "Accepted when" line, and deliver it with the summary; DONE: is the
// fallback, summary only.
func writeDelivering(b *strings.Builder, owner *taskmodels.Task) {
	criteria := "`criteria`: `[]`, as this task has no \"Accepted when\" lines"
	if len(owner.Accept) == 1 {
		criteria = "`criteria`: exactly one entry, for the \"Accepted when\" line"
	}
	if len(owner.Accept) > 1 {
		criteria = fmt.Sprintf("`criteria`: exactly %d entries, one per \"Accepted when\" line, in order", len(owner.Accept))
	}

	b.WriteString("## Delivering\n\n")
	b.WriteString("Write `report.json` (do not commit it) with exactly these fields:\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"criteria\": [ { \"met\": \"yes\", \"note\": \"how you know\" } ],\n")
	b.WriteString("  \"changes\":  { \"branch\": \"agc/" + owner.Name + "\", \"sha\": \"<pushed commit>\", \"files\": 12 },\n")
	b.WriteString("  \"checks\":   [ { \"name\": \"go test ./...\", \"passed\": true, \"note\": \"\" } ],\n")
	b.WriteString("  \"left_out\": [ { \"what\": \"what you did not do\", \"why\": \"why\" } ],\n")
	b.WriteString("  \"needs\":    [ \"what only the reviewer can decide\" ]\n")
	b.WriteString("}\n")
	b.WriteString("```\n\n")
	b.WriteString("- " + criteria + "; `met` is `yes`,\n")
	b.WriteString("  `no` or `partly`, `note` at most 500 characters.\n")
	b.WriteString("- `changes`: the branch you pushed and its commit (7 to 40 hex characters).\n")
	b.WriteString("- `checks`: what you ran and whether it passed.\n")
	b.WriteString("- `left_out`: what you did not do and why; `needs`: what you need the\n")
	b.WriteString("  reviewer to decide. `[]` when there is nothing.\n\n")
	b.WriteString("Then run `agc task deliver \"<one or two sentences>\" --report report.json`.\n")
	b.WriteString("The summary is what the reviewer reads first; the report is what they\n")
	b.WriteString("judge. A report that breaks these rules is refused with the reason: fix it\n")
	b.WriteString("and deliver again. Only when the report cannot be written, start your\n")
	b.WriteString("answer with `DONE:` followed by the summary instead; that delivers the\n")
	b.WriteString("summary alone.\n")
}

// GuestAgc is the agc binary in the agent image (images/agc-agent).
const GuestAgc = "/usr/local/bin/agc"

// IsHarness reports whether assignee is a harness id rather than a person.
func IsHarness(assignee string) bool {
	return agent.Harness(assignee).Validate() == nil
}

// AgentCommand is the first job of a sandbox whose task is assigned to a
// harness and whose request gives no command: the guest agc runs the
// agent loop on TASK.md (ADR 0009 item 2). Empty for a person's task.
func AgentCommand(owner *taskmodels.Task) string {
	if !IsHarness(owner.Assignee) {
		return ""
	}

	fields := []string{GuestAgc, "agent", "run", "--harness", owner.Assignee}
	if owner.Model != "" {
		fields = append(fields, "--model", owner.Model)
	}
	fields = append(fields, "--task", owner.Name)
	// The loop closes on /accept only from these two.
	if owner.Manager != "" {
		fields = append(fields, "--manager", owner.Manager)
	}
	if owner.Owner != "" {
		fields = append(fields, "--owner", owner.Owner)
	}
	fields = append(fields, "--brief", BriefPath)

	return strings.Join(fields, " ")
}

// prepare gives the sandbox user the home disk (a fresh ext4 disk belongs
// to root), lays out the home (homeScript), clones the repo, writes the
// agent config and the instructions at brief (TASK.md, or AGENT.md in an
// agent's home), and runs the setup script,
// as the sandbox user. Setup runs in the repo, or in the home directory
// when there is no repo, and can still change the agent config.
func prepare(ctx context.Context, sb Sandbox, spec task.Spec, brief string, instructions string) error {
	err := run(ctx, sb, executor.Command{Name: "chown", Args: []string{guestUser + ":" + guestUser, task.HomeDir}, User: "root"}, "chown "+task.HomeDir)
	if err != nil {
		return ez.Wrap(err)
	}

	err = run(ctx, sb, executor.Command{Name: "bash", Args: []string{"-c", homeScript}}, "home setup")
	if err != nil {
		return ez.Wrap(err)
	}

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

	err = sb.WriteFile(ctx, brief, []byte(instructions))
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

// guestUser is the sandbox user (sandbox.Spec's default).
const guestUser = "agent"

// homeScript lays out a fresh home disk, which hides the image's
// /home/agent: the shell dotfiles from /etc/skel, and ~/.claude and
// ~/.codex as links into the harness volume (task.HarnessDir), so the
// harness transcripts outlive the sandbox. A real directory already in
// their place is moved into the volume first. It runs again harmlessly.
const homeScript = `set -eu
cd "$HOME"
if [ ! -e .profile ]
then
  cp -r /etc/skel/. .
fi
for d in .claude .codex
do
  mkdir -p "` + task.HarnessDir + `/$d"
  if [ -d "$d" ] && [ ! -L "$d" ]
  then
    cp -r "$d/." "` + task.HarnessDir + `/$d/"
    rm -rf "$d"
  fi
  ln -sfn "` + task.HarnessDir + `/$d" "$d"
done
`

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
// the sandbox begins working without a human answering first-run prompts.
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

// discard undoes a half-booted sandbox: stops and removes the VM and its
// listener. The returned error is cause, extended to say what happened to
// the VM.
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
