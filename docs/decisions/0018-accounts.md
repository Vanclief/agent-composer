# ADR 0018: Accounts are rows plus secrets.env lines, with a default per provider

**Status:** Proposed — 2026-11-02. Builds on [ADR 0004](0004-secrets-are-placeholders-bound-to-hosts.md) ("Accounts"), [ADR 0008](0008-tasks-allocate-compute.md), [ADR 0009](0009-task-agents-and-managers.md) and [ADR 0016](0016-persistent-agents.md).

## Context

ADR 0004 "Accounts" lets a spec run a harness on a named secret:
`ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN`. That works, but every spec and
template has to spell the entry, the user edits `secrets.env` by hand, and
which account a task runs on depends on the plain name a spec happened to
list. Franco, 2026-11-02: "I don't want to have to do it manually, I want a
UI for that. I also don't want that the default is based on plain name, I
want something like this in Orca where I can manage them": a provider card
with usage bars, the active account's email, a "Switch to" list with a
System default entry and the other accounts, and "Manage Accounts…".

## Decision

1. **An account is a row plus a `secrets.env` line.** Table `accounts`:
   `name` (unique, `[a-z0-9-]{1,32}`), `provider` (`anthropic | openai`),
   `kind` (`oauth | api_key`), `secret`, `email`, `is_default`
   (`default` in the API), `created_at`. `secret` is the `secrets.env` name
   holding the value, `ACCOUNT_<NAME>` (upper-cased, dashes as
   underscores). The value is only ever in `secrets.env`, as ADR 0004 put
   it: the row holds no value, no route returns one, no error or log line
   holds one (`POST /api/accounts` is logged without its body's value).
2. **Provider and kind decide the variable.** `anthropic` accounts serve
   `claude_code`, `openai` accounts `codex_cli`. The sandbox receives an
   anthropic `oauth` account as `CLAUDE_CODE_OAUTH_TOKEN`, an anthropic
   `api_key` as `ANTHROPIC_API_KEY`, an openai `api_key` as
   `CODEX_API_KEY`; openai has no `oauth` kind yet.
3. **One default per provider.** At most one account per provider has
   `default` set: `POST /api/accounts/{name}/default` takes the flag off the
   provider's previous default in the same transaction, a partial unique
   index on `provider` where `is_default` backs it, and
   `DELETE /api/accounts/{name}/default` leaves the provider with none.
   Nothing picks a default on its own, not even the first account.
4. **Where the default applies.** When a sandbox is allocated
   (`AllocateFor` for a task, `AllocateHome` for a persistent agent) or
   started, and its spec's `secrets` set none of the variables its harness
   reads (`runtime/harnesses/catalog.go`), agc appends
   `<account.secret> as <variable>` for the provider's default and stores
   it in the sandbox's spec. The harness is the task's assignee when it is
   a harness, the agent's harness for a home. An explicit entry wins,
   whatever secret it names; no default, a person's task or a harness no
   provider serves (pi) means no change: the sandbox gets no credential, as
   before. So templates and specs can stop listing
   `CLAUDE_CODE_OAUTH_TOKEN`; editing them is left to whoever owns them.
5. **Writes.** `POST /api/accounts` checks the name is free and the secret
   is not already set in `secrets.env`, then writes
   `ACCOUNT_<NAME>='<value>'` (quoted the way `secrets.Load` unquotes it)
   and inserts the row under one transaction. `secrets.env` is rewritten
   whole, atomically (a 0600 temp file renamed over it in the same
   directory), keeping every other line, and created 0600 when missing.
   `DELETE /api/accounts/{name}` removes the row and the line, and is
   refused (409) while a running or booting sandbox's stored spec names the
   secret; since a bound default is stored in the spec, that covers it.
6. **What the UI shows** is the contract of the accounts tasks:
   `GET /api/accounts` returns `{ accounts: [AccountView], providers:
   [ProviderView] }`, an account with `usage: UsageView | null`
   (`{ updated_at, windows: [{ id, label, used, resets_at }], error }`), a
   provider with the harnesses it serves and its variable per kind
   (`docs/openapi.yaml`, tag Accounts). The panel's provider card, "Switch
   to" list and "Manage Accounts…" are built on it.

## Consequences

- Which account a task runs on is a choice made once, per provider, in
  the UI, not a name copied into every spec.
- A sandbox records the account it was given, so changing the default
  later does not move running or stopped sandboxes to another account; a
  sandbox allocated before its provider had a default picks it up on its
  next start.
- The `ACCOUNT_` prefix is reserved for accounts: a hand-written
  `ACCOUNT_X` line blocks creating account `x` until it is removed.

## Follow-ups

- **Sign in by device flow.** Today the value is pasted; signing in from
  the panel (an OAuth device flow, `claude setup-token` style) would write
  the same line and row, and let a greyed "Sign in again to use this
  account" entry refresh an expired token.
- **Usage.** Anthropic oauth usage is filled by a fetcher
  (`core/resources/accounts/usage`, set with `accounts.API.SetUsage`);
  until it is wired, `usage` is null. OpenAI usage is a later follow-up.
