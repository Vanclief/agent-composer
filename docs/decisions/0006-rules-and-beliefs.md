# ADR 0006: Rules, beliefs and decisions

**Status:** Accepted — 2026-11-01
**Builds on:** [0001 — git + plain text for all artifacts](0001-git-and-plain-text-for-all-artifacts.md)

## Context

Task specs say what machine, code and secrets an agent gets, but nothing
about what it must obey or what we hold true. Today that lives in prose
(`AGENTS.md`, memory notes), which a human can read but CI cannot run and an
agent cannot ask "what are the rules of X, and does X pass".

Franco's framing:

- **Rules** encode behavior: the rules of a sport, a condo's rules, a
  country's laws, the rules for accepting a PR. A rule says must or must
  not. When reality disagrees with a rule, the actor is wrong.
- **Beliefs** are things we hold true. When reality disagrees with a belief,
  the belief is wrong and gets revised. Beliefs chain: "I believe A because
  B is true." Down the chain you reach something observed (who saw it,
  when); up the chain you see everything that falls if this one is wrong.
- **Decisions** are choices made once on beliefs, whose outcome is an action
  or a standing state of the world (SQLite by default, Tasks in microsandbox
  VMs, secrets as placeholders). A rule constrains ongoing behavior and can
  be checked again and again; a decision produced something and is only
  revisited when what it rests on changes.
- Rules may rest on beliefs ("stealing is wrong", "Go is better than Rust
  for writing code with agents") and on decisions. When a belief is put in
  doubt, every decision and rule resting on it, directly or through others,
  shows up for review.

## Decision

1. **Three kinds of YAML file, loaded by the `knowledge` package.** A file
   is a ruleset if it has `subject:`, a beliefs file if it has `beliefs:`,
   a decisions file if it has `decisions:`. They live in `rules/`,
   `beliefs/` and `decisions/`.

   ```yaml
   subject: agent-composer code and pull requests
   rules:
     - id: go-no-explicit-semicolons
       rule: Never write an explicit `;` in Go outside a 3-clause for header.
       check: scripts/check-semicolons.sh    # optional; sh -c, exit 0 = pass
       because: [strict-style-makes-agent-output-reviewable]
   ```

   ```yaml
   beliefs:
     - id: agents-write-go-well
       belief: Go is better than Rust in general for writing code with agents.
       because: [go-has-one-formatting, go-syntax-is-small]
     - id: go-has-one-formatting
       belief: Go has one canonical format, enforced by gofmt.
       observed: { who: franco, when: 2026-11-01 }
   ```

   ```yaml
   decisions:
     - id: tasks-run-in-microsandbox-vms
       decision: A Task is one microsandbox VM, and agc reads its state live from the runtime.
       because: [host-subprocesses-share-the-host, microsandbox-spike-passed]
       adr: docs/decisions/0003-tasks-run-in-microsandbox-vms.md
       decided: { who: franco, when: 2026-11-01 }
   ```

2. **Ids are slugs, unique across every loaded file.** Every belief has
   `observed`, `because` or both. Every decision has `because` and
   `decided`. A belief rests only on beliefs; decisions and rules rest on
   beliefs and decisions. Every `because` must resolve, and the graph of
   beliefs and decisions has no cycles. Errors name the file and the id.
3. **One export format.** `agc rules show` prints a ruleset as a markdown
   checklist an agent or a human can read; `agc rules check` runs the
   mechanical rules (those with `check:`) and lists the manual ones, failing
   if a check fails, so CI uses the same file.
4. **Doubt is a listing, not a truth update.** `agc beliefs doubt <id>`
   lists the beliefs, decisions and rules that rest on a belief, each with
   its file; a decision in doubt is the signal to open a review of it.
   Nothing is written. `agc beliefs why <id>` follows a belief down to its
   observations.
5. **No confidence numbers and no automatic revision.** A belief is held or
   it is not; a human rechecks what doubt lists.

## Alternatives considered

- **Markdown files per rule set.** Readable, but neither checkable nor
  chainable without parsing prose.
- **Confidence scores and propagation.** Numbers nobody can justify, and an
  engine that changes conclusions behind a human's back.
- **A graph library.** The graphs are tens of nodes; maps and a depth-first
  walk are enough.

## Consequences

- `AGENTS.md` stays the prose guide; `rules/repo.yaml` and `rules/rig.yaml`
  encode the parts of it that are rules, and CI runs
  `go run . rules check rules/`.
- Each ADR has an entry in `decisions/repo.yaml` naming the beliefs it rests
  on, so doubting a belief points at the ADRs to revisit.
- The semicolon check is a heuristic over source lines, not a parser.
- Check commands run with `sh -c` from the directory agc is run in, so
  `agc rules check` runs from the repo root.

## Next step

Attach named rulesets, beliefs and decisions to task specs so agents in
sandboxes get them, for example written into the sandbox's `AGENTS.md` at
setup. Not part of this change.
