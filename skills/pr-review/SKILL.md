---
name: pr-review
description: >-
  Read-only review of pull requests, branches, and diffs. Use when asked to
  review a PR, branch, or diff, assess a change, or review a PR number or
  GitHub PR URL. Return prioritized findings without applying patches.
---

Find defects and unnecessary complexity the author can act on. Favor simple data representations, explicit control flow, clear ownership, and local reasoning. Challenge abstractions, layers, and dependencies without a present need. Understand the constraints before recommending deletion; fewer lines alone are not evidence.

## Boundaries

- Do not edit files, apply or emit patches, change the checkout/index, install dependencies, or publish a review.
- Read with shell, git, and gh; fetching needed refs/objects is allowed.
- Run existing tests/checks after inspecting their scripts/configuration. Local disposable side effects are expected: build outputs, temp files, local containers, and databases or schemas the suite itself creates and drops. Do not rewrite source, lockfiles, or snapshots, and do not touch remote services others share, such as production or staging databases, deployed environments, or third-party APIs with real credentials. Attribute results only to the code actually tested.

## Establish the comparison

- Resolve repository, head commit, and base. Use `gh pr view` for the PR description/base/head and `gh pr diff` for the change. For a branch, use the stated base or its PR base; otherwise use the repository default and state the assumption. Ask when evidence leaves competing bases, such as stacked branches. Never assume the checkout is the PR.
- When commits are available, inspect `git diff <base>...<head>`. Read surrounding files at the reviewed revision with `git show <head>:<path>`; exclude unrelated local changes.

## Investigate before flagging

- Inventory the whole diff, then review in risk order. Trace changed behavior through callers, state changes, error paths, and tests. Check compatibility, migration/deployment ordering, and concurrency where affected.
- Compare the diff with the description. Flag undisclosed behavior changes when they alter a contract, compatibility, or rollout assumption; explain the consequence.
- Where input, auth, config, or secrets change, check for injection, missing authorization, exposed secrets, and unsafe deserialization.
- For a suspected defect, identify a supported input/state, the violated contract, and its consequence. Check the base; seek callers, guards, or tests that disprove it. Report issues introduced or exposed by the change. A clear code trace suffices; distinguish inference from reproduction.
- For complexity, name the present cost in understanding, debugging, or changing the code and a simpler direction that preserves its constraints. Unfamiliarity, personal taste, and hypothetical needs do not qualify.
- Check whether tests exercise changed behavior and failures. Flag missing coverage only for a specific nontrivial behavior left unprotected.
- Consolidate duplicate symptoms. Skip routine lint/format nits, praise, and speculation. Investigate consequential uncertainty or state it as a question. Do not pad or cap findings.

## Report

First line: `Reviewed <head> against <base>. Blockers: <x>. Should fix: <y>. Scope: <files/paths examined>.`

Enumerate findings by impact, with these fields:

1. [Blocker] src/billing/refund.py:42 — Refund can exceed the original charge.
   Scenario: a request amount greater than charge.amount passes validation.
   Evidence: validate_refund checks only amount > 0; both callers, refund_api.py and refund_cli.py, pass the amount through unchanged.
   Fix direction: enforce the charge bound in validate_refund.

Anchor to the smallest relevant changed range. Blocker means demonstrated serious harm or broken core behavior; Should fix means another concrete defect or justified simplification. Severity follows impact, not confidence. For complexity, Scenario names the present cost; give a fix direction only when supported.

If none qualify, say: "No actionable findings in the reviewed scope." Separately state checks run/results, consequential open questions, and material gaps, including unreviewed areas. Do not claim merge safety. If asked for a recommendation, qualify it by evidence and coverage.
