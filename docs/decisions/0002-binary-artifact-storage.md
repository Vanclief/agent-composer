# ADR 0002: Binary artifact storage (images, PDFs, video)

**Status:** Accepted — 2026-07-25
**Builds on:** [0001 — git + plain text for all artifacts](0001-git-and-plain-text-for-all-artifacts.md)

## Context

ADR 0001 makes text the source of truth, but some artifacts are inherently
binary: images, PDFs, video. Workflows do not take large media as input today,
but they are expected to (video processing, photo sets), so the storage
strategy must not be foreclosed by the worktree design
(`WORKSPACES_PLAN.md`).

## Decision

Storage is decided by file class, not by a single tool:

1. **Small, stable binaries** (logos, diagrams, fixtures; <~5 MB, low churn)
   — committed directly to git. Bloat comes from churn, not existence.
2. **Generated binaries** (PDFs rendered from markdown, exports, renders)
   — gitignored and regenerated. Reproducible outputs are a cache, not a
   source; provenance is the text source plus AGC's execution record.
3. **Large or frequently-changing source binaries** (raw video, photo sets)
   — kept out of git entirely. Tracked by a plain-text manifest committed to
   git (`path`, `sha256`, `url`), blobs in object storage (S3/R2/NAS),
   fetched on demand. The manifest is text, so it diffs, merges, and flows
   through worktrees per ADR 0001.

Tier 3 is not built yet. It becomes real when the first large-media workflow
lands; until then the rule is only tiers 1 and 2.

## Alternatives considered

- **Git LFS** — one-system convenience for a mid-size editable class
  (10–100 MB design sources). Rejected as the default: per-clone setup,
  GitHub quotas (video-hostile), and every worktree materializes a full copy
  of every LFS file — parallel runs multiply disk by N. May still be adopted
  later for a specific proven file class.
- **DVC / git-annex** — off-the-shelf versions of the tier-3 manifest model.
  Extra dependency and ceremony, agents less fluent. Revisit only if the DIY
  manifest outgrows ~50 lines of tooling.
- **Committing everything to plain git** — dies on churny or large files;
  also pays the per-worktree copy tax.

## Consequences

- The worktree design must assume tier-3 assets are *not* present in the
  checkout: fetch lazily via the manifest or point at a shared read-only
  cache directory, so N parallel runs do not multiply media on disk.
- Workflows producing binary deliverables gitignore them (tier 2); the
  LFS-vs-gitignore question left open in ADR 0001 is resolved as: gitignore,
  regenerate on demand.
- When large-media input workflows arrive, the manifest schema and fetch step
  get designed then — as part of that feature, not speculatively now.
