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
  B is true." Down the chain you reach evidence (who saw what, when);
  up the chain you see everything that falls if this one is wrong.
- **Evidence** is what was seen, kept apart from what we conclude from it.
  An **opinion** is a belief held with no evidence under it. Belief should
  be computed from evidence, so it changes when the evidence does: new
  evidence grounds an opinion, retracted evidence puts a belief in doubt.
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

1. **Four kinds of YAML file, loaded by the `knowledge` package.** A file
   is a ruleset if it has `subject:`, an evidence file if it has
   `evidence:`, a beliefs file if it has `beliefs:`, a decisions file if it
   has `decisions:`. They live in `rules/`, `evidence/`, `beliefs/` and
   `decisions/`.

   ```yaml
   subject: agent-composer code and pull requests
   rules:
     - id: go-no-explicit-semicolons
       rule: Never write an explicit `;` in Go outside a 3-clause for header.
       check: scripts/check-semicolons.sh    # optional; sh -c, exit 0 = pass
       because: [strict-style-makes-agent-output-reviewable]
   ```

   ```yaml
   evidence:
     - id: gofmt-is-the-only-go-formatter
       evidence: gofmt ships with the Go toolchain and the ecosystem has no competing formatter.
       who: franco
       when: 2026-11-01
       source: https://go.dev/blog/gofmt    # optional: URL, file path or command
       # retracted: { who: franco, when: 2026-11-02, why: ... }   # optional
   ```

   ```yaml
   beliefs:
     - id: agents-write-go-well
       belief: Go is better than Rust in general for writing code with agents.
       because: [go-has-one-formatting, go-syntax-is-small]
     - id: go-has-one-formatting
       belief: Go has one canonical format, enforced by gofmt.
       evidence: [gofmt-is-the-only-go-formatter]
   ```

   ```yaml
   decisions:
     - id: tasks-run-in-microsandbox-vms
       decision: A Task is one microsandbox VM, and agc reads its state live from the runtime.
       because: [host-subprocesses-share-the-host, microsandbox-spike-passed]
       adr: docs/decisions/0003-tasks-run-in-microsandbox-vms.md
       decided: { who: franco, when: 2026-11-01 }
   ```

2. **Ids are slugs, unique across every loaded file.** Evidence has text,
   `who` and `when`; a `retracted:` needs `who`, `when` and `why`. A belief
   has `because`, `evidence`, both or neither. Every decision has `because`
   and `decided`. A belief rests only on beliefs and cites only evidence;
   decisions and rules rest on beliefs and decisions. Every `because` and
   every cited evidence id must resolve, and the graph of beliefs and
   decisions has no cycles. Errors name the file and the id. The earlier
   inline `observed: { who, when }` on a belief is rejected with an error
   naming the field.
3. **Grounded, opinion and in doubt are computed, never stored.** A belief
   is judged on the leaves of its because tree, the beliefs resting on no
   other belief. It is `grounded` when every leaf cites at least one piece
   of evidence that is not retracted, an `opinion` when some leaf cites
   none, and `in doubt` when some leaf cited evidence that is now all
   retracted. Both reasons are reported when both apply. Retracted
   evidence stays in its file: the history is the point. `agc beliefs
   show` prints every belief with its label; `agc rules show|check` and
   `agc decisions show` note a rule or decision resting on an opinion or a
   doubted belief, without changing any check's status. `agc evidence show`
   lists each piece with how many beliefs cite it.
4. **One export format.** `agc rules show` prints a ruleset as a markdown
   checklist an agent or a human can read; `agc rules check` runs the
   mechanical rules (those with `check:`) and lists the manual ones, failing
   if a check fails, so CI uses the same file.
5. **Doubt is a listing, not a truth update.** `agc beliefs doubt <id>`
   lists the beliefs, decisions and rules that rest on a belief, or on a
   piece of evidence through the beliefs citing it, each with its file; a
   decision in doubt is the signal to open a review of it. Nothing is
   written. `agc beliefs why <id>` follows a belief down to its evidence.
6. **No confidence numbers and no automatic revision.** A belief is held or
   it is not, and the tool never relabels or rewrites anything: labels are
   computed each time they are printed, and a human rechecks what doubt
   lists.

## Alternatives considered

- **Markdown files per rule set.** Readable, but neither checkable nor
  chainable without parsing prose.
- **Confidence scores and propagation.** Numbers nobody can justify, and an
  engine that changes conclusions behind a human's back.
- **Opinion as a fourth kind of file.** Whether a belief has evidence
  changes as evidence arrives or is retracted; storing it would drift from
  the evidence it is computed from.
- **Evidence inline on each belief.** Two beliefs seen from one fact would
  repeat it, and doubting the fact could not find both.
- **A graph library.** The graphs are tens of nodes; maps and a depth-first
  walk are enough.

## Consequences

- `AGENTS.md` stays the prose guide; `rules/repo.yaml` and `rules/rig.yaml`
  encode the parts of it that are rules, and CI runs
  `go run . rules check rules/`.
- Each ADR has an entry in `decisions/repo.yaml` naming the beliefs it rests
  on, so doubting a belief, or a piece of evidence under it, points at the
  ADRs to revisit.
- Beliefs we act on without evidence are written down as opinions rather
  than left out, so they show up next to the rules and decisions they hold
  up.
- The semicolon check is a heuristic over source lines, not a parser.
- Check commands run with `sh -c` from the directory agc is run in, so
  `agc rules check` runs from the repo root.

## Next step

Attach named rulesets, beliefs and decisions to task specs so agents in
sandboxes get them, for example written into the sandbox's `AGENTS.md` at
setup. Not part of this change.
