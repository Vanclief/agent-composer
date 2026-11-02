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

- `core/resources/<x>`: resource APIs (business logic), one file per verb. `tasks` is the work (title, brief, assignee, done; status computed); `sandboxes` is the compute a task is allocated (one VM each: async boot, stop/start/delete, terminals, the per-sandbox messaging socket), ADR 0008.
- `core/resources/knowledge`: read-only REST over `knowledge/`, loading `evidence/ beliefs/ decisions/ rules/ actions/` under `<home>/knowledge` (or `AGC_KNOWLEDGE_DIR`) on every request; list (`?without=`), why, doubt and conflicts.
- `interfaces/rest`: echo server; routes in `api_routes.go`, handlers in `handler/`, dispatch type switch in `server/server.go`.
- `interfaces/cli`: the `agc` command-line interface.
- `models`: bun models; every table is registered in `ALL` in `models/db_registry.go`.
- `models/migrations`: schema migrations as Go files named `YYYYMMDDHHMMSS_name.go`, run automatically at startup; see its README.
- `executor`: host and sandbox executors.
- `task`: the task.yaml file and the sandbox environment spec (`Spec`, `Sandbox`), saved specs.
- `secrets`: secret placeholders bound to hosts.
- `messaging`: messaging transport and identity.
- `images/agc-agent`: the agent image that runs inside sandboxes.
- `panel/`: React control panel, see `panel/CLAUDE.md`. `web/` is the older workflow builder UI.
- `docs/decisions`: ADRs 0001–0009 (tasks and sandboxes: 0003, 0008); read the relevant one before changing that area.
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
