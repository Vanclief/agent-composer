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
   The limit that holds on this transport is not agc's body caps but
   microsandbox's vsock forwarder, measured from a guest on 2026-11-03: it
   gives the guest 256 KiB of credit (262144 bytes) and returns it only on
   packets the host sends back, so one request body larger than that stops
   there for good, and it loses a single `write()` larger than 32 KiB. The
   guest's `agc` writes at most 16 KiB at a time and sends a delivery's
   shots in pieces (ADR 0010, "Delivery report"); `agc msg post --image`
   from a sandbox is bound by the same 256 KiB.

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

## Amendment (2026-11-02): Inbox

A person's inbox is `#<name>`, the channel named after a human participant (`#franco`).
It takes from the person anything, and from agc its own posts; from an agent or a Task only a question (`kind: question`, `agc msg ask`)
or a reply in a thread there, such as the answer to that person's question or a follow-up in it.
Any other post by an agent is refused with "only questions and answers go in a person's inbox; post notes in #<task>"
(`messaging.Service.Post`, next to `authorize`). DMs and every other channel are unchanged.
A persistent agent (ADR 0016) has an inbox as well, `#<name>` for its name, where its owner talks to it: the agent, any person and agc post there freely,
anyone else only a question or a thread reply (amended 2026-11-03, task `agent-inbox-channel`).
Why: the inbox is what the person reads to find what waits on them, and notes ("merged", "never mind", "conflicts")
drowned the questions there (22 unread, 12 of them notes). Notes go in `#<task>`, which the manager and the owner read anyway.
The rule `inbox-holds-only-questions` covers what the server cannot see: whether a question really needs that person.
When a task is accepted, agc closes the manager's accept questions about it in the owner's inbox, and its agents' questions in `#<task>`, with a `Closed: <task> accepted by <who>.` reply (ADR 0010 item 4), so done tasks leave no open questions in the Inbox.

## Archived channels

Added 2026-11-02 (task `channel-archive`). Every task leaves `#<task>`
behind for good, and the channel list grows with every task ever done.

- When: `agc rest` archives `#<task>` once the task's `done_at` is older
  than `archive_done`, in the same tick as the sandbox collector (ADR 0008
  "Garbage collection": at start, then every 10 minutes). It logs a line per
  channel and publishes `channel.archived` (`{ channel, archived_at }`) on
  the event feed. The state is a row in `channel_archives` (`channel`
  unique, `archived_at`), since channels have no rows of their own.
- What changes: `GET /api/msg/channels` (and `agc msg channels`) leaves an
  archived channel out unless asked with `?archived=1` (`--archived`), which
  lists it with `archived_at`. Reading and waiting work unchanged. A post is
  refused with "channel archived" (not authorized) for everyone but agc, so
  `/accept` or `/reject` typed in an archived channel is refused too; accepting
  or rejecting a task through the API never un-archives its channel.
- The setting: `messaging: { archive_done: 168h }` in the config file, env
  `AGC_MESSAGING_ARCHIVE_DONE` overriding it, a Go duration, default 7 days;
  `0` turns archiving off.
- Re-opening an archived channel is not possible yet; a route to un-archive
  is a later change.

## Images

Added 2026-11-03 (task `talk-image-attachments`). Franco sends screenshots
to agents the way he does in Claude Code: dropped or pasted into the post
box, and the agent he writes to opens them.

- What: a post may carry up to 4 images, PNG, JPEG, GIF or WebP (sniffed
  from the bytes, never trusted from the name or a header), at most 5 MB
  each; a message with images may have an empty body. They travel base64 in
  the post's `images` (`[{name, data}]`, a request of at most 28 MB), over
  agc's REST API and a sandbox's socket alike; `agc msg post --image <file>`
  (repeatable) sends them.
- Where: kept on the agc host under `<agc home>/images/<message id>/<name>`,
  the name cleaned to letters, digits, dots, dashes and underscores, with
  the extension of its type, unique within the message. The message row
  lists them in `images` (`[{name, type, size}]`, jsonb, null for text-only);
  `url` is filled in from the id on every read, never stored.
- Served: `GET /api/msg/images/{id}/{name}` for people (`core/resources/messages`:
  `GetImageRequest` and `API.OpenImage`, as task shots), `/images/{id}/{name}`
  on a sandbox's socket, only for messages in channels the caller may read
  (`messaging.Service.OpenImage`), as posted, with their type. `agc msg read`
  prints each image's `url`.
- Agents: the task loop and the home loop (`agc agent run`) download a
  turn's images into the sandbox, `/home/agent/.agc/images/<message id>/<name>`,
  and the prompt lists each path under its message's line (`image: <path>`),
  so Claude Code or Codex reads it as a file; an image that cannot be fetched
  is listed by its URL and why.
- Not yet: other files than images, and removing an image from a posted
  message; images are kept as long as their message is.
