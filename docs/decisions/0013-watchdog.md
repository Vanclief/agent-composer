# ADR 0013: A mechanical watchdog tells the owner when a running task is stuck

**Status:** Accepted — 2026-11-02. Builds on [ADR 0005](0005-messaging-transport-and-identity.md), [ADR 0008](0008-tasks-allocate-compute.md), [ADR 0009](0009-task-agents-and-managers.md) and [ADR 0010](0010-task-acceptance.md).

## Context

On 2026-11-02 the task `dms-whatsapp` sat idle for about 52 minutes
because its command had been split on whitespace, and `yc-pitch` sat on a
permission prompt. Nothing in agc noticed; a session polling a terminal by
hand did. ADR 0009 gives every task a channel, an owner with an inbox and
a reserved sender `agc`, so the owner can be told where escalations
already arrive. Franco's framing: the owner must be told, in agc's own
messaging, by a check that needs no model.

## Decision

1. **A loop in the agc server, every minute.** `core/watchdog` runs next to
   the scheduler (`Stack.StartScheduler`, so only `agc server` runs it, never
   a one-shot CLI command). Each minute it reads every task that is not
   done and every sandbox of it that has booted, has a `command` (the job
   ADR 0009 calls `agent`) and that the runtime reports `running`. A stopped
   or failed sandbox is someone's decision or already says so; it is not
   watched.

2. **Every signal is derived, none stored.** No column and no migration:
   - *Last terminal output*: msb keeps every job's output in a log with a
     timestamp per chunk. The sandbox executor gains `ListJobs` (all jobs,
     finished ones included, with exit codes) and `TailJob` (the last
     chunks of one job's log) behind `executor.JobWatcher`, and
     `sandboxes.API.AgentJob` picks the newest job whose command is the
     sandbox's `command` and returns when it last wrote and its last
     non-blank line, terminal escapes stripped. Nobody attaches to the
     terminal or buffers it.
   - *Last message in `#<task>`*: the newest row in `messaging_messages`
     not posted by `agc` (the opening, deliveries and the watchdog's own
     posts are not activity), and whether it came from the task's own
     sandbox (`task` on the message).
   - *Exit*: the agent job's state and exit code from the runtime.
   - *Review*: `delivered_at` on the task (ADR 0010).

3. **Three conditions, each one sentence.**
   - **silent**: the agent job runs, the task is not in review, it has
     written nothing to its terminal and nobody but agc has posted in
     `#<task>` for `silence`, and the last word in the channel is not the
     agent's own (an agent that answered and waits on its manager is not
     stuck). "dms-whatsapp has been silent for 52 minutes. Last terminal
     line: `>`"
   - **exited**: the agent job has exited and the task is not done.
     "dms-whatsapp's agent exited with code 2 12 minutes ago and the task
     is not done. Last terminal line: `…`"
   - **review**: the task has been delivered for `review` without an
     accept or a reject. "dms-whatsapp has been in review for 2 hours 5
     minutes, waiting on @manager or @owner to /accept or /reject it."

4. **agc posts it in `#<task>` and in `#<owner>`.** As a plain message
   through `messaging.PostAsSystem` (ADR 0009 item 4), so it is in the
   task's history and in the owner's inbox, where escalations already
   arrive; the manager reads `#<task>`. A task with no owner gets only
   `#<task>`. Each post also publishes `watchdog.alert` on the event bus
   (`GET /api/events`) with the kind, task, sandbox, since and body, and
   `cleared: true` when the condition stops holding. No panel UI reads it
   yet.

5. **At most once an hour per condition; silent once it clears.** While a
   condition holds it is posted again only after an hour. When it clears
   (output, a message, an accept or reject, a stop, the task done) the
   watchdog forgets it, so a recurrence is posted at once. The memory is
   in process; after a restart the channel is the memory: a condition is
   not posted again if agc posted the same sentence opening in `#<task>`
   within the hour.

6. **Thresholds are configuration.** `watchdog: { silence: 15m, review: 2h }`
   in the config file, read like `defaults:` (ADR 0009 item 1):
   `AGC_WATCHDOG_SILENCE` and `AGC_WATCHDOG_REVIEW` override it, the
   built-ins are 15m and 2h, `0` turns a check off, and an unreadable
   duration is logged and the built-in used.

7. **Drift from the brief is not checked here.** Whether an agent is still
   doing what its brief asks is a judgment, not a mechanical check; when it
   is built it is a judged rule (ADR 0006, `judge:`), asked of a trusted
   harness and recorded as an opinion, never posted as if agc had measured
   it.

## Rationale

- Both incidents were visible in signals agc already had (a quiet terminal,
  a quiet channel); what was missing was someone looking. A loop over
  existing state is the smallest thing that looks.
- Deriving from msb's job log and the messages table keeps ADR 0003's
  principle: nothing mirrored that can drift, and no migration.
- No model: a model asked "is this agent stuck?" is a sensor, and an LLM is
  a terrible sensor (`llms-are-parsers-not-sensors`). Every sentence the
  watchdog posts is a fact with a number in it.
- The owner's inbox is where ADR 0009 already sends what needs the owner,
  so the watchdog adds no channel, no notification system and no UI.
- Once an hour, not once a minute: a digest nobody reads is how a human
  quietly leaves the loop (`unread-digests-remove-the-human`); a repeat
  every minute would be that digest.

## Alternatives considered

- **Record `last_output_at` on the sandbox row.** agc would have to stay
  attached to every terminal to update it, and the value would be stale
  whenever agc is down. msb already keeps it.
- **Watch the harness transcripts on the volume (ADR 0008).** They say what
  the harness did, not whether the job is alive, and a custom `command` has
  none.
- **Post only in `#<task>`.** The manager might be the agent that is stuck
  waiting, or a person who is away; the owner is the one who must hear.
- **Use direct message channels or presence.** Both live on unmerged
  branches (`agc/dms-whatsapp`, `msg-presence`); the watchdog uses only
  `#<task>` and `#<owner>`, and can move to a DM channel once one exists.

## Consequences

- `executor` gains `Job`, `JobOutput` and the optional `JobWatcher`; the
  sandbox executor implements it (a stub without cgo).
- `sandboxes.API` gains `AgentJob` and `Running`; `core/watchdog` is new;
  `Stack` gains `Watchdog`; `controller.Config` gains `watchdog`.
- The event stream gains `watchdog.alert`; `docs/openapi.yaml` lists it
  together with the panel work that reads it (action
  `panel-shows-watchdog-alerts`), since documenting it regenerates the
  panel's types.
- msb's job API does not document its timestamp unit; `sandbox.UnixTime`
  reads it by magnitude. Check it on the rig against a real job.
- An agent waiting on its manager for hours is not reported; a reply the
  manager owes is a question (`agc msg ask`), which the inbox already
  lists.
