# ADR 0018: One agc allocates sandboxes on other agc servers

**Status:** Proposed — 2026-11-02. Builds on [ADR 0003](0003-tasks-run-in-microsandbox-vms.md), [ADR 0004](0004-secrets-are-placeholders-bound-to-hosts.md), [ADR 0005](0005-messaging-transport-and-identity.md), [ADR 0008](0008-tasks-allocate-compute.md) and [ADR 0009](0009-task-agents-and-managers.md). ADRs 0011 to 0017 are on sibling branches.

## Context

Every sandbox runs on the rig, the only machine with KVM. Franco has more
compute on the same LAN, first a Mac mini (microsandbox runs on Apple
Silicon), and wants one agc to use it: tasks, messaging, knowledge and the
panel stay in one place, and the VMs run wherever there is room.

What a sandbox needs from agc today is all local: the sandboxes API drives
msb through a `Runtime` interface (create, connect, start, stop, remove,
status), resolves the spec's secrets from the local secrets file (ADR
0004), and serves a unix socket per sandbox that the VM reaches over vsock
(ADR 0005). That socket is more than messaging: it takes `agc task deliver`
(ADR 0010) and, in an agent's home, `GET /agent`, `POST /agent/sleep` and
the task routes (ADR 0016), all answered from the primary's database.

Siblings act on sandboxes through the same `Runtime`: the watchdog
(agc/watchdog) and recovery (agc/recovery) call `api.rt.Connect` and
`api.rt.Status` directly, and the agents (agc/agents-*) stop and start homes
through the sandboxes API.

## Decision

1. **One primary, many peers.** The primary is the agc that holds tasks,
   messaging, knowledge and the panel. A peer is just another `agc rest`
   on another machine, with its own msb and its own secrets file. The
   primary knows its peers; a peer knows only its primary; peers do not
   know each other. A server may be both a primary (to its peers) and
   serve its own local sandboxes: the local server is always `local`.

2. **Config, not code.** In `core/controller.Config`, read like
   `defaults:`:

   ```json
   "servers": [{ "name": "mini", "url": "http://192.168.0.50:1202" }],
   "primary": { "url": "http://192.168.0.45:1202" },
   "listen": "192.168.0.50:1202"
   ```

   `servers:` on the primary (names are lowercase letters, digits and
   dashes, never `local`, unique; URLs http or https), `primary:` on a
   peer, `listen:` on either. A bad entry stops agc at startup with the
   index and the reason.

3. **The sandbox row gets `server`.** Migration `20261103141700` adds
   `server TEXT NOT NULL DEFAULT ''`; empty is local, so old rows need no
   backfill, and the view shows `local`. The task's `sandbox:` block (and
   a saved spec, and an agent template's `sandbox:`) gets `server: <name>`;
   `agc task create --on <name>` overrides it (`--server` was taken: it is
   the agc server the CLI talks to). An unknown name fails before anything
   is inserted: "server attic is not configured here; the servers are
   local, mini". A sandbox's server is set at allocation and never changes.

4. **One Runtime per server, routed by the row.** The local Runtime is
   unchanged. `peers.Remote` implements the same interface by calling the
   peer. `api.rt` is now a router: every call looks up the sandbox's row by
   name and goes to that server's runtime (with no peers configured it
   skips the lookup). So every caller of `api.rt`, in this package or a
   sibling, reaches the right server without knowing there is more than
   one, and the interface is unchanged: the watchdog, recovery and the
   agents need no change to keep compiling and working (see item 11 for
   what they see on a peer).

5. **The peer routes.** A peer serves its primary, under the token:
   `GET /api/peer/host`, `POST /api/peer/secrets/check`,
   `POST /api/peer/vms`, `GET|DELETE /api/peer/vms/{name}`,
   `POST /api/peer/vms/{name}/start|stop|exec` and
   `GET /api/peer/vms/{name}/terminals/{job}/ws`. They drive the peer's msb
   directly, one route per Runtime method, plus one `exec` route whose
   `op` names the executor call (run, look_path, write_file, read_file,
   remove_file, start_terminal, list_terminals): the boot (clone, agent
   config, TASK.md, setup, the command), the unpushed-work check and the
   terminal routes all run on the primary and reach the VM through it. The
   peer keeps no sandbox rows; the primary does. The existing
   `/api/sandboxes` routes do not fit: they are row-based, so reusing them
   would need a task and a sandbox row in the peer's database, kept in step
   with the primary's. `interfaces/cli/client` does not fit either: it
   speaks tasks and rows, and `core` must not import `interfaces`; its
   error shape (`{error: {code, message}}`) is reused, so a peer's error
   keeps its code (a VM msb does not know is still `ENOTFOUND`) and names
   the server.

6. **The socket is forwarded whole.** A peer VM's vsock socket is on the
   peer, in its own directory (`<home>/peer-sockets`). The peer answers
   nothing on it itself: it is a transparent proxy that passes every
   request, any method and path, to the primary at
   `/api/peer/sockets/<sandbox>/<path>` with the token, streaming the
   answer (a `GET /wait` blocks through it). The primary answers it with
   exactly the handler the sandbox's own socket would have had it run
   here: messaging as its task (grants from the spec and `#<owner>`), `agc
   task deliver`, and in a home the agent and task routes, as the agent.
   So a task on a peer delivers, sleeps and creates tasks exactly like a
   local one, and whatever a later branch adds to the socket is forwarded
   too. The peer vouches for which sandbox spoke (the path); who that
   sandbox speaks as comes from the primary's rows, never from the
   request, so the primary's single access rule (ADR 0005) applies
   unchanged and a peer cannot claim a task or a grant. The primary
   refuses a forwarded request for a sandbox that runs on itself. This is
   wider than forwarding join, post, read, wait and channels to
   `/api/msg/*` with the identity in the request, and safer: `/api/msg/*`
   is the human route, and putting the task's identity in the request
   would make the primary trust a caller it otherwise derives itself.
   After a restart, a peer finds the VMs it runs for a primary by the
   label `agc.server=<name>` and brings their forwarders back.

7. **Secrets never travel.** For a peer's sandbox the primary resolves the
   spec's secrets to names and hosts only (`secrets.Bind`), asks the peer
   whether its own secrets file has each (`POST /api/peer/secrets/check`,
   before the row is inserted), and sends names and hosts on create and
   start; the peer fills the values from its own file (`Store.Fill`). A
   missing secret fails the allocation at once: "secret OPENAI_API_KEY is
   not set on server mini: add it to <peer's secrets file> there". The
   primary's own file is never read for a peer's sandbox.

8. **The peer token.** `AGC_PEER_TOKEN`, a shared secret both sides read
   from their secrets file on every request (a change needs no restart),
   sent as `Authorization: Bearer`. It is in the known-hosts table of
   `secrets/secrets.go` and ADR 0004 bound to no host: an entry with an
   empty host list is agc's own secret, checked by agc, never sent to a
   third party and never given to a sandbox. `Resolve`, `Bind` and `Fill`
   refuse it, with or without `NAME@host`. A server without a token serves
   no peer routes (503 saying so) and calls no peer. It is never in the
   repo; tests use a fixture string.

9. **Terminals go through the primary.** The panel opens the terminal
   WebSocket on the primary as before; the primary's handler connects
   through the router to the Remote, whose `AttachTerminal` opens the
   peer's terminal WebSocket (same frames) and relays. The panel never
   talks to a peer.

10. **Listening is config.** `listen:` sets the bind address; empty keeps
    `":" + app.port`, as before. Note that this is all interfaces, not
    127.0.0.1: `rig-services-bind-localhost` describes how the rig is
    supposed to run, and the code never enforced it. A peer must be
    reachable from the primary, and the primary from the peer. The rule is
    not changed here: `proposals/peer-servers.yaml` proposes one for Franco
    to decide (bind a peer to its LAN address and rely on the token, or
    reach it over an SSH tunnel or Tailscale and keep localhost).

11. **What the siblings see on a peer.** Status, stop, start, remove and
    connect work for a peer's sandbox through the unchanged interface. The
    watchdog and recovery also type-assert `executor.JobWatcher` (their
    branches add it) on what `Connect` returns, to read msb's job log; the
    Remote's sandbox does not implement it yet, so for a peer's sandbox the
    watchdog says the runtime keeps no job log and recovery skips its job
    check. An action covers adding it once `JobWatcher` is merged (two more
    `exec` ops).

12. **`GET /api/servers`** lists `local`, then each peer by name, with
    `reachable` (the peer answered `GET /api/peer/host` within 3s, all
    peers asked at once), `sandboxes` and `in_use` (`cpus`, `memory_mib`:
    the sum of its running sandboxes' specs, msb's defaults for a spec that
    sets none, from the primary's rows), `free` (the host's cpus less those
    in use, never below zero, and the host's free memory) and `host` (the
    server's own report: os, arch, cpus, memory, free memory from
    `/proc/meminfo` or, on macOS, `sysctl` and `vm_stat`). No panel view
    yet; another task owns the panel.

## Consequences

- One shared token means the primary cannot tell peers apart: any peer
  holding it could speak for any peer's sandbox, and any primary holding it
  could drive any peer's msb, including VMs the peer runs for itself. The
  primary refuses only a forwarded request for its own local sandbox. A
  token per peer is an open action.
- A peer that is down makes its sandboxes `unknown` with the reason, like
  a local runtime that cannot answer; deleting one fails until the peer is
  back, as it would locally.
- A peer's sandbox keeps its volume `agc-<name>` on the peer, so its
  transcripts and turns (read host-side on the primary) are not readable
  from the primary yet. `host:` volume paths in a spec are paths on the
  peer.
- Names are picked on the primary. A peer that also runs its own tasks can
  hold a VM of the same name; msb then refuses the create and the boot
  fails saying so. Keeping peers to their primary avoids it.
- A peer's long `exec` (a clone, a setup script) runs under a 30-minute
  bound on the peer, not under the primary's request: a primary that gives
  up does not stop it.
- Moving a sandbox between servers, peers of peers, and picking a server
  by free capacity are left open (`actions/peer-servers.yaml`).

## Alternatives

- **The primary drives the peer's msb over SSH.** No new routes, but msb
  would need the secrets on the command line of a remote shell, and the
  vsock socket would still have to reach the primary.
- **Each peer a full agc with its own tasks, synced to the primary.** Two
  databases to reconcile for every task and message; the primary would no
  longer be where things are.
- **A peer-side copy of the sandboxes API with rows.** It would let the
  existing `/api/sandboxes` routes serve the primary, at the price of rows
  on the peer kept in step with the primary's; the runtime-level routes
  keep the peer stateless.
