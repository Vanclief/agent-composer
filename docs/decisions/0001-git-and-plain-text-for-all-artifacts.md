# ADR 0001: Git + plain text for all versioned artifacts

**Status:** Accepted — 2026-07-25

## Context

AGC workflows edit folders of files through LLM harnesses (Claude Code, Codex CLI).
Parallel executions need isolation and a way to combine results
(see `WORKSPACES_PLAN.md` — git worktrees). The artifacts are not only code:
workflows will also produce legal contracts, design decisions, and specs.
The question was whether git should version all of it, or whether a different
version control system fits non-code artifacts better.

## Decision

1. **Git is the single versioning substrate for every artifact type** — code,
   contracts, design decisions, specs.
2. **Artifacts that matter are authored in plain text** (markdown, YAML, LaTeX).
   Binary formats (.docx, PDF) are build outputs generated from text sources
   (e.g. via pandoc), never the source of truth.
3. **AGC's git integration stays thin**: shell out to the `git` CLI, persist
   nothing git already knows (the `WORKSPACES_PLAN.md` principle). This keeps
   the switching cost low if a successor ever earns its place.

## Rationale

- The workers are LLM agents, and git is the only VCS they are deeply trained
  on — branching, diffing, conflict resolution, commit messages. Any other
  system downgrades agent competence.
- One merge model covers every artifact type, but only if artifacts are text.
  Git's weakness is binary diff/merge, and no alternative VCS fixes that;
  text-first authoring does.
- GitHub supplies the human-approves-agent-work layer (review, PRs, CI) at no
  build cost.

## Alternatives considered

- **Jujutsu (jj)** — saner conflict model, but backed by git repos anyway, so
  adopting it later requires no migration; agents barely know it. Watch, don't
  build on.
- **Perforce / SVN** — centralized, built for giant binary assets; wrong shape
  for a local-first agent tool.
- **DVC / git-annex / LFS** — large-data add-ons on top of git, not
  replacements. Revisit only if workflows start producing multi-gigabyte
  artifacts.
- **Docs-style continuous versioning (Drive, Notion)** — linear history, no
  branching; cannot represent parallel agent runs.

## Consequences

- Parallel isolation is git worktrees for repos; non-git folders run in place
  (serialized), per the ranking in `WORKSPACES_PLAN.md` discussion.
- Workflows producing binary deliverables must decide per repo: commit via LFS
  or gitignore as build artifacts. Resolved in
  [ADR 0002](0002-binary-artifact-storage.md): gitignore and regenerate.
- Design decisions land as ADRs in `docs/decisions/` (this file is the first).
