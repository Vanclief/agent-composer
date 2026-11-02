# ADR 0009: Every task has an agent and a manager, and they talk in its channel

**Status:** Accepted — 2026-11-01. Builds on [ADR 0005](0005-messaging-transport-and-identity.md) and [ADR 0008](0008-tasks-allocate-compute.md).

## Context

ADR 0008 gives a task a brief and lets a sandbox run a `command`. Left
there, every task needs someone to write the command, and the only way to
talk to the agent is to type into its terminal. Franco's framing: a task has
a default agent (a harness and a model, today Claude Code with Opus 5.5)
and a manager who receives a message when the task is created and can talk
to the agent from then on. The manager must be able to see the exact text
the agent was given, and the exact text the manager was sent.

## Decision

1. **Defaults are configuration.** agc's config file gains
   `defaults: { harness: claude_code, model: claude-opus-5-5, manager: <name> }`,
   overridable with `AGC_DEFAULT_HARNESS`, `AGC_DEFAULT_MODEL` and
   `AGC_DEFAULT_MANAGER`. A task's `assignee` defaults to the harness and the
   task gains `model` (defaults to the configured model when the assignee is
   a harness, empty for a person) and `manager` (a messaging name; defaults
   to the configured manager, or to the creator's name when the request
   carries one). The CLI and the panel's create dialog show the defaults
   filled in, so a user sees who will work and who will manage before
   pressing create.

2. **The agent is the sandbox's first job, started by agc.** When a sandbox
   is allocated for a task whose assignee is a harness and no `command` is
   given, agc runs the guest binary: `agc agent run` with the harness, the
   model, the task name and `/home/agent/TASK.md`. That command is the loop:

   - join the task's channel as the sandbox name;
   - run the harness once with `TASK.md` as the prompt, non-interactively
     (`claude -p --output-format json` or the codex equivalent), keeping the
     session id, and post the final answer in `#<task>`;
   - then `agc msg wait '#<task>'`; every message not posted by itself becomes
     the next turn on the same session, and each answer is posted back;
   - exit when the sandbox stops, or when the manager posts `/done`, after
     which the harness is asked once for a closing summary that is posted.

   The job is a TTY job named `agent`, so the terminal still shows the raw
   stream as a log; the conversation lives in the channel. A `command` on the
   sandbox request replaces all of this for custom programs.

3. **TASK.md is stored and shown, byte for byte.** The text written into the
   sandbox is kept on the sandbox row as `instructions` and returned in
   `SandboxView.instructions`. It is the title, the brief, and these standing
   instructions, filled in:

   ```
   # <title>

   <brief>

   ## How this task works

   You are <harness> (<model>) working for the task `<name>` inside a sandbox.
   Your manager is <manager>. The repository, if any, is cloned at
   /home/agent/<repo>. Work counts only once it leaves this sandbox:

   - Commit on a branch named `agc/<name>` and push it.
   - Post what you changed and what you verified in `#<name>` with
     `agc msg post '#<name>' <text>`; your manager reads that channel and
     answers there. Ask questions there too, then keep working on what does
     not depend on the answer.
   - Do not modify files outside the repository, and never print secret values.
   ```

4. **The manager's message is a channel message, visible to everyone in the
   channel.** On task creation agc joins `#<task>` as the reserved name `agc`
   and posts exactly:

   ```
   @<manager>, task `<name>` is open: <title>.
   Agent: <harness> (<model>) on sandbox `<sandbox>`, starting.
   The agent's instructions are the next message. Reply here to talk to it;
   post /done to close the task.
   ```

   followed by a second message whose body is the `instructions` text from
   item 3, verbatim. When the task has no sandbox (a person's task) the
   second line reads `Assigned to <assignee>, no sandbox.` and there is no
   second message. `agc` is a reserved participant name no one else can
   take; messages from it carry `task` empty and `human` false.

5. **The manager reaches the task the same way as anyone else.** They are a
   messaging participant (a person in the panel, or an orchestrating session
   using `agc msg wait` through the REST API). Being named `manager` on a
   task means agc addresses them in the opening message; it grants nothing
   else. `/done` from the manager sets the task's `done_at` (the same effect
   as `POST /api/tasks/{id}/done`); from anyone else it is an ordinary
   message.

6. **The panel shows both texts where they belong.** The task page's Brief
   tab shows the brief, and under it "What the agent received", the
   `instructions` of the newest sandbox, verbatim, in a mono block. The
   Messages tab is the channel, so the opening message to the manager is the
   first thing in it. Nothing is paraphrased.

## Rationale

- Config defaults make `agc task create -f` with only a brief produce a
  working task with a known agent, model and manager.
- Driving the harness non-interactively with a resumable session is what the
  harness CLIs already support, and the guest `agc msg` client already
  exists; the loop is the missing twenty lines, not a runner.
- Keeping the agent's instructions on the sandbox row and the manager's
  message in the channel means the question "what exactly was it told?" is
  always answerable from the UI, with no logs to dig through.
- A reserved `agc` sender keeps system messages distinguishable from humans
  and agents without a new message type.

## Alternatives considered

- **agc drives the harness through the runtime's harness package and the
  sandbox executor.** Structured and testable, but a bigger change to the
  executor seam for the same observable behavior. Revisit if the loop proves
  fragile.
- **Type channel messages into the agent's interactive terminal.** Works for
  humans, breaks on TUI timing for programs, and the agent's replies would
  stay in the terminal.
- **A separate notification system for managers.** The channel already
  reaches people and agents alike, and the panel already shows it.

## Consequences

- `tasks` gains `model` and `manager`; `sandboxes` gains `instructions`.
  `CreateTaskRequest` gains `model` and `manager`; `TaskView` returns both;
  `SandboxView` returns `instructions`.
- The guest `agc` gains `agent run`; the image needs no other change.
- `agc msg join agc` is refused for everyone but the server.
- The CLI's `agc task create` prints the defaults it applied.
