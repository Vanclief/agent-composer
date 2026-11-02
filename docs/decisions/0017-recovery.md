# ADR 0017: agc restarts at boot and brings its sandboxes and agents back

**Status:** Accepted — 2026-11-02. Builds on [ADR 0008](0008-tasks-allocate-compute.md), [ADR 0009](0009-task-agents-and-managers.md), [ADR 0010](0010-task-acceptance.md) and [ADR 0013](0013-watchdog.md).

## Context

On 2026-11-02 Franco powered the rig off and on. What came back:

- Nothing restarted: neither the msb server nor `agc rest`. Both had been
  started by hand or by `scripts/rig-deploy.sh` with `nohup`.
- Every VM that had been running came back as `crashed` in msb.
- The named volumes `agc-<sandbox>`, with the harness homes and the
  transcripts (ADR 0008), were intact.
- agc's resume pass (`sandboxes.Resume`) re-attached messaging only to VMs
  still running and skipped the rest.
- A task whose VM crashed showed as open, and nobody was told.

The watchdog (ADR 0013) looks only at sandboxes the runtime reports
`running`, so a crashed one was invisible to it too. And an agent started
again from its sandbox's command would have read TASK.md and begun from
zero, although its harness session was still on the volume.

## Decision

1. **agc and msb are systemd user units on the rig.**
   `scripts/rig-deploy.sh` writes `~/.config/systemd/user/agc-msb.service`
   (`msb $AGC_RIG_MSB_SERVER`, default `server start`, in the foreground)
   and `agc-rest.service` (`agc rest` in `$AGC_RIG_RUN`, with the same
   `AGC_KNOWLEDGE_DIR` and `AGC_DEFAULT_OWNER` the script passed to
   `nohup`, `After=` and `Wants=agc-msb.service`), both
   `Restart=on-failure` and `WantedBy=default.target`. A unit carries
   paths and those two settings, never a secret: agc reads its secrets
   from its own files. Both use `KillMode=process`: the VMs `msb start`
   boots for agc stay in the cgroup of the process that started them, and
   the default `control-group` would take every running VM down with each
   restart of agc.

2. **The units run only with lingering.** Without lingering the user
   manager starts at the first login and stops when the last session ends,
   taking its units along; an `agc rest` run by the unit would die when the
   deploy's SSH session closes. So the deploy uses the units (`enable`,
   `systemctl --user restart agc-rest`) only when `systemctl --user` works
   and `loginctl show-user` says `Linger=yes`. Otherwise it writes the unit
   files without enabling them, starts agc with `nohup` as before, and
   prints the one command that is left:

       sudo loginctl enable-linger <user>

   Franco runs it once in a terminal on the rig; an agent never does
   (`rig-sudo-by-franco`). The next deploy enables the units. A deploy
   never restarts msb (its VMs would go down); it starts `agc-msb` only
   when no msb server runs, so one started by hand keeps running and the
   unit takes over at the next boot.

3. **The server restarts what went down while it was off.** Only `agc
   rest` does this, in `Stack.StartScheduler` before the
   watchdog starts; a one-shot CLI command or the MCP server still only
   re-attaches listeners. `sandboxes.Resume(ctx, restart)` starts again
   every sandbox that is *recoverable*: it booted, agc never stopped it
   (`stopped_at` is empty), its task is not done, and msb reports it as
   anything but `running` (after a reboot, `crashed`). `sandboxes.Restart`
   does what `Start` does (secrets resolved again, listener attached,
   runtime started, `stopped_at` cleared), then starts the sandbox's
   `command` in a terminal, as its boot did, unless a job with that
   command already runs: msb does not restart jobs. agc then posts in
   `#<task>` and `#<owner>`: "recovery came back after a restart; its agent
   continues." A sandbox that cannot start is logged, posted once ("…
   did not come back after a restart: sandbox … is crashed and could not
   start: …") and skipped. A task with two such sandboxes gets one line
   for each. A sandbox whose image predates `agc agent run` (built before
   2026-11-01 evening) is started but its agent is not: `Restart` asks the
   guest agc for `agent run --help` first, and the line says "… came back
   after a restart, but its agent was not relaunched: the sandbox's image
   predates `agc agent run`."

   A crashed VM that agc then stops reads `stopped`, not `crashed`: msb
   keeps reporting `crashed` after the stop, and `stopped_at` says agc
   stopped it (observed by claude_mac restarting by hand after the
   reboot).

4. **The watchdog restarts a crashed sandbox, a few times an hour.** A new
   condition in `core/watchdog`: a recoverable sandbox msb reports
   `crashed` is restarted with `sandboxes.Restart`, at most
   `recovery.restarts` times within an hour per sandbox (default 3). Each
   try is posted in `#<task>` and `#<owner>` ("x crashed; agc restarted it
   (restart 2 of 3 this hour) and its agent continues.", or that it could
   not, with the error). Past the limit it gives up and says so once, with
   the reason, and leaves the sandbox alone until it is seen running again.
   The count is in process; a restart of agc starts it over, and the
   startup pass of item 3 is a try of its own. The silence check of ADR
   0013 still covers an agent that is up but stuck.

5. **The agent continues its session.** `agc agent run` saves its harness
   session id after every turn in `~/.harness/agc-agent.json`
   (`--state`, `task.HarnessDir`, on the volume next to `~/.claude` and
   `~/.codex`), with the harness, the task, the last message it handled in
   `#<task>` and whether it delivered. When it starts and the file holds a
   session for the same harness and task, it resumes it (`claude --resume
   <id>`, `codex exec resume <id>`, as it already does between turns) with
   one line as its turn: "agc restarted your sandbox; continue where you
   were; the repo clone may need a fresh pull." It then reads `#<task>`
   from its saved cursor, so a message posted while it was down is its next
   turn; its own earlier posts are not. If it had delivered, it takes no
   turn and waits for review. If the harness cannot resume the session, it
   starts over on TASK.md. Only without the file does it run on TASK.md.

6. **Configuration.** `recovery: { restarts: 3 }` in the config file, read
   like `defaults:` (ADR 0009 item 1): `AGC_RECOVERY_RESTARTS` overrides it,
   `0` turns the watchdog's restarts off (the startup pass still runs), and
   an unreadable value is logged and the built-in used.

## Rationale

- The volumes survived the reboot; everything lost was process state agc
  can rebuild from its own rows. Restarting is cheaper than asking anyone.
- systemd user units are what the rig already has; no new daemon, no root.
  Lingering is the one thing that needs root, so it is printed, not run.
- `stopped_at` already records that agc stopped a sandbox on purpose; a
  sandbox down without it went down on its own, so the rule needs no new
  column and no migration.
- Each restart is said in the owner's inbox, where ADR 0013 already says
  what needs the owner; a limit and one "gave up" line keep a crash loop
  from becoming a digest nobody reads (`unread-digests-remove-the-human`).
- Every check is mechanical: a status word, a timestamp, a count
  (`llms-are-parsers-not-sensors`).
- Resuming the session keeps what the agent already knew and did; starting
  from TASK.md would redo work and repeat messages in `#<task>`.

## Alternatives considered

- **Restart every sandbox not running, `stopped_at` or not.** A sandbox
  someone stopped, or that acceptance stopped (ADR 0010), would come back.
- **Have msb restart its VMs.** After the reboot it brought them back
  `crashed`, not running; and a VM msb restarted would still need its
  secrets resolved again, its listener and its agent job, which only agc
  has.
- **A system unit (root).** Needs sudo for every change of the unit; the
  user units change with each deploy.
- **Store the session in agc's database.** The harness session lives on
  the sandbox's volume anyway; agc would have to be told it over the
  socket and would hold a value only the sandbox can use.

## Consequences

- `sandboxes.API` gains `Restart`, `Status` and `Recoverable`; `Resume`
  takes `restart`. `watchdog.Probe` reads `Status` instead of `Running`
  and gains `Restart`; `watchdog.Settings` gains `Restarts`.
- `controller.Config` gains `recovery`; `Stack` gains `Sandboxes`.
- `agc agent run` gains `--state`. Sandboxes run the agc of their image:
  rebuild `images/agc-agent` for running agents to save and resume.
- `scripts/rig-deploy_test.sh` (`make rig-deploy-test`) checks the units
  and the restart path against stub commands; CI does not run it yet,
  since adding a workflow step needs a token with `workflow` scope.
- A crash during an agent's very first turn leaves no session id (the
  harness reports it only when the turn ends), so that agent starts over
  on TASK.md.
- A message the agent was answering when it crashed is answered again
  after the resume line.
- Not tested on the rig: a reboot with lingering on, whether `msb server
  start` stays in the foreground, and whether VMs survive `systemctl
  --user restart agc-rest` with `KillMode=process`.
