# ADR 0012: Schedules create tasks from a template, once per new item a poll finds

**Status:** Accepted — 2026-11-02. Builds on [ADR 0008](0008-tasks-allocate-compute.md), [ADR 0009](0009-task-agents-and-managers.md) and [ADR 0010](0010-task-acceptance.md).

## Context

Some work recurs or arrives from outside: prwatch, Franco's background
GitHub service, polls every two minutes for open pull requests labeled
`ready-for-review` and runs a review worker per PR. Moving that into agc
means agc must create tasks on its own, on an interval, for things a
command finds, without creating the same task twice. The review itself is
a task like any other and is out of scope here.

## Decision

1. **A schedule is a task template on an interval.** A schedule has a
   unique `name` (task name rules), `every` (a Go duration, at least `1m`),
   `enabled` (default true), `task` (the fields of a `task.yaml`, stored as
   JSON; `name` optional) and an optional `poll: { command, secrets }`.
   `last_run_at` (when the last tick started, failed ones included) and
   `last_error` are stored; `next_run_at` = `last_run_at` + `every` is
   computed, and a schedule that never ran is due at once.

2. **A tick without a poll creates one task.** Its `title`, `name`, `brief`
   and each `accept` entry are Go templates with `{{.Now}}`; the name
   defaults to `<schedule>-<yyyymmdd-hhmm>` (UTC).

3. **A tick with a poll creates one task per new key.** agc runs the
   command on its host with `sh -c`, a 60s timeout and an environment of
   only `PATH`, `HOME` and the named secrets from the secrets file (ADR
   0004's file, read as values here because the command runs on the host,
   not in a sandbox). Each stdout line is a JSON object of strings with a
   non-empty `key`. A key not yet in `schedule_items` renders the template
   over the item's fields (`Review {{.url}}`) and creates the task through
   the tasks API exactly as `POST /api/tasks` does; the item is recorded
   with the task. A task without a template name is
   `<schedule>-<8 hex of sha256(key)>`; one without a template title takes
   the item's `title`, so the list reads as the PR, not the hash. A failing command or bad output sets
   `last_error` and creates nothing; a task that cannot be created is left
   unrecorded, so the next tick tries it again. Secret values are redacted
   from `last_error`.

4. **Only `agc rest` runs schedules.** Its scheduler checks every 30s and
   ticks each enabled, due schedule in its own goroutine. Ticks of one
   schedule never overlap: one that is due while the last still runs is
   skipped, and `POST /api/schedules/{id}/run` answers 409 then. Locally,
   `agc schedule run` is the only way a schedule runs.

5. **A template is checked when it is saved.** Create and replace render
   it with every item field it reads set to `sample` and reject it unless
   the task passes `ApplyDefaults` and `Validate`, so a schedule cannot
   fail every tick on a typo. At run time a field the item lacks is an
   error, not empty text.

6. **Deleting a schedule keeps its tasks.** Its items go with it.

## Rationale

- A task per item, keyed by what the poll says identifies it, is what
  prwatch does; a key of URL plus head SHA reviews each push once.
- The poll is a shell command, not a GitHub integration: `gh` already
  speaks GitHub, and any other source is one command away.
- Creating through the tasks API keeps one path for defaults, the opening
  message and allocation (ADR 0008, 0009).

## Consequences

- Tables `schedules` and `schedule_items` (unique on schedule and key),
  added by migration `20261102200000_schedules`.
- Routes: `GET/POST /api/schedules`, `GET/PUT/DELETE /api/schedules/{id}`,
  `POST /api/schedules/{id}/run`, `GET /api/schedules/{id}/items`. CLI:
  `agc schedule create|list|show|run|enable|disable|delete`, local or
  against `AGC_SERVER`. Example: `docs/schedules/pr-reviews.yaml`.
- `docs/schedules/pr-reviews.yaml` is the production review schedule
  (`Vanclief/compose` first): each task, managed by `reviewer-1` with the
  `pr-review` skill, posts the review itself as one comment with `gh` and
  moves the pull request's label, replacing prwatch.
- Every tick publishes `schedule.ran {schedule, created}` on the events bus
  (`GET /api/events`).
