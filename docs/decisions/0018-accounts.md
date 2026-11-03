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
   Since task `templates-default-account`, the shipped templates
   (`agents/*.yaml`) name no credential and rely on the default.
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

## Sign-in

A token from `claude setup-token`, what `POST /api/accounts` stores, is
inference-only. The Claude Code 2.1.316 bundle says so ("Long-lived tokens
(from `claude setup-token` or CLAUDE_CODE_OAUTH_TOKEN) are limited to
inference-only"), and `GET /api/oauth/profile` and `GET /api/oauth/usage`
need the `user:profile` scope: on the rig the first poll of account
`holaemma` read `usage: HTTP 429, profile: HTTP 403`. The interactive login
Claude Code runs (`claude auth login`) grants full scopes and a refresh
token, so agc runs the same login per account
(`core/resources/accounts/oauth`, the values read from the bundle cited
there): the claude.ai authorize page with PKCE S256 and a random state,
the manual redirect (the page shows `code#state` to paste), the production
client id, scopes `user:profile user:inference`.

- **Where it lives.** The refresh token is the `secrets.env` line
  `ACCOUNT_<NAME>_LOGIN`, written by the same helpers as the account's
  own line, replaced when a refresh rotates it, removed on sign-out and
  when the account is deleted. The row gains `signed_in_at` (null: no
  login). The access token and its expiry, and a begun sign-in's state and
  verifier (10 minutes, one per account), are in memory only; after a
  restart the first poll refreshes from the refresh token. No route, error
  or log line holds a token or the pasted code. A sign-in whose line name
  is another account's secret (account `x` beside an account `x-login`)
  is refused.
- **What it is for.** Only usage and identity: the poller reads both
  endpoints with the login's access token, refreshing it when missing or
  within 5 minutes of expiry; a refresh refused with 400 or 401 reads
  `sign-in expired: sign in again` and changes nothing else. An account
  with no login is read with its own token as before, and its profile
  status decides: 403 reads `inference-only token: sign in for usage`
  whatever usage said, 401 reads `token rejected: sign in again` (a
  revoked token, which usage alone also answers with 401), anything else
  the status text. Sandboxes keep getting the account's own token: an access token
  expires in hours and a sandbox's environment is fixed at boot, so
  running inference on the login is a later ADR.
- **Endpoints.** `POST /api/accounts/{name}/signin` → `{ url, expires_at }`
  (anthropic oauth accounts only); `POST /api/accounts/{name}/signin/complete
  { code }` takes `code#state` (a bare code while one sign-in is pending),
  exchanges it, writes the line, sets `signed_in_at` and, when empty,
  `email` from the profile, polls the account once and returns the
  account; a missing or lapsed sign-in, a state that does not match or a
  refused exchange (its HTTP status, never its body) is a 400.
  `DELETE /api/accounts/{name}/signin` signs out. `AccountView` carries
  `signed_in_at`.

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

- **Sign in by device flow.** Done as "Sign-in" above, for usage and
  identity only; the account's value is still pasted, and running
  sandboxes on the login is a later ADR.
- **Usage.** Anthropic oauth usage is filled by a poller
  (`core/resources/accounts/usage`) that only `agc rest` runs, every 10
  minutes; `core.NewStack` connects it (`SetLister` with the accounts API,
  `accounts.API.SetUsage` with `accounts.PollerUsage`), and `usage` is null
  until its first reading of the account. Each cycle logs one info line
  with counts only. OpenAI usage is a later follow-up.
