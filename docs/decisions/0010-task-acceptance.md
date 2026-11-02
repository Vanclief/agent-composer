# ADR 0010: A task is done when someone accepts it, and acceptance stops its compute

**Status:** Accepted — 2026-11-02. Builds on [ADR 0008](0008-tasks-allocate-compute.md) and [ADR 0009](0009-task-agents-and-managers.md).

## Context

ADR 0008 made `done` a flag anyone can set with one call, and ADR 0009 let
the manager or owner set it by posting `/done`. Nothing says what done
means for a given task, the agent has no way to say "I think this is
finished", and a finished task leaves its sandboxes running. Franco's
framing: a task carries the criteria it must meet; the agent, the manager
or Franco himself (from a direct message) accepts it against those
criteria; once accepted the task stops, its compute is stopped, and the
task itself persists so what happened can be read later and a VM revived.

## Decision

1. **A task says what accepted means.** Task gains `accept`: an ordered
   list of criteria, each a short sentence in the imperative or the
   indicative ("`npm run check` passes", "the Stop icon is filled"). It comes
   from `accept:` in `task.yaml` and `accept` on `POST /api/tasks`, may be
   empty, and is shown in `TASK.md` under a heading "Accepted when", one
   bullet per criterion, after the brief. Criteria are text agc does not
   evaluate; a criterion that can be checked by a command may later carry a
   `check:` like a knowledge rule (ADR 0006), and nothing here prevents that.

2. **The agent delivers; it does not finish.** From inside the sandbox,
   `agc task deliver <summary>` posts "Delivered: <summary>" in `#<task>` and
   sets the task's `delivered_at` and `summary`. The `agc agent run` loop
   (ADR 0009) calls it itself when a turn's answer starts with `DONE:`, with
   the rest of the answer as the summary. Delivering changes nothing about
   the sandbox: the agent keeps waiting on the channel for review.

3. **Status gains `review`.** Computed as before, never stored: `done` when
   `done_at` is set, else `review` when `delivered_at` is set, else `running`
   when a sandbox is running, else `open`. A task can be delivered more than
   once; the last `summary` wins.

4. **Accepting is one act with four doors.** `POST /api/tasks/{id}/accept`
   with `{ "by": "<name>" }` (default: the task's owner), `/accept` posted
   in `#<task>` by the manager or the owner, `/accept <task>` posted by the
   owner in the owner's own inbox (`#franco`), and the Accept button in the
   panel. All four set `done_at` and `accepted_by`, post
   "Accepted by <name>." in `#<task>`, and then stop every `running` or
   `starting` sandbox of the task through the same path as
   `POST /api/sandboxes/{id}/stop` (a `starting` one as soon as its boot
   finishes, keeping its disk). The agent loop exits on "Accepted by" or
   the stop, with no closing turn: the last delivery's summary is the
   record. Nothing is
   deleted: the task row, its criteria, summary, messages and sandbox rows
   stay, and a stopped sandbox can be started again (ADR 0008 item 4).

   `POST /api/tasks/{id}/done` and the `/done` command are replaced by
   `accept`; `agc task done` becomes `agc task accept`. A task may be
   accepted without having been delivered (the owner closing it by hand);
   `delivered_at` stays null then.

5. **Rejecting sends it back.** `POST /api/tasks/{id}/reject` with
   `{ "by", "why" }`, or `/reject <why>` in `#<task>` by the manager or the
   owner, clears `delivered_at`, posts "Rejected by <name>: <why>" in the
   channel, and the agent takes that message as its next turn. `summary` is
   kept so the history is readable.

6. **The panel shows the criteria and the verdict.** The task page's Brief
   tab lists the criteria as a checklist (unchecked boxes: agc does not
   evaluate them), the status chip shows `review` verbatim, and a task in
   review shows Accept and Reject (with a reason) in its actions; the New
   task dialog's Order card gets an "Accepted when" field, one criterion per
   line. A done task shows "Accepted by <name> <when>" and its summary.

## Rationale

- Done has to mean something before an agent can be trusted to say it.
  Putting the criteria on the task, in the agent's own instructions, makes
  the agent and the acceptor read the same list.
- Delivery and acceptance are different acts by different parties; the
  `review` status is that gap made visible. It also gives Franco the place
  to accept from his inbox, which is where escalations already arrive.
- Stopping, not deleting, keeps ADR 0008's promise that nothing in the
  sandbox is silently lost, and keeps the cost of an accepted task at zero.

## Alternatives considered

- **An Agent entity that owns tasks, to persist what happened.** The task
  already persists everything that happened: its channel, its sandbox rows
  with their specs, its summary and criteria; a stopped sandbox is revived
  with `start`, and a task may hold several sandboxes (ADR 0008). An agent
  that carries memory from one task to the next is a different need and
  gets its own ADR when it arises.
- **agc evaluates the criteria.** Acceptance by command exists already as
  knowledge rules; wiring them to tasks is a later step, not a reason to
  block acceptance by a person or a manager agent today.
- **Accepted tasks delete their sandboxes.** Rejected: ADR 0008 item 6.

## Consequences

- Task gains `accept` (JSON array of strings), `delivered_at`, `summary`,
  `accepted_by`; migration adds the columns. `status` adds `review`.
- Routes: `POST /api/tasks/{id}/accept`, `POST /api/tasks/{id}/reject`;
  `POST /api/tasks/{id}/done` is removed. Guest CLI: `agc task deliver`.
  Channel commands: `/accept`, `/reject <why>`, and `/accept <task>` in an
  owner's inbox; `/done` is gone.
- The agent loop treats a `DONE:` answer as a delivery and keeps waiting.
- `task.yaml` gains `accept:`; the panel gains the criteria field,
  checklist, `review` chip, Accept and Reject.
