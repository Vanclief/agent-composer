# YC companies near agc

**How this was gathered, and what it is not.** The task asked for a read of the YC directory. The directory
(ycombinator.com/companies) renders from a search API in the browser, and I did not query that API directly:
the command was blocked, and I did not work around the block. Headless Chromium also refused the sandbox's TLS
certificate, so I did not use it either. What follows comes from two web searches limited to ycombinator.com, and
the one-line descriptions are search-result snippets, not company profiles I read. Batches and details may be wrong.
Franco should check each company's page and walk the directory himself before relying on this list.

agc, in one line, for comparison: a git-and-YAML layer where beliefs cite evidence a human witnessed, agents can
only *propose*, and gates decide what lands (ADR 0006, 0007).

## Knowledge and memory for agents

| Company | What the snippet says | How agc differs |
|---|---|---|
| Hyper (P26), [Launch HN](https://news.ycombinator.com/item?id=48387095) | Knowledge graph of facts with typed edges (in tension with, derived from, supersedes); keeps superseded facts. | Closest in shape. agc also types the *source*: evidence has a human or a mechanical check as `who`, never a model, so a graph node cannot ground itself. Plain files in git, not a hosted graph. |
| Memory Store, [launch](https://www.ycombinator.com/launches/QPs-memory-store-your-company-s-brain) | Company memory from Slack, Gmail and Granola; auto-updating briefs including Decision Logs. | It extracts memory from conversations with a model. agc refuses that: a model's reading is an opinion until a person witnesses it. |
| Mosaic, [launch](https://www.ycombinator.com/launches/T3K-mosaic-shared-memory-for-your-team-s-agents) | Syncs coding-agent sessions across a team: what was tried, what changed, why. | Records what agents did. agc records what a human holds true and what must not be done, and checks it. |
| Memorable, [launch](https://www.ycombinator.com/launches/U2j-memorable-graph-procedural-memory-for-ai-agents-stop-re-teaching-your-agents) | Graph procedural memory from tool-call traces, so agents reuse workflows. | Optimizes agent behaviour. agc constrains it with rules and gates and keeps the human's final say. |
| Hyperspell, [page](https://www.ycombinator.com/companies/hyperspell) | Live, permissioned context graph over a company's tools. | Retrieval of context. agc is not retrieval; it is a record of why something is believed and what rests on it. |
| Screenpipe (S26), [Launch HN](https://news.ycombinator.com/item?id=49024620) | Searchable memory of what you saw, said and heard. | Capture of raw material. agc would treat that capture as a source a person cites, not as belief. |
| Skillsync (W26) | Makes agent session history searchable and portable across coding agents. | Session portability; no notion of grounded vs. ungrounded claims. |
| Graphify Labs (S26) | Knowledge graph control plane for enterprise software. | Enterprise knowledge graph; agc is per-repo, plain text, single human owner today. |
| WonderSearch (F26) | Search for agents, meaning-based, no embeddings or ingestion. | Search. agc has no search product. |

## Agent tooling and control planes

| Company | What the snippet says | How agc differs |
|---|---|---|
| Omnara (S25) | Open-source control plane for managed agents. | Runs and supervises agents. agc's gates decide which *proposals* land and track rejection rate; it overlaps only at supervision. |
| Replicas (P26) | Delegates tasks to Claude Code or Codex from an app, Slack or Linear. | Task delegation. agc has tasks in microsandbox VMs too, but the product claim is the knowledge layer, not dispatch. |
| Zenbu (P26) | Extensible IDE for orchestrating coding agents. | Orchestration UI. |
| Centauri AI (W24), coSlash | Management layer for parallel coding-agent sessions. | Orchestration; outside the two-year window, listed for context. |
| Inkbox (S26) | Persistent identities for agents: email and phone. | Identity for agents to act in the world. agc's messaging identity is internal (ADR 0005). |
| Keet (S24) | Authentication for AI agents. | Outside the window. agc binds secrets to hosts as placeholders (ADR 0004), a narrower thing. |
| dari.dev (F25) | Deploy agents as API endpoints. | Deployment. |
| Clad Labs, Kestrel AI (F25) | Appeared only as F25 entries; I could not tell what they build. | Unknown. Check. |

## Agent observability

| Company | What the snippet says | How agc differs |
|---|---|---|
| Papaya (F26) | Optimization engine for agents from production traces. | Improves agent output from traces. agc governs what the human accepts. |
| Hue (F26) | Turns production agent runs into repeatable tests. | Tests agent behaviour. agc's rules can run mechanical checks (`agc rules check`), but about a repo, not agent runs. |
| Laminar (S24) | Open-source observability for agents. | Outside the window; observability, not belief. |

## Decision software

| Company | What the snippet says | How agc differs |
|---|---|---|
| Decisional, [launch](https://www.ycombinator.com/launches/Lcw-decisional-an-ai-agent-for-financial-analysts) | AI agent for financial analysts. | Vertical agent. Name overlap only. |

I found no YC company in the results whose product is "decision records grounded in witnessed evidence with
human-gated agent proposals". That is a finding about two searches, not about the directory. It may mean the niche
is empty, or that I did not look well enough; Franco should not quote it as fact.

## Sources

- https://www.ycombinator.com/companies/industry/ai-assistant, /aiops, /developer-tools, /open-source (search results)
- Links in the tables above.
