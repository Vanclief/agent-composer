# ADR 0011: Direct messages are `@<human>/<task>` channels, forwarded outward to WhatsApp

**Status:** Proposed — 2026-11-02 (proposal `dms-are-human-task-channels`; open until Franco accepts it). Builds on [ADR 0005](0005-messaging-transport-and-identity.md) and [ADR 0004](0004-secrets-are-placeholders-bound-to-hosts.md).

## Context

ADR 0005 gave every task one channel, `#<task>`, and left private channels
out to ship the transport. Franco asked why there are no direct messages
(2026-11-01): an agent that needs a decision only he can make should be able
to say so to him, privately, and he should be told even when the panel is
closed. Today a task can post in `#<task>` (which its manager and everything
granted can read) or in the owner's inbox `#franco` (ADR 0009), a channel
anyone granted it can read, and nothing leaves agc.

## Decision

### Part 1: direct messages in agc

1. **A DM is a channel named `@<human>/<task>`**, for example
   `@franco/yc-pitch`. Both names use the charset of a channel name
   (lowercase letters, digits, `_`, `-`, at most 64 each). There is no new
   table: the channel is a string in `messaging_messages` like any other.

2. **One access rule, extended.** `authorize` in `messaging/service.go`
   gains one clause: a task may read, wait on and post in any channel named
   `@<anyone>/<its own task>`. Humans may do anything, as before. No other
   task can read, wait on, post in or list participants of it, by either
   spelling. A task needs no grant for it, and cannot be granted another
   task's DM through the spec because the clause names the caller's own task.

3. **`@franco` is short for `@franco/<my task>`.** The service expands a bare
   `@<human>` to `@<human>/<caller's task>` in post, read, wait and
   participants, taking the task from the caller agc resolved from the vsock
   socket, never from the request. `agc msg post @franco <text>` and
   `agc msg read @franco` work unchanged over any transport, and the stored
   channel is always the full name. A human, who has no task, uses the full
   name; a bare `@franco` from a human is refused as an invalid channel.

4. **Listing.** `agc msg channels` and `GET /api/msg/channels` return, for a
   task, its own channel, its grants and every `@<anyone>/<its task>` that has
   messages (a DM exists once someone posts in it). A human sees every
   channel, as before. The wire format is unchanged, so `docs/openapi.yaml`
   (which does not describe the messaging routes) and the generated panel
   types need no change.

5. **TASK.md gains one line**: "Post progress in `#<task>`, and anything that
   needs `<human>` in `@<human>`." The human is the task's `assignee` when it
   is a person, else `franco`. The guest README (`/etc/agc/README.md`) gains
   a short "Direct messages" section.

6. **The panel inbox is not part of this change.** The task `talk-follow`
   builds the panel side and treats any channel named `@<me>/...` as a DM,
   badged by unread count kept per browser. This ADR fixes only the naming it
   relies on. The panel needs no server-side read state.

### Part 2: WhatsApp bridge, outbound only

7. **A `notify` package with one interface.**
   `Notifier.Notify(ctx, Notice{Human, Sender, Task, Text}) error`. The
   messaging service holds one (`SetNotifier`) and, after storing a message in
   a `@<human>/<task>` channel written by anyone but a human, calls it on its
   own goroutine with a 15 second timeout. A failure or panic is logged and
   dropped: a post never waits on, or fails because of, the notifier. A
   human's own replies do not notify anyone.

8. **The implementation is the WhatsApp Cloud API**:
   `POST https://graph.facebook.com/<version>/<phone_number_id>/messages`,
   `Authorization: Bearer <token>`, body
   `{"messaging_product":"whatsapp","to":"<number>","type":"text","text":{"body":"<sender> in <task>:\n<text>"}}`.
   A human with no configured number is skipped silently. Text over about
   3,900 characters is cut with `...`.

9. **Config, not code.** The phone number id and each human's number live in
   agc's config file (the `notify.whatsapp` block, read like `defaults:` in
   `core/controller`); the access token is the secret `WHATSAPP_TOKEN`, added
   to the known-hosts table in `secrets/secrets.go` bound to
   `graph.facebook.com` and to the table in ADR 0004. agc reads it from the
   secrets file on every send. No token and no number is in the repo.
   Without `phone_number_id` and at least one number the notifier is off.

   ```json
   {
     "notify": {
       "whatsapp": {
         "phone_number_id": "<id from Meta>",
         "version": "v21.0",
         "numbers": { "franco": "<digits with country code, no +>" }
       }
     }
   }
   ```

   And in `secrets.env`: `WHATSAPP_TOKEN=<permanent token>`.

### What Franco has to create at Meta

None of this exists yet and the code was tested only against a local test
server.

1. A **Meta Business account** (business.facebook.com), verified enough to
   use the WhatsApp Business Platform.
2. A **Meta developer app** (developers.facebook.com) of type Business, with
   the **WhatsApp** product added. This links the app to a WhatsApp Business
   Account (WABA).
3. A **sending phone number**: Meta's test number for development, or a real
   number registered to the WABA. Its **Phone number ID** (not the number) goes
   in config as `notify.whatsapp.phone_number_id`.
4. **Franco's own number** as the recipient, digits with country code, goes in
   `notify.whatsapp.numbers.franco`. With a test number the recipient must be
   added to the allowed list and verified by code first.
5. A **permanent access token**: in Business Settings create a System User,
   give it the app and the WABA with `whatsapp_business_messaging`
   permission, and generate a token that never expires. It goes in
   `secrets.env` as `WHATSAPP_TOKEN`. The temporary 24-hour token in the app
   dashboard is for trying only.
6. **The 24-hour window.** A free-form text message reaches a person only
   within 24 hours of their last message to the business number; otherwise
   Meta requires an approved message template. Outbound-only text will
   therefore stop arriving unless Franco writes to the number now and then,
   or a template (for example "New message from {{1}} in {{2}}") is approved
   and the sender is changed to use it. The code sends type `text` only.

## Open question: inbound

Franco replying from WhatsApp into the DM channel needs Meta to call a public
HTTPS webhook. The rig is LAN-only and its services bind to `127.0.0.1`
(`rules/rig.yaml`: `rig-services-bind-localhost`), so nothing is built. Two
options:

- **A tunnel to one webhook route.** A tunnel (Cloudflare Tunnel, Tailscale
  Funnel) forwards one path on a public name to a small listener on the rig
  that checks Meta's `X-Hub-Signature-256` with the app secret and posts into
  `@franco/<task>` as Franco. It is quick, but it opens the first public
  entrance to a host that holds real secrets (`rig-holds-real-secrets`), and
  the listener must map "which task" from a reply, which WhatsApp does not
  carry; it would need a convention such as replying to a message, or a
  `task: text` prefix.
- **A relay outside the rig that is polled.** A tiny hosted function receives
  Meta's webhook, verifies the signature, stores the message, and the rig
  pulls from it over outbound HTTPS. Nothing listens on the rig and the rig's
  rule holds, at the cost of another thing to host and secure, and the same
  "which task" convention.

The open action `whatsapp-inbound-needs-a-reachable-webhook` tracks it.

## Rationale

- A DM costs one string convention and one clause because messaging already
  resolves the caller from the transport; the name encodes both parties, so
  the access check needs no lookup.
- Expanding `@franco` on the server means the task name comes from the
  vsock identity agc assigned and not from anything the agent says, and the
  CLI needs no knowledge of its task.
- Notifying outward is a hook on the one place messages are stored, so every
  way a message arrives (vsock, REST, agc itself) behaves the same.
- Failing quietly keeps a flaky third party from stopping an agent from
  talking; the cost is a missed ping, and the message is still in the panel.
- The token is a placeholder-bound secret like the others, with its host in
  the table, so no new secret mechanism exists.

## Alternatives considered

- **A `dm` table or a `private` flag on channels.** More schema, a migration,
  and a second place access is decided. The channel name already carries it.
- **Use `#franco`, the owner's inbox.** It is one channel for all tasks, which
  anyone granted it can read, so a task would see the others' questions.
- **Notify from the CLI inside the sandbox.** It would need `WHATSAPP_TOKEN`
  and the Meta host inside the VM, the exposure ADR 0004 exists to avoid.
- **Notify for every `#<task>` message.** Too noisy; the DM is the "needs the
  human" channel.

## Consequences

- No schema change, no migration, no new route; `GET /api/msg/channels`
  returns more rows for a task.
- `ValidateChannel` accepts `@human/task`. A grant in a task spec may name one
  (`task/spec.go` uses the same check) but it adds nothing to the rule.
- `core/controller.Config` gains `notify.whatsapp`; `secrets.KnownHosts` gains
  `WHATSAPP_TOKEN`.
- A message to a DM with no configured number is stored and shown in the panel
  and tells no one.
- Until inbound exists Franco answers in the panel, or in a `@franco/<task>`
  post made with `agc msg`; WhatsApp is a doorbell, not a conversation.
