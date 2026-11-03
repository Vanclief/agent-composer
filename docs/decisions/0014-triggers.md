# ADR 0014: Outside events become messages in channels, routed by code

**Status:** Accepted — 2026-11-02. Builds on [ADR 0004](0004-secrets-are-placeholders-bound-to-hosts.md), [ADR 0005](0005-messaging-transport-and-identity.md), [ADR 0008](0008-tasks-allocate-compute.md) and [ADR 0009](0009-task-agents-and-managers.md).

## Context

Work in agc reacts to things that happen elsewhere: Franco comments on a
pull request a task opened, a review asks for changes, CI fails on a
task's branch, someone opens an issue. Today nobody in agc hears about
any of it unless a human copies it into a channel. Agents already listen
on channels (ADR 0005, 0009, 0016), so the cheapest way to let them react
is to turn each outside event into a message in the right channel.

Two things are kept out of this mechanism on purpose. Deciding whether an
event is worth acting on is a judgment, so it belongs to an agent (the
main agent, built by the sibling task `main-agent`), not to the code that
fetches events. Time-based triggers already exist as schedules (ADR 0012)
and are not repeated here.

## Decision

1. **A trigger is a plain message posted by agc.** agc posts it with
   `messaging.PostAsSystem` as the reserved participant `agc` (ADR 0009),
   kind `message`; there is no new message kind. The body names the
   source, the actor, what happened and where, and the text:
   `[github] franco commented on PR #12 (agc/dms-whatsapp) in
   vanclief/agent-composer: Please rebase...`, then the link on its own
   line. The text has its whitespace folded and is cut at 400 characters
   with `...`; the link is where the rest is. An event without a known
   actor says `someone`.

2. **Routing is code.** An event about a pull request (or a workflow run)
   whose head branch is `agc/<task>` goes to `#<task>` when a task by that
   name exists, done or not: a comment on a done task's PR still reaches
   the task's channel and whoever reads it. Everything else, issues,
   other branches, branches of tasks that do not exist, goes to `#main`,
   the main agent's channel. A post wakes whoever reads that channel
   through messaging's `OnPost` hooks, as any post does (ADR 0016).

3. **GitHub is the first source, polled.** The rig's services bind to
   127.0.0.1 (`rules/rig.yaml`, `rig-services-bind-localhost`), so
   GitHub has no way to deliver a webhook; agc asks instead. Every
   `triggers.github.poll` (default `60s`, at least `10s`) agc reads, for
   each configured repo, five kinds, each with its own cursor:
   - `issues`: issues and pull requests opened, and closed or merged
     (`GET /repos/{repo}/issues?state=all&since=`);
   - `issue_comments`: comments on issues and pull requests
     (`GET /repos/{repo}/issues/comments?since=`);
   - `review_comments`: comments on a pull request's code
     (`GET /repos/{repo}/pulls/comments?since=`);
   - `reviews`: approvals, change requests and reviews with a body, read
     per pull request from the pull requests updated in the window
     (`GET /repos/{repo}/pulls`, then `/pulls/{n}/reviews`); a commented
     review without a body is the wrapper of review comments already
     posted, and is skipped;
   - `runs`: finished workflow runs (`GET /repos/{repo}/actions/runs?status=completed`),
     with their conclusion, routed by their head branch.
   A pull request's head branch is read once per poll and repo.

4. **The token is a host secret.** agc reads `GITHUB_TOKEN` from the
   secrets file on every poll with `secrets.Load` and `Resolve`, as it
   does for schedule polls (ADR 0012): the call runs on the host, not in
   a sandbox, so it needs the value, and it never prints it. A missing or
   rejected token is the source's `last_error`; the cursor stays where it
   was, so the events of the failed polls post once the token works.

5. **Nothing posts twice, across restarts.** The cursors live in one
   JSON file, `<agc home>/triggers/cursors.json` (mode 0600, written
   through a temporary file and a rename). Each source keeps the time its
   last successful poll started, when it last polled, its last error, how
   many messages it posted, and the keys it has seen. A poll asks for
   everything since the cursor less five minutes, so an event GitHub
   lists late still arrives, posts the events whose key it has not seen,
   oldest first, saving the file after each, and then moves the cursor
   and forgets keys older than the new window. A source's first poll only
   sets its cursor to now: history is not posted. A file, not a table:
   one process writes it, nothing queries it, and it needs no migration.

6. **The fast-path filter is config; the semantic filter is the main
   agent's.** The config file's `triggers:` block, read like `defaults:`:

   ```json
   "triggers": {
     "github": {
       "repos": ["vanclief/agent-composer"],
       "ignore_users": ["dependabot[bot]"],
       "poll": "60s",
       "post_own": false
     }
   }
   ```

   An empty `repos` list means triggers are off. Events by a login in
   `ignore_users` (case-insensitive) are marked seen and not posted.

   agc writes to GitHub as the user behind `GITHUB_TOKEN`, so by default
   its own events are dropped too: each poll asks GitHub who the token is
   (`GET /user`, its `login`, once per poll) and an event whose actor is
   that login (case-insensitive) is marked seen and not posted, like an
   ignored user's. If `GET /user` fails, the failure is logged and the
   poll goes on, posting own events that once. `post_own: true` (config
   only, default false) skips the lookup and posts own events as before.

7. **Only `agc rest` polls.** `core.Stack.StartScheduler`, which only
   `agc rest` calls, starts the poll loop next to the scheduler; CLI
   commands that build the stack in-process never poll. `agc triggers
   show [--json]` reads the config and the cursor file and lists every
   source with its cursor, last poll, posted count and last error, and
   any source in the file that is no longer configured. There is no REST
   route yet.

## Rationale

- A message is what every agent already reads, waits on and answers; a
  new kind or a new table of events would need its own readers.
- Routing by branch name is exact for the PRs agc's own tasks open
  (`agc/<task>`, AGENTS.md's subagent workflow), and anything it cannot
  place lands with the one agent whose job is to place things.
- Polling costs a few requests a minute per repo and needs no public
  entrance to a machine that has none and should not get one.
- A cursor with an overlap and a set of seen keys survives GitHub's
  eventual consistency and agc restarts without a second post.

## Consequences

- Package `core/resources/triggers` (`API.Start`, `PollOnce`, `Route`,
  `Show`); settings `controller.TriggersSettings`; CLI `agc triggers show`.
- Tests run the poller only against `httptest`; nothing in CI calls
  GitHub.
- Left open (actions in `actions/triggers.yaml`): more sources (check runs
  from apps other than Actions, other forges, mail, chat), and inbound
  webhooks, which need a public entrance to agc that the rig rules
  forbid today.
- A poll reads at most ten pages of 100 per kind (the reviews kind walks
  at most 300 pull requests, the runs kind the 100 newest finished runs).
  A busy repo after a long downtime can exceed that, and what lies past
  it is not posted.
