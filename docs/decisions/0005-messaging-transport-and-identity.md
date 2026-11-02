# ADR 0005: Messaging transport, identity and channel scope

**Status:** Accepted — 2026-11-01

## Context

`docs/messaging-protocol/index.html` defines how agents talk: three nouns
(participant `@name`, channel `#name`, message), four verbs (post, read, wait,
join/leave with list), and guarantees (durable append-only log, total order
per channel, the reader owns its position). It deliberately leaves out
transport, identity, storage and operations.

With Tasks as isolated VMs ([ADR 0003](0003-tasks-run-in-microsandbox-vms.md))
those gaps must be filled: an agent in a VM has no network path to agc, no
identity agc can trust, and no way to know the interface exists. We also want
Tasks to talk to each other (the Slack-for-agents idea) without one Task being
able to read or disrupt everything.

## Decision

1. **One message space for the whole agc, served by agc itself.** No separate
   messaging server. Channels are global names; they are how Tasks talk to
   each other.

2. **Transport is HTTP+JSON over a per-Task vsock socket.** When agc creates a
   Task it passes `--vsock <socket>:<port>`; inside the VM the agc command
   connects to that port and agc answers on the host end. vsock is a
   host-to-VM pipe with no IP address, no DNS and nothing to firewall, so
   there is no token to bootstrap and no way for a Task to reach another
   Task's socket. The same handlers are mounted on agc's normal REST API for
   the control panel and the `host` executor.

3. **Every Task gets its own channel, and that is all it can reach by
   default.** Creating Task `review-42` creates `#review-42`. Agents in that
   Task may join and post there without declaring anything. Any other channel
   must be listed in the Task spec:

   ```yaml
   environment:
     channels:
       - "#releases"          # read and post
       - "#alerts:read"       # read only
   ```

   A join or post outside `#<own task>` and the listed channels is refused
   with an error naming the channel, so the agent can tell the user instead
   of guessing. The human in the control panel is bound by none of this.

4. **Identity is assigned by agc, not claimed by the sender.** On join an
   agent proposes a display name such as `reviewer`. agc rejects it if that
   name is currently in use anywhere in the space; otherwise it returns a
   participant id, a random opaque string. Every message agc stores carries
   `from`, `participant_id` and `task`, filled in by agc from the id and the
   socket the request arrived on. A sender cannot set those fields. Names
   are released on leave or when the job that joined ends, and a later holder
   of the same name has a different id, so history is never ambiguous.
   Readers see `@reviewer`, and the UI shows the Task next to it.

5. **Agents learn the interface from the image.** The base image ships the
   agc command with the four verbs:

   ```
   agc msg join <name>
   agc msg post <#channel> <text> [--reply-to <id>]
   agc msg read <#channel> [--after <cursor>] [--limit N]
   agc msg wait <#channel> [--after <cursor>] [--timeout 30s]
   agc msg channels
   ```

   plus `/etc/agc/README.md`, a short plain-text page with the verbs, the
   rules above and two examples. Harness instructions get one sentence:
   "Messaging: run `agc msg --help` or read `/etc/agc/README.md`." No MCP
   server in v1; a wrapper over the same HTTP calls can be added for
   harnesses that want tools.

6. **Storage is one append-only table in the existing SQLite database.**
   Columns: id, channel, from, participant_id, task, reply_to, body,
   created_at. Messages are plain text. Cursors are message ids. No
   retention policy in v1.

7. **Humans are participants.** The control panel posts and reads as the
   logged-in user (`@franco`), with the same verbs over REST. First UI: a
   channel list, a message stream with Task shown next to each sender, and a
   post box. The Task view shows the channels that Task's agents have joined.
   `GET /api/msg/participants?channel=%23name` lists who has posted in a
   channel, active first, then those who left; `?all=1` (humans only) lists
   everyone who ever joined, in any channel.

## Rationale

- Serving messaging from agc over vsock gives transport, authentication and
  Task attribution from one runtime flag. Anything network-based would need
  a token per Task and a way to deliver it, which is the problem ADR 0004
  exists to avoid.
- Per-Task channel plus an explicit allow list gives the Slack model with a
  blast radius of one Task by default. Cross-Task conversation is a stated
  intent in the spec, which is where a reviewer expects to find it.
- Assigned ids stop impersonation without passwords: a Task cannot forge the
  socket it arrived on, and it cannot pick a name already held. The display
  name stays human-friendly.
- A CLI in the image works for every program in the VM, scripts included,
  and needs no harness-specific integration.
- One SQLite table reuses the database, backups and migrations agc already
  has, consistent with `WORKSPACES_PLAN.md` and
  [ADR 0001](0001-git-and-plain-text-for-all-artifacts.md).

## Alternatives considered

- **One message space per Task.** Simplest isolation, but Tasks could not
  talk at all, which defeats the purpose. The per-Task default channel gives
  the same isolation with a door.
- **Network transport (HTTP to the host over the sandbox's network).**
  microsandbox blocks host and private addresses by default and would need a
  published port and a token per Task. More moving parts for less safety.
- **Names claimed freely, trust the agent.** One agent posting as `@franco`
  or as the reviewer is exactly the tampering the user asked to prevent.
- **A real broker (NATS, Redis streams).** Another daemon, another config,
  for message volumes measured in dozens per minute. Revisit only if agc
  runs across several hosts.
- **MCP tools only, no CLI.** Ties messaging to harnesses that speak MCP and
  leaves shell scripts and `run` steps without a way to post.

## Consequences

- Task creation needs the vsock flag and a listener per Task in agc; the
  `host` executor uses REST on localhost instead and attributes messages to
  the host pseudo-Task.
- The base image grows by the agc command and the readme. Building the
  Linux agc binary into the image is part of the image build.
- Spec validation must reject `channels` entries without a leading `#` and
  unknown suffixes other than `:read`.
- The protocol document's "deliberately missing" list stays as is; this ADR
  fills transport, identity and scope only. Typed messages, acks and
  presence remain out.
- When a Task is deleted its channel is kept with its history; a new Task
  with the same name continues the same channel. The UI says so on delete.

## Amendment (2026-11-01)

Messages carry a `kind`, `message` or `question`; a reply from anyone other than the asker answers a question.
This is the one typed message we take on, because an agent that needs an answer must be able to say so,
and the owner or manager must see what is waiting on them (`GET /api/msg/questions`) instead of rereading channels.
