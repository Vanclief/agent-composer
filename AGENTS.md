# AGENTS.md

## Go style rules (strict)

### No explicit semicolons outside `for`

- Never write an explicit semicolon `;` in Go code.
- Exception (only): semicolons are allowed _only_ inside a 3-clause `for` header:
  `for init; condition; post { ... }`
- All other explicit-semicolon forms are forbidden, including:
  - multiple statements on one line: `a(); b()`
  - `if init; condition { ... }` (e.g. `if err := f(); err != nil { ... }`)
  - `switch init; { ... }` and type-switch init forms

### Required rewrite style

- One statement per line.
- If you need an initializer for `if`/`switch`, move it to its own line above the statement.

### Self-check before final output

- Before returning code, scan what you wrote:
  if it contains `;` outside `for init; condition; post {}`, rewrite until compliant.

## Review guidelines

- Flag any explicit semicolon `;` usage outside `for init; condition; post { ... }` as an issue.
- Flag any `if init; cond {}` / `switch init; {}` usage as an issue (requires rewrite to multi-line).

## Repo map

- `core/resources/<x>`: resource APIs (business logic), one file per verb. `tasks` is the work (title, brief, accept criteria, assignee, model, manager, owner, delivered_at/summary, done_at/accepted_by; status open/running/review/done, computed); `sandboxes` is the compute a task is allocated (one VM each: async boot, stop/start/delete, terminals, the per-sandbox messaging socket), ADR 0008; `/home/agent` is a private ext4 disk sized by the spec's `disk` (default 20G, min 2G) and `~/.claude`, `~/.codex` link into the named volume `agc-<sandbox>` at `/home/agent/.harness`, kept after delete (`volume` on the row); `GET /api/sandboxes/{id}/transcripts[/{path}]` reads the session files from the host side of that volume (`AGC_MSB_VOLUMES_DIR`, default `~/.microsandbox/volumes`), never through the VM (ADR 0008 "Disk and transcripts"); `agc rest` deletes the stopped sandboxes of tasks done longer than `sandboxes.keep_done` ago (env `AGC_SANDBOX_KEEP_DONE`, default 24h, `0` off; never a home; volumes kept), by hand `agc sandbox gc [--dry-run] [--keep]` / `POST /api/sandboxes/gc`, and the list carries `disk` per sandbox (ADR 0008 "Garbage collection"). ADR 0009: `defaults: { harness, model, owner }` in the config file (env `AGC_DEFAULT_HARNESS|MODEL|OWNER`) fill in a task, and `GET /api/defaults` (`tasks.GetDefaults`) returns the effective `{ harness, model, owner }` plus `system` (`msgmodels.SystemName`, `agc`); `manager` is required; the sandbox stores its TASK.md as `instructions` (`sandboxes.Instructions`) and, for a harness with no `command`, runs `agc agent run` (`interfaces/cli/agent.go`) as the job `agent`; on create agc posts the opening message and the instructions in `#<task>` as the reserved participant `agc` (`tasks.OpeningMessage`); the manager is the owner's only interlocutor for a task: a task sandbox is granted `#<task>` and its spec's channels only (`sandboxes.Grants`), its agent asks its manager, who raises to the owner in `#<owner>`, and `GET /api/msg/questions?for=<name>` counts `#<name>` and `#<task>` of tasks they manage (`tasks.channelsFor`), not of tasks they only own. ADR 0010: TASK.md lists `accept` under "Accepted when"; `agc task deliver` (inside a sandbox over its messaging socket, `POST /task/deliver` on it, after the shots in pieces of at most 192 KiB to `POST /task/shots/{label}?offset=` since no body over 256 KiB crosses microsandbox's vsock, ADR 0005 item 2; elsewhere `POST /api/tasks/{id}/deliver`), or an agent-loop answer starting `DONE:`, sets `delivered_at` (status `review`); `POST /api/tasks/{id}/accept`, `/accept` in `#<task>` from the manager or owner, or `/accept <task>` from the owner in `#<owner>` (`tasks/commands.go`) sets `done_at`, stops the task's sandboxes and closes its open agent questions in `#<task>` and the manager's `Accept <task>?`/`<task>:` ones in `#<owner>` with agc's `Closed: <task> accepted by <who>.` reply (the collector tick closes those of every done task) (the manager merges a delivered branch itself unless `merge-needs-franco-for-ui-or-breaking` fails, then asks the owner, ADR 0010 "Who merges"); `reject` / `/reject <why>` clears `delivered_at`. A delivery may carry `report` (`models/task.Report`: `criteria` one `{met: yes|no|partly, note}` per `accept` line in order, `changes {branch, sha, files}`, `checks`, `left_out`, `needs`; rules in `Report.Validate`/`ValidateFor`, ADR 0010 "Delivery report"), stored on the task (jsonb, replaced on every delivery, kept on reject), rendered after the summary in the Delivered message (`tasks.DeliveredMessage`), sent with `agc task deliver <summary> --report <file|->`; TASK.md's "Delivering" section asks for it. A report's `shots` (`{label, file}`, at most 8 PNGs ≤ 2 MB) travel base64 in the delivery's `files` map, are kept under `<agc home>/shots/<task>/<label>.png` (`tasks.SetShotsDir`), served at `GET /api/tasks/{name}/shots/{label}.png`, with `url` under `AGC_PUBLIC_URL` when set; the reviewer puts them in its `Before:`/`After:` lines.
- `core/resources/agents`: ADR 0016 persistent agents. `turns*.go` is the turns reader: `GET /api/agents/{name}/turns[/{n}]` (volume `agc-<name>`, 404 when absent) and `GET /api/sandboxes/{id}/turns[/{n}]` parse the Claude Code and Codex session files on the volume host-side on every request (no cache), a turn per user message that is not a tool result, `n` across sessions by start time, `channel`/`thread` from the loop's `agc-sessions.json`, lines that are not JSON counted in `skipped`.
- `interfaces/cli/agent_home.go`: ADR 0016 items 5–7, `agc agent run --agent <name>` (or `AGC_AGENT` with no task) in a persistent agent's home sandbox: reads `GET /agent` on the socket (`{ agent, channels, asked, idle }`; `asked`: channels outside `channels` where it started a thread, granted, where only replies in its threads are turns, `msgmodels.AnswerFor`) every cycle, waits on all the channels at once (`GET /wait?channels=a,b`, `messaging.Service.WaitChannels`, one global cursor), one turn per thread root (`<channel>:<root id>` → harness session) answered as a reply to the root, role `main` one session (`main`) fed `BatchPrompt(records)`; sessions and the cursor in `/home/agent/.harness/agc-sessions.json` and `agc-cursor.json` (the kept volume); the sessions file is shared with the task mode and read and written only through `interfaces/cli/sessions.go` (`loadSessionStore`, `Get`, `Set` merges with the file under one lock, safe for concurrent turns; keys `<channel>:<root>`, `main`, and `task:<task name>` reserved for the task loop, ADR 0017); turns of different threads run at once (`interfaces/cli/agent_turns.go` `turnRunner`: one queue per session key, a queued turn folds in later messages, at most `GET /agent`'s `turns`, the template's `turns` 1..8 default 3, `1` one batch at a time), the cursor saved when a batch is dispatched; with a clone (the loop's cwd, `--repo`) each thread but `main` runs its harness in its own detached worktree `/home/agent/threads/<channel>-<root>` (`threadWorktree`, `cmd.Dir` through `withHarnessDir`), pruned at start; idle (no turn running) → `POST /agent/sleep` and exit 0.
- `core/resources/schedules` (models in `models/schedule`): ADR 0012. A schedule (`name`, `every` ≥ 1m, `enabled`, `task` template as JSON, optional `poll: { command, secrets }`, `last_run_at`, `last_error`; `next_run_at` computed) creates a task through `tasks.Create` every interval, or with a poll runs `sh -c` on the host (60s, env PATH, HOME and the named secrets) and creates one task per new JSON-lines `key`, recorded in `schedule_items`; templates are Go text/templates over the item's fields and `.Now`. Only `agc rest` runs the scheduler (`schedules.API.Start`, every 30s, no overlapping ticks); `agc schedule run` ticks by hand; every tick publishes `schedule.ran` on `core/events`.
- `core/resources/agents` (model `agent.Agent` in `models/agent`): ADR 0016 persistent agents, a row each (`name`, `role`, `number`/`series`, `kept`, `harness`, `model`, `effort`, `owner`, `home`, `idle`); state (`asleep|waking|awake`, else the home sandbox's word) is computed from the home sandbox, never stored. Templates are `agents/<role>.yaml` under the agc home (`AGC_AGENTS_DIR` overrides; the repo ships `agents/reviewer.yaml`, `agents/main.yaml` and `agents/designer.yaml`, a UI/UX designer for the panel whose tasks always wait for Franco), read on every request. `Reads` (its inbox `#<name>`, an inbox like a person's, `#<task>` for tasks it manages, the template's `reads` globs) is the one channel list behind `GET /agent`, the `OnPost` wake hook and the home's grants; whether a post is a turn for an agent is `msgmodels.TurnFor` (`models/messaging/turn.go`), shared by the wake hook, the home loop and the task loop (ADR 0016 item 6): never its own post, always a person's, another agent's only when it is a question or names `@<name>` (a plain one is only context for the next turn in its thread), and agc's own posts wake nobody except `Delivered:` and `Accepted by <someone else>.` in `#<task>`, which wake the task's manager; a reply in a thread the agent started is a turn in any channel (`msgmodels.AnswerFor`, the hook finds the root's author); role `main` passes messaging's `authorize` everywhere (`messaging.SetMainAgent`). A home is a sandbox with no task (`task_id` zero, `sandboxes.agent` set, `SandboxView.agent`), booted by `sandboxes.AllocateHome` with AGENT.md and `agc agent run --agent`, mounting the agent's volume `agc-<agent>` whatever the home is called, so a later home reuses it, and the loop joins messaging as the agent's name, taking it over from a stale participant; its socket speaks as `messaging.Caller{Agent: <home>}` and serves `GET /agent`, `POST /agent/sleep` (204, then stops) and, since a VM cannot reach `agc rest`, `POST /tasks`, `GET /tasks/{name}`, `POST /tasks/{name}/accept` and `POST /tasks/{name}/reject` (managed tasks only; `agc task create|get|accept|reject` inside a home use them); participants and messages carry `agent` (the home's name). Every home state change, create, keep and delete publishes `agent.state`. CLI `agc agent list|show|create|keep|wake|sleep|delete`.
- `core/resources/accounts` (model `account.Account` in `models/account`): ADR 0018 accounts, a credential for one provider kept as an `ACCOUNT_<NAME>` line of `secrets.env` (written by `secrets.Store.Set`/`Remove`, atomically, 0600) plus a row (`name`, `provider` anthropic|openai, `kind` oauth|api_key, `secret`, `email`, `default`); no route returns a value and `CreateRequest` marshals without it. At most one default per provider (`POST|DELETE /api/accounts/{name}/default`, transaction plus partial unique index); `sandboxes.resolveSecrets` binds the default as `<secret> as <variable>` on allocate, home allocate and start when the spec sets no variable the harness reads, and stores it in the spec; `DELETE /api/accounts/{name}` is refused while a running sandbox's spec names the secret (`sandboxes.API.SandboxesUsingSecret`). `usage` stays null until a `UsageSource` is set (`API.SetUsage`); sign-in (ADR 0018 "Sign-in", `signin*.go`, `accounts/oauth`) runs Claude Code's OAuth login per account (`POST /api/accounts/{name}/signin`, `.../signin/complete`, `DELETE .../signin`), keeps the refresh token as `ACCOUNT_<NAME>_LOGIN`, sets `signed_in_at`, and the poller reads usage with it, never sandboxes.
- `skills` and `core/resources/skills`: ADR 0008 "Skills". The skills repository is `<agc home>/skills/<name>/SKILL.md` plus files (`AGC_SKILLS_DIR` overrides; the repo ships `skills/rtsdesign`, `scripts/rig-deploy.sh` points at it with `AGC_RIG_SKILLS`), read on every request; a spec's `skills` (also a template's `sandbox.skills`, carried as `skills` on the agent view) are checked before boot and written by `prepare` (and again on start) into `~/.claude/skills/<name>/` and `~/.codex/skills/<name>/`, replacing the volume's copy; `GET /api/skills`, `GET /api/skills/{name}` (`text/markdown`), `agc skill list|show`.
- `core/resources/triggers`: ADR 0014. Outside events posted by agc as plain messages (`[github] <actor> <what> in <repo>: <text>` + link): a PR or run whose head branch is `agc/<task>` goes to `#<task>` when the task exists, else `#main`. `agc rest` (`Stack.StartScheduler`) polls the `triggers.github.repos` of the config file every `poll` (default 60s; no repos, off) with `GITHUB_TOKEN` from the secrets file, dropping `ignore_users` and the token's own login (`GET /user` once per poll) unless `post_own: true`; cursors per `github <repo> <kind>` in `<home>/triggers/cursors.json`; `agc triggers show` lists them.
- `core/resources/knowledge`: read-only REST over `knowledge/`, loading `evidence/ beliefs/ decisions/ rules/ actions/` under `<home>/knowledge` (or `AGC_KNOWLEDGE_DIR`) on every request; list (`?without=`), why, doubt and conflicts.
- `interfaces/rest`: echo server; routes in `api_routes.go`, handlers in `handler/`, dispatch type switch in `server/server.go`.
- `interfaces/cli`: the `agc` command-line interface.
- `interfaces/cli/client`: HTTP client for the task, sandbox and spec routes; `agc task|sandbox` use it when `AGC_SERVER` or `--server` is set, else the in-process stack.
- `models`: bun models; every table is registered in `ALL` in `models/db_registry.go`.
- `models/migrations`: schema migrations as Go files named `YYYYMMDDHHMMSS_name.go`, run automatically at startup; see its README.
- `executor`: host and sandbox executors.
- `task`: the task.yaml file and the sandbox environment spec (`Spec`, `Sandbox`), saved specs.
- `secrets`: secret placeholders bound to hosts.
- `messaging`: messaging transport and identity; `PostAsSystem` posts as `agc`, `OnPost` hooks run after every post; messages carry `human`; in a person's inbox `#<name>` only the person and agc post freely, anyone else only a question or a thread reply, and a persistent agent's `#<name>` is an inbox too, where the agent itself also posts freely (`isInbox`: a participant of that name that is human or has `agent` set; ADR 0005 "Inbox"); `agc rest` archives `#<task>` of tasks done longer than `messaging.archive_done` ago (env `AGC_MESSAGING_ARCHIVE_DONE`, default 168h, `0` off; `channel_archives`), hidden from `GET /api/msg/channels` unless `?archived=1`, readable, closed to posts but agc's (ADR 0005 "Archived channels"); a message carries up to 4 `images` (PNG/JPEG/GIF/WebP ≤ 5 MB, base64 in the post, `messaging.Service.PostImages`), kept under `<agc home>/images/<message id>/<name>` (`SetImagesDir`), listed as `{name, type, size, url}` and served at `GET /api/msg/images/{id}/{name}` (`core/resources/messages`, `GetImageRequest` + `OpenImage`; `/images/{id}/{name}` on a sandbox socket, readers of the channel only), `agc msg post --image`; the task and home loops download a turn's images to `/home/agent/.agc/images/<message id>/<name>` and list each path under its message line in the prompt (`interfaces/cli/agent_images.go`, ADR 0005 "Images"). `messaging/guest-readme.md` is `/etc/agc/README.md` in the image.
- `images/agc-agent`: the agent image that runs inside sandboxes.
- `panel/`: React control panel, see `panel/CLAUDE.md`. `web/` is the older workflow builder UI.
- `docs/decisions`: ADRs 0001–0010, 0012, 0014, 0016 and 0018 (tasks and sandboxes: 0003, 0008; messaging: 0005; knowledge: 0006; proposals and gates: 0007; task agents, managers and owners: 0009; task acceptance: 0010; scheduled tasks: 0012; triggers: 0014; persistent agents: 0016, Proposed; accounts: 0018, Proposed); read the relevant one before changing that area.
- `docs/openapi.yaml`: REST API spec.

## Adding a REST endpoint, checklist

1. Request/response types and the method in `core/resources/<x>/<verb>.go`, following the existing pattern: a `Validate()` on the request and `func (api *API) X(ctx, requester interface{}, req)`.
2. Handler in `interfaces/rest/handler/`.
3. Route in `interfaces/rest/api_routes.go`.
4. Case in the dispatch type switch in `interfaces/rest/server/server.go` (long operations get their own timeout there).
5. Document it in `docs/openapi.yaml`.
6. Route test in `interfaces/rest/*_routes_test.go`.
7. Run `make types` so the panel's generated TypeScript matches (CI fails on drift).

## Schema changes

- Edit the model AND add a migration in `models/migrations` for existing databases.
- Never hand-edit a database.

## Verifying

- Go: `go build ./... && go vet ./... && go test ./...`
- `gofmt -l .` must list only the three known unformatted files: `core/resources/hooks/create.go`, `core/resources/hooks/update.go`, `core/resources/workflow/compose.go`. Do not reformat them.
- Panel: `make panel-check`.

## Rules, beliefs and decisions

- Five kinds of file (ADR 0006): `evidence/` records what was seen (who, when, source), `rules/` says what must or must not be done, `beliefs/` what we hold true and why, `decisions/` the choices made on those beliefs (one per ADR), `actions/` what to do because of a decision (`opened:`, and `done:` once done).
- Beliefs cite evidence by id. A belief is grounded, an opinion (no evidence under it), contested (something grounded contradicts it) or in doubt (its evidence is all retracted); the label is computed when printed, never written. Retract evidence with `retracted: { who, when, why }` and keep it in the file.
- `by:` on a belief names who holds it (omitted: the repo owner). A leaf belief with `experience: { of, in }` (`of` a person, never a model) is a judgment: stronger than an opinion, weaker than evidence; the weakest leaf wins, in doubt > contested > opinion > judgment > grounded.
- `or:` lists alternative reasons (`{ because, evidence }` on beliefs, `{ because }` elsewhere): grounded if any set is. `contradicts: [ids]` on a belief or evidence flags a contradiction; nothing picks a side.
- Evidence `who` is a person or a mechanical check (`ci`), never a model: agc rejects it. A model that read a real source goes in `translated_by`, with `source` set; a model's own conclusion is an opinion.
- `go run . rules check rules/` runs the mechanical rules and lists the manual ones (CI runs it); `agc rules show` prints them as a markdown checklist. Both note rules resting on an opinion or a doubted belief.
- A rule no command can check may say who is trusted to judge it: `judge: { trusted: [claude_code, codex_cli], ask: <sharp question> }` (`trusted` holds harness ids from `runtime/harnesses/catalog.go` or people; `ask` defaults to the rule text). Trust is per rule: rig rules trust only `franco`. Unchecked rules print "— needs <trusted>" or "— needs a human".
- `agc rules judge --as <harness id> [--on <path|git ref|->] [--model <m>] [--record]` asks that agent, read only, whether each rule trusting it holds (default subject: `git diff origin/master...HEAD`), prints ✓/✗/? per rule and skipped ones, and fails on a fail or an unclear answer. A verdict is an opinion, never evidence: `--record` appends it to `beliefs/judgments.yaml` as a belief with `by: <agent>` and `when:`, no evidence; without `--record` nothing is written.
- `agc beliefs show`, `agc decisions show`, `agc actions show` (open first, then done) and `agc evidence show` print each item as a sentence with its ids in a trailer.
- `agc beliefs why <id>` follows a belief down to its evidence; `agc beliefs doubt <id>` lists the beliefs, decisions, rules and open actions resting on a belief or a piece of evidence, and apart those still standing on other support; `agc beliefs conflicts` lists contested contradictions with the evidence and opinions under each side.
- `--without <id>` (repeatable) on the show commands judges as if that evidence or belief were gone, under a "Without <ids>:" first line; nothing is written. `--ids` on every show and why command prints the compact id form; use it in scripts.

## Deploying to the rig

- `make rig-deploy` (wraps `scripts/rig-deploy.sh`; `--dry-run` prints the remote script, `--help` lists the env overrides).
- Requires a clean tree and the branch pushed to origin at the local SHA; the rig builds `origin/<branch>`.
- The rig is the only KVM host. Sandbox integration tests run there with `AGC_SANDBOX_TEST=1`.

## Subagent workflow

- Agents work in git worktrees branched from the current feature branch (not master).
- Commit in the worktree; the main session merges.
- Keep commits scoped to the task.
- Never push.

## Design language

- UI follows the `rtsdesign` skill: icons, slots, presets; raw YAML only behind a toggle.
- Statuses are shown verbatim from the backend; never relabel or derive them client-side.
- Disabled controls say why in their own label, not only in a tooltip.
