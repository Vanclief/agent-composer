# ADR 0003: Tasks run in microsandbox VMs

**Status:** Accepted — 2026-11-01

## Context

Today an execution runs harnesses (Claude Code, Codex CLI) as host subprocesses
in a `ProjectDir`, optionally inside a git worktree of that repo. That gives
file isolation between parallel runs but nothing else: agents share the host's
network, credentials, installed tools and process table, and a human cannot
look over an agent's shoulder except through logs.

We want a **Task**: an isolated environment that lives longer than one
workflow run, can hold several terminals (agents, shells, a browser), can be
watched live, can be put away and brought back, and can be deleted completely.
The Orca workspace is the mental model.

A spike on 2026-11-01 (Linux rig, AMD 5800X3D, KVM) proved the path with
microsandbox 0.7.13: a Debian image with git, Node, Claude Code and Codex boots
in under two seconds; a detached TTY job survives disconnects and replays
output to a browser terminal over WebSocket; `claude -p` works on a
subscription OAuth token delivered as a placeholder secret; a git clone over
HTTPS works the same way; the real token never appears inside the VM and is
blocked when sent to a non-allowed host.

## Decision

1. **A Task is one microsandbox sandbox.** Task name = sandbox name. The
   sandbox carries the label `agc.task=<id>`. There is no second
   environment type in v1.
2. **The Task spec is one small YAML block**, usable on its own
   (`agc task create -f task.yaml`) or embedded in a workflow spec under
   `environment:`. Same shape in both places:

   ```yaml
   environment:
     image: agc-agent            # default; any OCI image works
     cpus: 2
     memory: 4096                # MiB
     repo: github.com/vanclief/agent-composer   # cloned to /home/agent/<name>
     ref: master                 # branch, tag or sha; optional
     setup: |                    # runs once after clone; optional
       go mod download
     secrets: [CLAUDE_CODE_OAUTH_TOKEN, GITHUB_TOKEN]   # names only, see ADR 0004
     env:
       GOFLAGS: -mod=mod
     volumes:
       - go-mod-cache:/home/agent/go/pkg/mod:rw   # named volume, created on first use
       - datasets:/data                           # named volume, read-only (default)
       - host:/srv/models:/models                 # host directory, explicit, read-only
   ```

   A volume entry is `NAME:DEST[:rw]` for a microsandbox named volume, or
   `host:PATH:DEST[:rw]` for a host directory. Named volumes are the
   default because they are portable across hosts and visible to
   `msb volume ls`; host paths are an explicit escape hatch. Mounts are
   read-only unless `rw` is written, because the runtime does not
   coordinate concurrent writers across Tasks. Volumes outlive the Task:
   deleting a Task never deletes a volume. The repo is a clone, not a
   volume, so it stays private to the Task.

3. **Lifecycle is microsandbox's lifecycle, shown verbatim.** Stop keeps the
   disk and frees CPU and memory; that is what the UI calls a saved task,
   using the backend word `stopped`. Start boots it again from its disk.
   Delete is `RemoveSandbox` and is final. agc invents no state of its own.
4. **agc persists only what microsandbox does not know**: task id, display
   name, the spec it was created from, the sandbox name, and which workflow
   executions ran in it. Everything else (status, resources, disk, jobs) is
   read live from the runtime. Same principle as `WORKSPACES_PLAN.md`.
5. **A terminal is a TTY job.** `ExecDetached` with a TTY, then `job.Attach`
   bridged to a WebSocket that agc serves and xterm.js in the control panel
   consumes. Jobs outlive the browser tab and the agc process; reattaching
   replays recent output. Agents started by workflow nodes are jobs too, so
   every agent is watchable by default.
6. **One `Executor` interface replaces the direct `exec.CommandContext` in the
   harnesses.** Two implementations: `host` (what we have today, kept for
   machines without KVM and for development) and `sandbox` (jobs in a Task).
   Harness code does not know which one it is on.
7. **One base image, built from a Dockerfile in this repo.** Debian, git,
   tmux, Node, Claude Code, Codex CLI, a git credential helper that reads
   `GITHUB_TOKEN`. Loaded with `docker save | msb load` until a registry is
   worth the trouble. Per-repo tooling goes in `setup`, not in new images.
8. **agc is the only daemon.** microsandbox has none; each sandbox is a child
   process with state under `~/.microsandbox`. agc restarts the sandboxes
   that were running when the host comes back.

## Rationale

- One sandbox per Task maps every Task operation onto one runtime call:
  create, stop, start, remove, fork, snapshot. No orchestration layer to
  write or debug.
- Reading status from the runtime instead of mirroring it removes a class of
  "UI says running, VM is dead" bugs.
- The `Executor` seam is the only change the existing engine needs. Nodes,
  connectors, loops, conversations, hooks and the DSL compiler are untouched.
- Jobs as terminals give live monitoring for free and make a stuck agent
  something you can type into, which logs never will.
- The spike showed every piece working with no workaround, so there is no
  unknown left that argues for a smaller first step.

## Alternatives considered

- **Keep host subprocesses, add Docker for isolation.** Containers share the
  host kernel and need a daemon; no snapshots, no secret placeholders. Docker
  remains the fallback if a host has no KVM, behind the same `Executor`.
- **One sandbox per workflow node.** Clean isolation per agent, but agents
  working on one repo then need a shared filesystem or constant syncing. The
  Task is the shared desk; isolation between tasks is what matters.
- **A separate Task object with its own lifecycle state machine in SQLite.**
  Duplicates what the runtime already tracks and drifts from it. Rejected
  per `WORKSPACES_PLAN.md`.
- **Cloud sandboxes (microsandbox cloud, E2B, Daytona).** microsandbox
  supports a cloud backend behind the same SDK, so this stays open without
  designing for it now. Local-first matches how agc is used.

## Consequences

- Hosts need Linux with KVM or macOS on Apple Silicon. The spike rig is the
  first such host; the `host` executor keeps development working elsewhere.
- Credentials enter a Task only by the mechanism in
  [ADR 0004](0004-secrets-are-placeholders-bound-to-hosts.md).
- Agents inside a Task reach agc through a per-task vsock socket; messaging
  on top of it is [ADR 0005](0005-messaging-transport.md).
- Workflow nodes need `environment:` to say where they run; `run`, `wait`,
  `script` and trigger node kinds become possible once the executor exists
  and get their own spec changes.
- The `worktree` package stays for the `host` executor and is not used inside
  a Task; a Task clones its repo and uses plain branches.
- Deleting a Task deletes uncommitted work in its clone and disk but keeps
  every volume it mounted. The UI must say both in the delete control itself.
- Named volumes are listed and removed through the runtime (`msb volume`);
  agc stores nothing about them beyond the Task spec that mounts them.
- Agent state that should outlive a Task (for example a harness home with
  session history) is a named volume mounted `rw` by exactly one Task at a
  time. This is one of the memory mechanisms, alongside harness session
  resume and message history.
