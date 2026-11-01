# ADR 0007: Proposals and gates

**Status:** Accepted — 2026-11-01
**Builds on:** [0006 — Rules, beliefs and decisions](0006-rules-and-beliefs.md)

## Context

ADR 0006 gave agc a way to say what we hold true, what must be done and
what was decided, and to see what falls when a belief is wrong. It did not
say how anything an agent produces gets in. Today an agent commits in a
worktree and the main session merges; nothing records what the agent put
forward, what it rested on, or what the human did with it.

Franco's framing, from the conversation that produced this:

- "Rules should be taken by humans but agents can propose a new rule or
  decision or change. I as a human should be the one always having the
  final decision, which includes on what requires my final decision, what
  doesn't and what I should be observing."
- The Serac question (Westworld): does the human end up doing what the
  machine says and only then deciding? Serac had the final say over
  Rehoboam the whole time. What made him a servant was that he could not
  evaluate the recommendation, so saying no became irrational, so he
  stopped. Final authority without the ability to judge is a ritual.
- The protections are not about who signs but whether the human can still
  disagree with reasons: legibility (if you cannot follow why, you do not
  approve), sources the system did not produce, a rejection rate that is
  watched, overrides recorded as data, and goals that are never proposed.

## Decision

1. **Everything an agent produces is a proposal.** A sixth kind of file,
   `proposals:` in `proposals/`, loaded by the `knowledge` package:

   ```yaml
   proposals:
     - id: add-no-panic-rule
       proposes: rule                 # rule | belief | evidence | decision | action | gate | code
       proposal: Never call panic in a REST handler.
       because: [strict-style-makes-agent-output-reviewable]   # beliefs and decisions it rests on
       evidence: []                   # evidence it cites, like a belief
       by: claude_code
       when: 2026-11-01
       change: claude/no-panic        # optional: where the change itself lives
       became: no-panic-in-handlers   # optional: the id it was recorded under, once accepted
       accepted: { who: franco, when: 2026-11-02 }
       # or rejected: { who: franco, when: 2026-11-02, why: Handlers already recover. }
   ```

   A proposal is open, accepted or rejected, never two of those. A
   rejection needs `why`: an override is a record of what the human saw
   that the proposer did not, and it stays in the file. `goal` is not a
   proposable kind and is refused by name: what we want is never proposed,
   only how to get there. Nothing in agc lands a proposal: a human merges,
   or the gate lets the proposer land it.
2. **Gates, written by a human, say which proposals land without one.** A
   seventh kind, `gates:` in `gates/`:

   ```yaml
   gates:
     - id: agent-beliefs-land-for-review
       proposes: belief
       by: [claude_code, codex_cli]   # proposers covered; empty means anyone
       tier: observe                  # decide | observe | delegated
       sample: 5                      # observe only: about one in 5 is audited as if decide
       since: 2026-11-01              # rejections before this day do not tighten the gate
       because: [model-verdicts-are-opinions-not-evidence]
   ```

   Three tiers. `decide`: a human accepts it before it lands; the default
   for any proposal no gate covers. `observe`: it lands and shows up in
   `agc proposals show`; with `sample`, about one in that many is audited
   and needs a decision instead, picked by hashing the id so the same
   proposals are audited whatever order the files load in. `delegated`:
   it lands unseen. When several gates cover a proposal the strictest
   wins. The tier is chosen per gate by the human on reversibility and
   blast radius; a belief by an agent is an opinion by construction and
   costs a line, a piece of evidence by an agent is the one thing the
   model must never author alone.
3. **The fixed point.** No gate covers `proposes: gate`; a gate on gate
   changes is refused when the file loads. A gate change always needs a
   human, and only a human edits `gates/` (rule `agents-never-edit-gates`).
   Tightening happens by itself; loosening is always a human act.
4. **Tightening is computed, never written.** A rejection under an observe
   or delegated gate, on or after the gate's `since`, pulls the gate to
   `decide` for every proposal it covers, and `agc proposals show` and
   `agc gates show` say which rejection did it. To loosen it again the
   human edits the gate and moves `since` forward, which is the human act
   the design wants. Nothing relabels the gate in the file.
5. **Legibility is the gate before any gate.** A proposal that acts (rule,
   decision, action, gate, code) and rests on nothing is illegible: there
   is no why to read before accepting it, so it needs a decision whatever
   its gate says, and its sentence says to reject it rather than trust it.
   A belief or a piece of evidence resting on nothing is an opinion, which
   is a readable thing to be. `agc proposals why <id>` prints the chain
   under a proposal, each belief down to its evidence with its label, so
   the human reads the why and not the proposal alone.
6. **The rejection rate is the health signal.** `agc gates show` prints
   every gate with the tier it gives now and its accepted, rejected, open
   and audited counts, then the default for proposals no gate covers. A
   decide gate that has accepted five or more and rejected none says so:
   either the proposer is perfect or the human has stopped deciding, so
   move it to observe honestly, or audit it.
7. **Commands.** `agc proposals show` groups proposals by what they need
   now: my decision, my eyes, nothing, then accepted and rejected.
   `agc proposals why <id>` reads the chain. `agc proposals accept <id>`
   and `agc proposals reject <id> --why …` write the verdict on the
   proposal in its file and nothing else, `--by` defaulting to the first
   name in `git config user.name`. `agc gates show` prints the gates.
   `--ids` prints the compact form everywhere. `GET /knowledge` now
   returns `proposals` and `gates` with the computed tiers and counts.
8. **Agents land only what their gate allows** (rule
   `agents-land-only-what-their-gate-allows`): a proposal under "Needs my
   decision" waits for `accepted:`; one under observe or delegated the
   proposer may land, and the record stays in `proposals/`.

Left out on purpose: goals as a kind of file (what we want is stated to
the agent, not stored for it to propose against), a `check:` on gates
that would run a command before a delegated tier applies (CI already runs
`agc rules check`; a gate on code should wait for a reason to be finer),
listing open proposals under `agc beliefs doubt`, automatic merging of
anything, and any tracking of who is right more often. The rejections are
the data; reading them is still a human's job.

## Alternatives considered

- **Approval as a field on each kind** (`approved: true` on a rule). It
  says nothing about what the agent proposed or why it was turned down,
  and nothing could compute a tier from it.
- **A review queue in the database.** Proposals are text about text; git
  already records who changed what, and a YAML file an agent can write in
  its worktree is the same mechanism the rest of the knowledge uses.
- **Gates as rules with a `gate:` block.** It would make `rules check`
  list them as manual rules and let the system's own rule machinery touch
  the gates. A separate kind keeps the fixed point one `if`.
- **Writing the tightened tier into the gate file.** It is the same
  mistake as storing a belief's label: it drifts from the rejections it is
  computed from, and a human loosening it could not tell what they were
  undoing.
- **Tracking the human's own accuracy** (were my overrides right?). It
  would need the system to judge the human, which inverts the design; the
  rejections and what later happened are in the files for a human to read.

## Consequences

- An agent's output is now three things: the change itself, a proposal
  record saying what it rests on, and, if its tier lets it land, the
  landing. The proposal record is cheap and the main session can refuse to
  merge a change without one.
- `agc proposals show` is the digest. Its first section is the human's
  queue; if it is empty for a long while on a decide gate with many
  accepted, `agc gates show` says to move the gate or audit it.
- `rules/repo.yaml` gains two judged rules, both resting on this decision;
  `beliefs/repo.yaml` three beliefs, two of them opinions, which is what
  they are.
- `proposals/repo.yaml` starts with one proposal: this design, proposed by
  claude_code and accepted by franco, which became the decision in
  `decisions/repo.yaml`.

## Next step

Have `agc task` attach the open proposals and the gates to a task's
sandbox along with its rulesets (the action
`attach-rulesets-to-task-specs`), so an agent in a sandbox knows what it
may land. Not part of this change.
