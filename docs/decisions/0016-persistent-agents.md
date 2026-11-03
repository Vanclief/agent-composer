# ADR 0016: Agents persist across tasks, sleep by default and wake when spoken to

**Status:** Proposed — 2026-11-02. Builds on [ADR 0005](0005-messaging-transport-and-identity.md), [ADR 0008](0008-tasks-allocate-compute.md), [ADR 0009](0009-task-agents-and-managers.md) and [ADR 0010](0010-task-acceptance.md). ADR 0015 (the `main` router) is written on top of this one.

## Context

A task agent (ADR 0009) lives as long as its task: it is born with the
sandbox, talks in `#<task>`, delivers, and its VM is stopped on accept. That
is right for work with an end, and wrong for the other thing Franco wants:
someone to talk to across days, who remembers what was said, manages tasks
on his behalf, and whom he can open as many threads with as he likes. His
constraints, in his words: "I may end up needing to talk to multiple
agents", "the VM should turn on when I talk to the agent and he has things
to do, not always", templates "like Reviewer", a unique name per agent,
"use numbers until I want to preserve an agent and then I can give it
another role or rename it", and the harness conversation logs kept because
they are "the source of truth".

The panel already has the Agents page against a mock (`panel/mock/server.ts`,
"agents" section) and its types in `panel/src/api/agents.ts`; this ADR gives
that mock a backend and settles what it left open.

## Decision

1. **An agent is a row, not a process.** Table `agents`: `name` (unique),
   `role`, `number`, `kept`, `harness`, `model`, `owner`, `home` (the home
   sandbox's name, empty until the first wake), `idle` (duration), `created_at`,
   `last_active_at`. Nothing runs for an agent that nobody is talking to.

2. **Templates are files; the role is the template's name.** A template is
   `agents/<role>.yaml` under the agc home (`AGC_AGENTS_DIR` overrides, like
   the knowledge dir in ADR 0006): `harness`, `model`, `sandbox` (the spec
   of ADR 0008, `disk` included), `idle` (default `15m`), `reads` (channel
   patterns the agent watches besides its own; `main` lists `*`), `rules`
   (ids of rules in the knowledge dir of ADR 0006, checked on every read of
   the template, so an unknown id fails create and wake naming it and the
   known ids, and shown in the agent view as `rules: [{ id, rule }]`), and
   `instructions`, the text the agent gets as its AGENT.md the way a task
   gets TASK.md (ADR 0009 item 3). `POST /api/agents { role }` creates
   `<role>-<n>`, `n` the next number for that role, never reused even after
   a rename, so the list stays readable while the agent is on trial.
   Templates name no harness credential, only `GITHUB_TOKEN`, and rely on
   the provider's default account (ADR 0018).
   An optional `effort` (`low`, `medium`, `high` or `xhigh`, empty for the
   harness's default) is copied to the row like `model` (`POST /api/agents`
   may override it) and reaches the harness through `agc agent run
   --effort`: Claude Code's `--effort`, Codex's `model_reasoning_effort`.
   `sandbox.skills` names skills of agc's skills repository, written into
   the home on every boot and wake (ADR 0008 "Skills"); the agent view
   carries them as `skills`, read from the template on every request.
   The repo ships `agents/reviewer.yaml` (reviews code and manages code
   tasks, merging what it may), `agents/main.yaml` (reads every channel,
   ADR 0015), `agents/developer.yaml` (an agc developer that manages its
   own tasks) and `agents/designer.yaml` (a UI/UX designer for the panel
   who proposes changes to Franco, makes the agreed ones into tasks it
   manages, and merges them only after Franco accepts).

3. **Keeping names it.** `POST /api/agents/{name}/keep { name, role? }` sets
   `kept`, renames the row and may change the role. The home sandbox and the
   volume keep their old names: a rename is a label, not a move. Nothing is
   deleted automatically: a never-kept agent sits in the list as `asleep`
   until `DELETE /api/agents/{name}`, which deletes the row and the home
   sandbox and keeps the volume (item 5).

4. **The home sandbox is one VM, stopped by default.** The first wake
   allocates a sandbox named after the agent from the template's `sandbox`,
   with the repo and secrets the template names; later wakes start it again
   (ADR 0008 `start`), which relaunches the home's job, `agc agent run
   --agent`, on every start, since a boot from disk runs no job. The state the panel shows is computed from the home
   sandbox, never stored: `asleep` when there is no home or it is `stopped`,
   `waking` while it is `starting`, `awake` while it is `running`, and the
   sandbox's own `failed` when it died. `POST /api/agents/{name}/wake` and
   `/sleep` do what the names say, `sleep` posts "Asleep." in the agent's DM
   channel so the thread shows where the VM stopped.

5. **Memory is the volume, and the transcript is the record.** The home
   sandbox gets the named volume `agc-<name>` of ADR 0008 "Disk and
   transcripts", so `~/.claude` and `~/.codex` outlive the VM and the
   sandbox. The volume is the agent's, not the home's: every home of the
   agent mounts `agc-<name>` whatever the sandbox is called, so a home
   allocated after the last one was deleted (named `<name>-2` because the
   kept volume holds `<name>`) reuses it, and the harness sessions,
   `agc-sessions.json` and `agc-cursor.json` carry over (seen on the rig,
   2026-11-02: a new home on `agc-<name>-2` replayed old turns). A task's
   sandbox keeps its own volume, named after the sandbox. The agent's memory files and every harness session live there;
   agc writes no second copy of anything. `GET /api/agents/{name}/turns`
   lists the agent's turns read from those session files, one per message
   batch it answered: `{ n, started_at, ended_at, channel, injected: [message
   ids], tool_calls, answer }` with `answer` the first line; `GET
   /api/agents/{name}/turns/{n}` returns the whole turn: the text injected,
   each tool call `{ name, args, result }`, the answer, and when the
   transcript has them the harness session id and the token counts, so cost
   is visible per turn. The panel shows them on the agent page as a Turns
   tab.

6. **A message wakes the agent; silence puts it to sleep.** An `OnPost` hook
   (ADR 0005) matches each message against every agent's channels: its DM
   channel `@<owner>/<name>` (the naming of ADR 0011), `#<task>` for every
   task whose `manager` is the agent, done ones included (a task stays in
   its manager's channels for good), and the template's `reads`. A match
   is a turn for the agent only by who posted it (`msgmodels.TurnFor`, the
   one rule the wake hook, the home loop and the task loop
   `agc agent run --task` share): an agent's own posts never are; a
   person's post (`human`) always is; another agent's post (not human and
   not agc: a task's sandbox or a home) is a turn only when it is a
   question (`kind: question`) or names the agent (`@<name>` in the body),
   so two agents never answer each other's acknowledgements in a loop
   (seen on the rig in `#secret-accounts`, 2026-11-02); such a plain post
   still advances the cursor and is shown with the next turn in its
   thread. A reply in a thread an agent started is a turn for that agent
   in any channel, read or not, by a person or another agent, never its
   own (`msgmodels.AnswerFor`): the asker always hears the answer, so the
   wake hook wakes it and `GET /agent` lists those channels as `asked`
   (where it started a thread outside its channels, in practice
   `#<owner>`), which the home waits on, is granted, and takes only the
   replies in its own threads from (seen 2026-11-02: franco's "Yes do it"
   to the reviewer's question in `#franco` woke nobody). agc's own posts wake nobody, except two in `#<task>` that wake
   the task's manager: `Delivered:` (a delivery to review) and
   `Accepted by <someone else>.` (the owner accepted what the manager
   asked about, so the manager merges it; the reviewer template); an awake
   home also takes `Rejected by` and an agc question naming it, and the
   task loop takes only `Rejected by`. The DM channel joins the list only when
   messaging knows direct-message channels (ADR 0011); until then messaging
   rejects `@` names and the list leaves it out. A match on
   an `asleep` agent starts the home (item 4); on an `awake` one it is picked
   up by the running loop. The loop is `agc agent run --agent <name>` inside
   the home, the job `agent` of ADR 0009. It joins messaging as the
   agent's `<name>`, never the home's name or `<name>-2`, with `agent`
   set to the home, since `TurnFor`, `@<name>` mentions and `Accepted by
   <name>.` all compare that name. When a participant of that name is
   still joined and messaging's presence says it is absent (its sandbox
   is gone or not running: a deleted home's), the join takes the name
   over and marks the old participant left; a human's name, or one whose
   sandbox is running, is refused (409) and the loop exits with the
   error. Only a home's join takes over; a task's sandbox still moves on
   to `<name>-2`. The loop reads "messages since my last
   turn" over the sandbox's messaging socket as records (`id`, `channel`,
   `from`, `kind`, `human`, `body`, `created_at`), the agent's own messages
   left out; how the records become a prompt is the role's business (ADR
   0015 for `main`). When no channel it reads has had
   a message for `idle`, the loop exits and agc stops the sandbox. Every
   state change publishes `agent.state { agent }` on the event bus
   (`core/events`), which the panel already consumes. The AGENT.md the
   home reads is the template's `instructions` ending, when the template
   names `rules`, with a "Rules you follow" section: each rule as
   `agc rules show` says it, with its id, in template order, then a line
   asking the agent to check its work against them before it answers or
   delivers.

7. **A thread is a session.** For an agent talked to in a DM, each thread
   root (ADR 0011) is one harness session, resumed by root id on every turn
   in that thread, so a long conversation keeps its context and two
   conversations do not bleed into each other. A message with no thread
   starts one. The `main` role instead keeps one long session over a batch of
   new messages across all its channels; how that batch is built, and what
   `main` may do with it, is ADR 0015. Merging two threads into one is not
   modeled: nothing here prevents it, and a thread's transcript is a file
   that can be read into another session later.

8. **An agent manages tasks; it does not do them.** An agent's tasks are the
   rows whose `manager` is the agent's name (ADR 0009), created by the agent
   with `agc task create` from its home, and shown on the agent page. The work
   still runs in task sandboxes with task agents; a persistent agent is the
   one who briefs, answers questions, accepts or rejects (ADR 0010), and
   talks to Franco about it. Like a task sandbox (ADR 0009 item 5), a home
   may post in its owner's inbox `#<owner>`, to ask there; the inbox is not
   in its channels, so a post there does not wake it. Whether an act needs
   a human yes is a gate of ADR 0007, settled per role in ADR 0015 for
   `main`.

9. **`main` reads everything.** One clause in `authorize` (ADR 0005): the
   participant whose agent has role `main` may read, wait on and post in
   every channel. Other agents follow the existing rules: their DM, the
   channels of their tasks, what `reads` lists.

## Notes on item 5 (task `agents-turns`, 2026-11-02)

- Built in `core/resources/agents/turns*.go`, for agents (`agc-<name>`)
  and, with the same reader, for task sandboxes
  (`GET /api/sandboxes/{id}/turns[/{n}]`). The contract differs from item 5
  in one place: `injected` is the user text as the harness recorded it, not
  message ids; the transcript does not carry agc's message ids. The loop can
  put them in the text it sends if they are wanted.
- Claude Code (2.1.x): a turn starts at a `type: user` line whose content
  is a string or text blocks (`isMeta`, `isCompactSummary`, `isSidechain`
  and "[Request interrupted by user" lines are not turns); `assistant`
  lines are one per content block and repeat their call's `message.usage`
  under one `message.id`, so usage is counted once per id; `tool_result`
  blocks pair with `tool_use` by id. Tested on a fixture cut from a real
  session.
- Codex (0.166): `session_meta`, `response_item` (`message`,
  `function_call`/`_output`, `custom_tool_call`/`_output`,
  `local_shell_call`) and `event_msg` `token_count`, whose running
  `total_token_usage` gives a turn's tokens as a difference. Only a session
  with no model reply could be produced here (no OpenAI credentials in the
  sandbox), so the user-turn and context-message handling is tested on a
  real file and the tool call, answer and token lines of the fixture are
  written from the format, untested against a real Codex run.
- `n` needs every session's turns (DM threads are concurrent sessions), so
  every file gets a light parse of line envelopes on each request and only
  the files holding the requested page are parsed in full.

## Rationale

- Rows not processes because a stopped agent costs nothing and the list of
  agents is then a list of names, not of VMs; waking from `stopped` is seconds
  on the rig (ADR 0008 start), which is within what a chat reply tolerates.
- State computed from the home sandbox, never stored, so the agent list can
  never disagree with the Compute list (the rule of
  `ui-shows-backend-status-verbatim`).
- Templates as files so a role is reviewed in git like a rule or a task.yaml,
  and the Reviewer that worked yesterday is the Reviewer that runs today.
- The volume as the one memory because it already exists for every sandbox,
  it holds the thing Franco called the source of truth, and a second store
  of "memory" would drift from it.
- Turns derived from the transcript rather than logged by the loop because
  the loop would log what it meant to send; the transcript is what the model
  saw.
- No automatic deletion because a numbered agent that was useful for one
  evening is cheap to keep asleep and expensive to lose; Franco deletes.

## Alternatives considered

- **One host process per agent** (`agc main run`): no VM, no boot. Rejected
  because the agent then runs with agc's own credentials and filesystem, the
  thing sandboxes exist to avoid, and because it would be the one agent that
  cannot run a tool; the router of ADR 0015 is a role of this design instead.
- **Always-on agents**: simpler loop, no wake hook. Rejected on Franco's
  words: the VM turns on when he talks to the agent and it has things to do.
- **A memory table** (facts, summaries) next to the transcript: rejected as
  a second truth; the harness keeps its own memory files on the volume.
- **Templates created from the panel** and stored in the database: rejected
  for now, a template is text that belongs in git; the panel can edit the
  file later.

## Consequences

- New: `agents` table and migration (unique timestamp, see
  `models/migrations/README.md`), `core/resources/agents` (list, get, create,
  keep, wake, sleep, delete, turns), routes and handlers, `agc agent run
  --agent`, the `OnPost` wake hook and the idle exit, `agent.state` on the
  bus, the `authorize` clause for `main`, `docs/openapi.yaml`, `make types`.
- The panel's Agents page switches from the mock to the API without a
  contract change; the Turns tab is new.
- Deleting a sandbox already keeps its volume (ADR 0008); deleting an agent
  must too.
- Open with Franco: whether templates should also be creatable from the
  panel, and whether never-kept agents idle for weeks should be swept.
