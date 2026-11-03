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

   Accepting also closes the task's questions (added 2026-11-03, task
   `close-done-questions`): agc replies `Closed: <task> accepted by <who>.`
   in the thread of every open question in `#<task>` and of every open
   question in `#<owner>` from the task's manager whose body starts with
   `Accept <task>?` or `<task>:`, which answers each for
   `GET /api/msg/questions` (`messaging.Service.CloseQuestions`); no column
   or state is added. A question a person asked is never closed, and
   rejecting closes nothing. The collector's tick in `agc rest` (ADR 0008
   "Garbage collection") does the same for every done task, logging a line
   per question, so questions left open by an accept before this change,
   or by a failed reply, close on the next tick.

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

## Delivery report

Added 2026-11-02. A delivery may carry `report`, a JSON object, so the
reviewer sees what they are accepting without reading the channel. It is
optional on `POST /api/tasks/{id}/deliver` and on `POST /task/deliver` on
the sandbox socket (passed through unchanged), and is stored on the task
(`report`, jsonb, next to `summary`; `TaskView.report`, null when absent):

```json
{
  "criteria": [ { "met": "yes", "note": "GET /api/agents returns the list; test TestListAgents" } ],
  "changes":  { "branch": "agc/agents-core", "sha": "efbec13", "files": 41 },
  "checks":   [ { "name": "go test ./...", "passed": true, "note": "" } ],
  "left_out": [ { "what": "DM channel for agents", "why": "needs ADR 0011, TODO left in agents/wake.go" } ],
  "needs":    [ "Decide whether never-kept agents are swept" ]
}
```

Rules, checked on the request and in the model, any break is `EINVALID`:

- `criteria` has exactly one entry per line of the task's `accept`, in the
  same order, and none when the task has none; a mismatch names both counts.
- `met` is `yes`, `no` or `partly`; `note` is at most 500 characters and may
  be empty.
- `changes` is optional; when given, `branch` is required, `sha` is 7 to 40
  hex characters, and `files` is optional and not negative.
- `checks[].name` is required, `passed` is a boolean, `note` ≤ 500.
- `left_out[].what` and `why` are required; `needs[]` are non-empty strings.
- Every list holds at most 50 entries. Strings are trimmed.

Each delivery replaces the report with its own (null when it sends none),
so a report never describes an earlier delivery; a rejection keeps it,
like the summary. The summary stays one or two sentences, read first; the
report is what the reviewer judges. The "Delivered:" message renders it
after the summary: `- [x]`, `- [ ]` or `- [~]` (yes, no, partly) and the
criterion's text with its note, `Changes: <branch> @ <sha> (<files> files)`,
`Checks:` with ✓ or ✗, then `Left out:` and `Needs you:` as bullets, empty
sections omitted. TASK.md's "Delivering" section tells the agent to write
`report.json` and run `agc task deliver "<summary>" --report report.json`;
an answer starting `DONE:` still delivers the summary alone, the fallback
when no report can be written.

Shots, added 2026-11-02: the owner judges a UI change by looking at it, not
by reading code. A report may carry `shots`, at most 8 of
`{ label, file }`: `label` a slug unique in the report (`before` and
`after` for a UI change), `file` the PNG's path where the agent took it.
The PNG travels with the delivery in `files`, base64 under its label, in
the same JSON body on `POST /api/tasks/{id}/deliver` (whose body cap is
raised to 24 MB for it); `agc task deliver --report` reads each file and
sends it. Inside a sandbox no request body over 256 KiB crosses the vsock
socket (ADR 0005 item 2), so there `agc task deliver` first sends each shot
as raw PNG pieces of at most 192 KiB to the socket's `POST
/task/shots/{label}?offset=<n>` (offset 0 starts the shot over; the socket
keeps them in memory, at most 8 of 2 MB), then `POST /task/deliver` with
no `files`, which takes the shots its report names from those, and forgets
them once delivered (added 2026-11-03). A request that makes no progress
for 60 seconds, sending or awaiting the answer, fails naming the step and
the size (`sending shot before (1.5 MB, piece at 0 B) to the sandbox
socket: no answer for 1m0s`), and the socket logs a body that takes longer
than 10 seconds to read. Each must be a PNG (by its
signature) of at most 2 MB, one per shot and no other, else `EINVALID`.
agc keeps them on the host under `<agc home>/shots/<task>/<label>.png`
(files 0600, directories 0700), never reads them through the VM again,
serves them at `GET /api/tasks/{name}/shots/{label}.png` (`image/png`, 404
when absent), and stores the shot on the report with `file` set to
`<label>.png` and `url` to that route, under `AGC_PUBLIC_URL` when set,
else root-relative. No table holds them: the report names them. Each
delivery replaces the task's shots (one without shots removes them); a
reject or an accept keeps them. The Delivered message adds a
`<Label>: <url>` line per shot after the checks. TASK.md's "Delivering"
asks for `before` and `after` on a change a person sees; the reviewer adds
`Before:` and `After:` lines to its question to the owner, which the panel
renders as images, and rejects a UI change that has no shots.

## Who merges

Added 2026-11-02. A delivery that carries a branch (`changes.branch` and
`changes.sha` in the report) is merged by the task's manager, not by the
owner, unless the rule `merge-needs-franco-for-ui-or-breaking`
(`rules/repo.yaml`) fails:

1. **The manager judges and checks.** On delivery the manager judges the
   rules on the diff from the task's base to the delivered sha,
   `git diff <base>...<sha> | agc rules judge --as claude_code --on -`, and
   runs the repository checks (`go build ./... && go vet ./... &&
   go test ./...`, `gofmt -l .`, `go run . rules check rules/`, and
   `make panel-check` when `panel/` changed).
2. **Nothing to show, nothing broken: the manager merges.** When every
   check passes, no judged rule fails and
   `merge-needs-franco-for-ui-or-breaking` holds, the manager merges the
   branch into the base, pushes the base and accepts the task. A failing
   check or another failing rule is a rejection with the reason, as in
   item 5.
3. **A UI or breaking change: the owner is asked.** When that rule fails
   or its verdict is unclear (unclear counts as "needs Franco", never as a
   pass), the manager posts one question in `#<owner>` with the judge's
   reason and the compare link for `<base>...<sha>`, and the task waits on
   the owner in `review`.
4. **The owner's verdict.** There, the owner's accept (any of item 4's
   doors) means approved: the manager merges the branch and pushes after
   it. The owner's reject sends the task back to the agent with the reason,
   as in item 5, and nothing is merged.

The manager is the owner's only interlocutor for a task: the task's agent
is not granted `#<owner>` (ADR 0009 item 5), so a question it has for the
owner goes to its manager in `#<task>`, and the manager raises it in
`#<owner>` as item 3 does.

The rule applies to merges into our repositories only: a task that only
posts on a pull request elsewhere (a review, a label) has no branch to
merge, so its manager accepts it without asking the owner.

`accept` itself is unchanged: it still only sets `done_at` and
`accepted_by` and stops the task's sandboxes; merging is the manager's act
around it, and deploying to the rig stays a hand step (`make rig-deploy`).
The owner is asked only for UI and breaking changes because his review time
goes to what he can see and to what can break others, while style and
correctness are covered by the mechanical rules and the tests (belief
`franco-reviews-what-he-sees-and-what-breaks`).
