# Implementation Plan: Git Worktrees for Workflow Executions

> Hand-off document for an implementing agent. Repo: `agent-composer` (Go). Read this
> top-to-bottom before writing code. Every file path/line below was verified against the
> tree at branch `franco/harness`.

## 1. Goal & principle

Today a workflow execution runs in a single directory string, `ShellRoot`, fanned out
unchanged to every node's `Conversation.ShellRoot` and used as the harness subprocess CWD
(`cmd.Dir`). We want a workflow to optionally run in a **git worktree** — a separate
checkout of a branch of the target repo — so multiple runs can work on **different branches
of the same repo concurrently** without colliding, and a later run can continue on an
existing branch.

**Guiding principle: git is the source of truth for worktrees. We do not persist them.**
`git worktree list` already knows every worktree's path, branch and HEAD, so we read from
it instead of keeping a second copy that could drift. Consequences:

- **No new DB table, no new columns, no migration.** The existing `workflow_executions.
  shell_root` already records where a run happened; the branch is derivable from that path.
- No "workspace" resource, no modes enum, no ephemeral/status/ownership bookkeeping.
- We manage **worktrees**, not branches. Branch lifecycle (push, PR, merge, delete) stays
  with git/GitHub and the agent — we never own or delete a branch (see §10).

Non-goals for v1: managing branches, `git clone`, per-node isolation, sandboxing.

## 2. Concept & the one resolution rule

A worktree is a `(repo, branch, directory)` triple. The whole feature is one optional
request field, `worktree` (a branch name), plus an optional `base` (start point for a new
branch). Resolution, given a repo and a branch:

```
resolve(repo, branch, base):
  1. branch already has a worktree in `git worktree list`  → reuse its path        (continue existing work)
  2. branch exists but has no worktree                      → worktree add <dir> <branch>   (base ignored)
  3. branch does not exist                                  → worktree add -b <branch> <dir> <base|HEAD>
```

All three are consequences of what git reports — nothing we store. git also refuses to
check the same branch out in two worktrees, which is exactly why concurrent runs on
different branches cannot collide.

## 3. New package `worktree/` (top-level, pure, no DB)

Small, DB-free, unit-testable wrapper over `git` via `os/exec`
(`exec.CommandContext(ctx, "git", ...)`, capture stderr, wrap errors with `ez` like
`runtime/harnesses/claude_code.go`).

```go
package worktree

type Manager struct { Root string } // global dir where worktrees live (§4)

type Info struct { Path, Branch, Head string; IsMain, Detached bool }

// RepoRoot resolves the enclosing git repo; ok=false when path is not a git repo.
func (m *Manager) RepoRoot(ctx context.Context, path string) (repo string, ok bool, err error)
func (m *Manager) List(ctx context.Context, repo string) ([]Info, error)

// Resolve returns the worktree dir for (repo, branch), reusing or creating per §2.
func (m *Manager) Resolve(ctx context.Context, repo, branch, base string) (path string, created bool, err error)

func (m *Manager) Remove(ctx context.Context, repo, path string, force bool) error
func (m *Manager) Prune(ctx context.Context, repo string) error
```

Git commands:
- repo root: `git -C <path> rev-parse --show-toplevel` (non-zero exit ⇒ `ok=false`, not an
  error — callers use this to decide whether to offer worktrees at all). Also snaps a
  *subdir* of a repo to the repo root automatically.
- list: `git -C <repo> worktree list --porcelain`; parse `worktree <path>` / `HEAD <sha>` /
  `branch refs/heads/<name>` / `detached` / `bare` blocks. First/`bare`-adjacent entry (the
  repo's own checkout) → `IsMain: true`.
- branch exists: `git -C <repo> show-ref --verify --quiet refs/heads/<branch>`.
- create (existing branch): `git -C <repo> worktree add <dir> <branch>`.
- create (new branch): `git -C <repo> worktree add -b <branch> <dir> <base|HEAD>`.
- remove: `git -C <repo> worktree remove <dir>`; add `--force` **only** when `force==true`.
- prune: `git -C <repo> worktree prune`.
- fetch (only when `base` is remote-looking, `^origin/` or `^refs/remotes/`):
  `git -C <repo> fetch origin` before creating, so the base is current.

Worktree dir path (`Resolve` computes it for the create cases):
`filepath.Join(Root, repoKey, branchKey)` where
- `repoKey = filepath.Base(repo) + "-" + sha256hex(absRepo)[:8]` (same-named repos stay
  distinct),
- `branchKey = sanitize(branch)` (`/` and unsafe chars → `-`); if that dir already exists on
  disk for a *different* branch, append `-" + sha256hex(branch)[:6]`.
Reuse (case 1) never computes a path — it returns whatever `git worktree list` reports.

## 4. Config — WorktreesRoot

Mirror how `ShellRoot` is threaded (verified path list):
- CLI flag `--worktrees-root` in `interfaces/cli/cli.go` (alongside `--shell-root` at
  `:66`), passed into `core.StackOptions` at `cli.go:294` and `:522`.
- Add `WorktreesRoot string` to `core.StackOptions` (`core/core.go`) and `runtime.Options`
  (`runtime/runtime.go:27`), stored on `Runtime`, exposed via `func (rt *Runtime)
  WorktreesRoot() string` next to `ShellRoot()` (`runtime.go:64`).
- Default when empty: `filepath.Join(userHome, ".agent_composer", "worktrees")`.
- Build one shared `*worktree.Manager{Root: rt.WorktreesRoot()}` and hang it on the `Stack`
  (`core/core.go` `Stack` struct) so both the execution flow and the REST handlers use the
  same instance.

## 5. Execution flow (the core wiring)

File: `core/resources/workflow/executions/create.go`.

### 5a. Extend `CreateRequest` (`:14`)
```go
type CreateRequest struct {
    WorkflowID string         `json:"workflow_id,omitempty"`
    File       string         `json:"file,omitempty"`
    Input      map[string]any `json:"input"`
    ShellRoot  string         `json:"shell_root,omitempty"` // repo or plain dir (unchanged; the base "path")
    Worktree   string         `json:"worktree,omitempty"`   // optional branch; only meaningful when ShellRoot is a git repo
    Base       string         `json:"base,omitempty"`       // optional start point for a NEW branch (default HEAD)
}
```
`Validate`: unchanged, plus reject `Worktree`/`Base` set with an empty `ShellRoot` only if
the effective base path can't resolve (keep it lenient — the resolution step errors clearly).

### 5b. Resolution in `prepareExecution` (`:78`, replacing the shellRoot block at `:96-101`)
```go
shellRoot := strings.TrimSpace(request.ShellRoot)
if shellRoot == "" && api.rt != nil {
    shellRoot = api.rt.ShellRoot()          // unchanged default
}

if strings.TrimSpace(request.Worktree) != "" {
    repo, ok, err := api.worktrees.RepoRoot(ctx, shellRoot)
    if err != nil { return nil, ez.Wrap(op, err) }
    if !ok { return nil, ez.New(op, ez.EINVALID, shellRoot+" is not a git repository", nil) }

    path, _, err := api.worktrees.Resolve(ctx, repo, request.Worktree, request.Base)
    if err != nil { return nil, ez.Wrap(op, err) }
    shellRoot = path
}

executor := workflowruntime.NewExecutor(shellRoot)
```
Everything downstream is unchanged: `conversation.ShellRoot = e.ShellRoot`
(`workflow/workflow_execute.go:384`) already gives every parallel node the same worktree
path, and `shell_root` is still recorded on the execution row via the existing
`StartWorkflow` call — so the branch/worktree used is auditable with **zero** executor,
recorder, model or migration changes.

`api.worktrees` is the shared `*worktree.Manager`; add it to `executions.API`
(`core/resources/workflow/executions/api.go`) and pass it through `workflow.NewAPI`
(`core/resources/workflow/api.go:20`) from the `Stack`.

## 6. REST endpoints

`interfaces/rest/api_routes.go`, after the `/api/workflows` group (`:22`):
```go
wt := api.Group("/worktrees")
wt.GET("", h.ListWorktrees)      // ?repo=<path>  → { "is_git": bool, "worktrees": [{path,branch,head,is_main}] }
wt.DELETE("", h.RemoveWorktree)  // ?repo=<path>&path=<worktree>&force=<bool>
```
- `ListWorktrees`: call `Manager.RepoRoot`; if not a repo return `{is_git:false}` (200, so
  the UI just hides the option — not an error); else `{is_git:true, worktrees: List(...)}`.
- `RemoveWorktree`: `Manager.Remove`; refuse if it targets the main worktree
  (`IsMain`) → `EINVALID`; on a dirty worktree, git refuses unless `force=true` — surface
  git's message, never force implicitly.
- The existing `CreateWorkflowExecution` handler decodes into `CreateRequest`, so the new
  `worktree`/`base` fields flow through automatically once §5a lands.
- Optionally extend `GetConfig` (`handler/workflow_executions.go:32`) to also return the
  default worktrees root.

Handlers read the shared `*worktree.Manager` off the server/stack (same place
`h.server.WorkflowAPI` comes from).

## 7. MCP & CLI

- MCP `mcp/agc/server.go`: add `Worktree` and `Base` string args to the run tool (`:25`),
  threaded next to the existing `ShellRoot` (`:96-105`) into `CreateRequest`.
- CLI `interfaces/cli/cli.go`: `run` command gains `--worktree` and `--base` flags → set on
  the `CreateRequest`; global `--worktrees-root` (§4). Optional `agc worktree list|remove`
  subcommands wrapping the same manager.

## 8. SPA

`interfaces/rest/static/workflow.html`, extending the existing `ShellRootPicker` (commit
d6808e9, ~line 1455; run body sends `shell_root` ~line 1913):
- When the chosen path changes, `GET /api/worktrees?repo=<path>`. If `is_git === false`,
  hide all worktree UI (behaves exactly like today — just runs in the dir).
- If git: show a **free-text** "branch / worktree" input (type an existing branch to reuse
  or check it out; type a new name to create), the existing worktrees as quick-pick
  suggestions, and an optional/advanced **base** field (default `HEAD`).
- On run, send `worktree` and (if creating) `base` alongside `shell_root`. Keep the
  existing `localStorage` behavior for last-used path.

## 9. Backward compatibility

`worktree` omitted (or path not a git repo) ⇒ identical to today: run directly in
`shell_root`/default, works for non-git dirs, no worktree touched. No data migration; no
change to existing execution rows or the recorder.

## 10. Robustness / edge cases (must handle)

- **`worktree` set but path isn't a git repo** → `EINVALID` (the UI already hides the option
  via `is_git:false`, this is the API-level guard). Never silently run in the plain dir when
  a worktree was explicitly requested.
- **Branch already checked out elsewhere** (e.g. it's the current branch of the user's main
  checkout) → `git worktree add` refuses; surface as `EINVALID: "branch <b> is checked out
  in <path>; pick another"`.
- **Removing a dirty worktree** → git refuses without `--force`; return that message and let
  the caller pass `force=true` explicitly. Never `--force` on the user's behalf (could
  discard uncommitted work).
- **Removing the main worktree** → refuse (`IsMain`).
- **We never delete branches.** `Remove` removes the worktree only; the branch and its
  commits stay in the repo. Post-merge branch cleanup is the user's normal git/GitHub flow.
- **Concurrent runs creating the same new branch** → the second `worktree add` fails; retry
  once as a "reuse" (by then the branch/worktree exists).
- **Stale worktree admin entries** → run `Manager.Prune` on startup and before `List`.
- **Path normalization** → `filepath.Abs` on incoming paths, mirroring `DefaultShellRoot`
  (`core/resources/workflow/api.go:52`).
- **`base` is a remote ref** → `git fetch origin` first so it's current (only for the
  create-new-branch case; ignored when reusing/checking out).

## 11. Testing

- `worktree` pkg (unit) against a temp `git init` repo: `RepoRoot` (repo, subdir, and
  non-repo → ok=false); `Resolve` for all three cases in §2 (assert reuse returns the same
  path, checkout of an existing branch, create-new with a `base`); `List` parsing incl.
  `IsMain`/detached; `Remove` (clean, dirty-without-force refused, main refused); `Prune`.
- `prepareExecution` (unit) with a fake/temp-repo manager: `worktree` empty → path
  unchanged; `worktree` set on a repo → ShellRoot becomes the worktree path; `worktree` set
  on a non-repo → `EINVALID`.
- Concurrency: two runs, different branches, same repo → two distinct worktree dirs, both
  succeed; same branch → second reuses the first's path.
- Backward-compat: `shell_root`-only request runs exactly as before.
- `go build ./...` and `go test ./...` green.

## 12. Out of scope / future

- Branch operations (list/delete/push, PR + merge detection via `gh`/GitHub API).
- `git clone` mode / cross-filesystem workspaces.
- Per-node worktrees.
- Sandboxing the agent to its worktree (the unwired `mcp/shell` `workdirResolver` +
  `runBashIsolated` are the building blocks if wanted later).
- Auto-cleanup of worktrees (ephemeral runs) — currently explicit via `Remove`.

## 13. Task order

1. `worktree` package (`RepoRoot`, `List`, `Resolve`, `Remove`, `Prune`) + unit tests.
2. Config: `WorktreesRoot` (CLI flag → `StackOptions` → `runtime.Options` +
   `WorktreesRoot()`), build the shared `*worktree.Manager` on the `Stack`.
3. Thread the manager into `workflow.NewAPI` → `executions.NewAPI`.
4. `CreateRequest.Worktree`/`Base` + the `prepareExecution` resolution block (§5).
5. REST `/api/worktrees` list + remove handlers.
6. MCP args + CLI flags/subcommands.
7. SPA worktree picker.
8. Tests + docs (`README`, `docs/openapi.yaml`).
