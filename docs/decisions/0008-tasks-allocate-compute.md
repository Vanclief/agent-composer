# ADR 0008: Tasks allocate compute

**Status:** Accepted — 2026-11-01. Amends [ADR 0003](0003-tasks-run-in-microsandbox-vms.md) items 1, 4 and 9.

## Context

ADR 0003 made a Task one microsandbox VM: same name, same lifecycle. The
first real task run on the rig (2026-11-01, `stop-icon-black`) showed what
that costs:

- A task cannot exist before its VM has booted and cloned, so creating one
  blocks for minutes and the API call times out while the server carries on.
- A task cannot be retried on a fresh VM, cannot outlive its VM, and cannot be
  given to a human, who has no VM.
- The thing that needs doing (the brief) had no place in the spec at all; the
  agent's prompt was smuggled in through `setup`.

Franco's framing: a task is something that needs to be done; it requests, or
is allocated, compute to get done. The VM is not the task.

## Decision

1. **Two things, two words.** A **Task** is an order: work to be done, in
   words. A **Sandbox** is the compute a task is allocated: one microsandbox
   VM, the word the runtime and this codebase already use. "Machine" is reserved for the physical host or a rented server. The
   verb is *allocate*: a task allocates compute and gets a sandbox; the
   Work page that lists sandboxes is titled **Compute**, and its button
   reads "Allocate compute".

2. **A Task is**: `id`, `title`, `name` (slug, derived from the title as
   before), `brief` (what to do and what done looks like, markdown),
   `assignee` (a harness id such as `claude_code` or `codex_cli`, or a
   person's name; empty means unassigned), `for` (optional id of the
   knowledge action or decision it serves, ADR 0006), `done_at`
   (nullable), `created_at`, `updated_at`. Creating a task writes the row
   and returns at once. The channel `#<name>` belongs to the task.

   A task's `status` is computed, never stored: `done` when `done_at` is
   set, otherwise `running` when one of its sandboxes is `running`, otherwise
   `open`. `POST /api/tasks/{id}/done` sets `done_at`; nothing else does.

3. **A Sandbox is**: `id`, `task_id`, `name` (the sandbox name), `spec` (the
   ADR 0003 environment block: image, cpus, memory, repo, ref, setup,
   secrets, env, volumes; unchanged), `command` (optional: the program the
   sandbox runs for the task as its first terminal, split on whitespace like
   `StartTerminalRequest.Command`), `boot_error` (nullable text),
   `booted_at` (nullable), `created_at`, `stopped_at` (nullable). The
   first sandbox of a task is named after the task; later ones `<task>-2`,
   `<task>-3`, so existing sandboxes keep their names. The sandbox carries
   labels `agc.task=<task>` and `agc.sandbox=<sandbox>`.

   A sandbox's `status` is read live as before: `starting` while `booted_at` is
   null and `boot_error` is null, `failed` when `boot_error` is set (with
   `status_error` carrying the text), otherwise the runtime's word
   (`running`, `stopped`, `unknown`). agc still invents no VM state.

4. **Allocation is asynchronous.** `POST /api/tasks/{id}/sandboxes {spec,
   command}` inserts the sandbox row and returns `201` with status `starting`;
   a goroutine with its own 30-minute context creates the sandbox, clones,
   writes `/home/agent/TASK.md` (the task's title and brief) and the
   harness pre-acceptance config, runs `setup`, sets `booted_at`, then, if
   `command` is set, starts it as a TTY job named `agent`. A failure sets
   `boot_error` and discards the sandbox. `POST /api/tasks` accepts an
   optional `sandbox` object and allocates it the same way after the insert, so
   one call still makes a working task.

5. **Sandboxes carry the VM operations; tasks carry the work.**
   `GET /api/sandboxes`, `GET /api/sandboxes/{id}`, `POST /api/sandboxes/{id}/stop`,
   `POST /api/sandboxes/{id}/start`, `DELETE /api/sandboxes/{id}`, and the
   terminals move under sandboxes: `GET|POST /api/sandboxes/{id}/terminals`,
   `GET /api/sandboxes/{id}/terminals/{job}/ws`. A sandbox id or name is accepted
   where `{id}` appears, as for tasks. `GET /api/tasks` and
   `GET /api/tasks/{id}` embed `sandboxes: [BoxView]` newest first. `DELETE
   /api/tasks/{id}` deletes its sandboxes, then the task. The old
   `/api/tasks/{id}/stop|start|terminals` routes are removed, not aliased.

6. **Work leaves the sandbox or it does not count.** agc stores no artifacts and
   syncs no files. A sandbox's output is a pushed branch, a message in the
   task's channel, or a knowledge file. The `TASK.md` agc writes ends with
   the standing instruction: commit on a branch named `agc/<task>`, push
   it, and post what changed in `#<task>`. Stopping or deleting a sandbox whose
   clone has uncommitted changes or unpushed commits says so in the control
   itself (the open action `task-delete-says-what-is-lost`, now about
   sandboxes).

7. **`task.yaml` gains the brief and nests the sandbox.** Top level: `name`,
   `title`, `brief`, `assignee`, `for`, and `sandbox:` holding the environment
   block plus `command`. `agc task create -f task.yaml` creates the task and
   allocates the sandbox when `sandbox:` is present. In a workflow spec,
   `environment:` keeps the environment block unchanged. Saved specs
   (`~/.agent_composer/config/specs/`) are sandbox specs.

8. **Migration.** `tasks` gains `brief`, `assignee`, `for`, `done_at`; a new
   `sandboxes` table is created, and every existing task row produces one sandbox
   with the task's name and spec, `booted_at` = the task's `created_at`.
   `tasks.spec` is dropped afterwards. Running sandboxes are unaffected
   because names do not change.

## Disk and transcripts

Added 2026-11-02 (task `sandbox-disk`). Agents ran out of space in the
image's small rootfs (Go build cache, Playwright browsers in /home/agent),
and the harness transcripts, the only record of why an agent did what it
did, were lost with the sandbox.

- The spec gains `disk` (task.yaml `disk:`, the REST `sandbox` block): a
  size such as `10G`, default 20G, minimum 2G. `/home/agent` is a private
  ext4 disk of that size (`--mount-owned /home/agent:kind=disk,size=<disk>`):
  a real bound, kept across stop and start, removed with the sandbox.
- The harness homes live on a named volume `agc-<sandbox>` mounted at
  `/home/agent/.harness` (`quota=4G`); `~/.claude` and `~/.codex` link into
  it. msb keeps named volumes when a sandbox is removed, so the transcripts
  survive. The row records the volume (`volume`, empty for older sandboxes)
  and a name stays taken while its volume is on the host, so a new sandbox
  never inherits another's transcripts.
- Why not one named volume with `quota=<disk>` for all of `/home/agent`:
  msb 0.7.13 enforces a named directory volume's quota in its virtiofs
  server as a budget on growth since the first guest write of each boot.
  It does not bound the volume's size on the host: every stop and start
  grants another quota, and deleted sandboxes would leave unbounded
  directories behind.
- `GET /api/sandboxes/{id}/transcripts` lists `.claude/projects/**/*.jsonl`
  and `.codex/sessions/**/*.jsonl` on the volume; `.../transcripts/{path}`
  streams one. Both read the host side of the volume
  (`$AGC_MSB_VOLUMES_DIR`, else `$MSB_HOME/volumes`, else
  `~/.microsandbox/volumes`), never the VM, so they work for stopped
  sandboxes and, by name, for deleted ones. Paths are confined to the
  transcript directories; `..` and links are refused.
- The fresh disk hides the image's `/home/agent`, so the git credential
  helper is in `/etc/gitconfig`, and the boot seeds `/etc/skel` and hands
  the disk to the sandbox user (as root) before cloning.

## Rationale

- Creating a task is now a row insert, which ends the create timeout without
  a workaround.
- Retry is a second sandbox; a human task is a task with no sandbox; both fall out
  of the model instead of needing features.
- The brief gives the agent its prompt through agc instead of through
  `setup`, and gives humans something to read in the panel.
- Keeping sandbox state live from the runtime and task state computed keeps the
  ADR 0003 principle: nothing mirrored that can drift.
- Not storing work keeps agc a coordinator; git and the channel already
  persist everything a task produces, as the first real run showed.

## Alternatives considered

- **Keep one table, make create asynchronous, add a brief.** Smallest change,
  but no retries and no human tasks; the first human-held task would force
  this split anyway.
- **Merge tasks with knowledge actions.** An action says why something should
  be done; a task has sandboxes, a channel and an assignee. A task's `for`
  points at the action instead.
- **Snapshots or file sync to save a sandbox's work.** Machinery for a problem
  the pushed-branch rule already solves. Revisit when a task needs an
  artifact that cannot live in git.

## Consequences

- The REST API, CLI, OpenAPI spec and panel change together on the feature
  branch; nothing aliases the old task-as-VM routes.
- The Work mode gets a Compute page listing sandboxes with their task, and the
  task page shows the brief, its sandboxes with terminals, and the channel.
- Messaging participants still carry `task`; a sandbox joins the task's channel
  under its own name.
- Sandbox labels gain `agc.sandbox`; `agc.task` stays for restarts after a host
  reboot (ADR 0003 item 8), which now re-creates sandboxes, not tasks.
