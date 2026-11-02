# ADR 0004: Secrets are placeholders bound to hosts

**Status:** Accepted — 2026-11-01

## Context

Today harnesses run as host subprocesses and inherit the host environment, so
Claude Code and Codex use whatever the host user is logged in as, and `gh`
and `git` use the host's credential store. Inside a Task
([ADR 0003](0003-tasks-run-in-microsandbox-vms.md)) none of that exists. Each
Task must be handed the credentials its agents need, and an agent that can
run arbitrary code must not be able to read or exfiltrate them.

microsandbox offers placeholder secrets: the guest environment variable holds
a dummy value (the variable name with a fixed prefix; never spelled out here, see rule `no-literal-secret-placeholders`), and the runtime's TLS-intercepting
proxy swaps in the real value only in requests to an allow-listed host. Any
other destination is blocked and logged. The 2026-11-01 spike confirmed this
works for Claude Code on a subscription OAuth token (`api.anthropic.com`), for
the GitHub API and for `git clone` over HTTPS through a credential helper
(`api.github.com`, `github.com`), and that a request carrying the placeholder
to a third host is refused with `secret_policy_denied`.

Limits of the mechanism: HTTPS only (no SSH keys, no guest-side signing);
hosts on the allow list receive the real value and could echo it back; adding
a secret to a running sandbox needs a restart; raw values passed through the
SDK are persisted in the host-side sandbox config, environment references are
not.

## Decision

1. **Three tiers, named in the Task spec, never valued there.**

   ```yaml
   environment:
     secrets: [GITHUB_TOKEN]                           # placeholders, host-bound
     env:
       GOFLAGS: -mod=mod                                 # plain, readable by agents
     files:
       - codex-auth:/home/agent/.codex/auth.json         # copied in, fully exposed
   ```

   `secrets` is the default and the only tier agc recommends. The harness
   credential comes from the provider's default account (ADR 0018) unless
   the spec names one. `env` is for
   non-sensitive configuration. `files` exists for credentials that cannot be
   substituted (rotating tokens a program rewrites, SSH keys) and the UI
   marks every file entry as exposed to the agent.

2. **Values live in `~/.agent_composer/config/secrets.env`**, mode 0600, one
   `NAME=value` per line, edited by the user. agc reads it when creating a
   Task and passes each requested name as a microsandbox environment
   reference, never as an inline value, so nothing is copied into the
   sandbox config. The REST API and control panel list names only; there is
   no endpoint that returns a value. A system keyring backend can replace the
   file later behind the same names.

3. **Known names carry their allowed hosts.** agc ships a table so the common
   cases need no configuration:

   | Name | Allowed hosts |
   |---|---|
   | `ANTHROPIC_API_KEY`, `CLAUDE_CODE_OAUTH_TOKEN` | `api.anthropic.com` |
   | `OPENAI_API_KEY`, `CODEX_API_KEY` | `api.openai.com` |
   | `GITHUB_TOKEN` | `api.github.com`, `github.com` |

   Any other name must declare hosts in the spec, `NAME@host1,host2`, or
   creation fails with a message naming the missing hosts. Substitution is
   headers only, which covers bearer tokens and Basic auth; query and body
   substitution are opt-in per entry.

4. **Violations block and log.** The default microsandbox action. The log
   line is attached to the Task so the control panel can show "an agent tried
   to send `GITHUB_TOKEN` to example.com" next to the Task. Terminate-on-
   violation is a per-Task option, off by default.

5. **Harness authentication is environment-based inside Tasks.** Claude Code
   through `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` (from
   `claude setup-token`); Codex through `CODEX_API_KEY` (read by `codex
   exec`). Interactive
   `claude login` inside a Task is possible but writes a token to the Task's
   disk, is not substituted, and is lost on delete, so the UI does not offer
   it. Codex on a ChatGPT subscription needs its rotating `auth.json` and
   therefore falls into `files`. (2026-11-02, Franco: list what the current
   Codex CLI reads; `codex exec` authenticates with `CODEX_API_KEY`, and
   reads `OPENAI_API_KEY` only to prefill the TUI login prompt.)

6. **Git uses HTTPS with the credential helper baked into the base image**,
   reading `GITHUB_TOKEN`. SSH remotes are not supported inside Tasks.

## Accounts

Added 2026-11-02, on Franco's ask for several accounts per provider. Each
harness reads one fixed variable (Claude Code `CLAUDE_CODE_OAUTH_TOKEN`, then
`ANTHROPIC_API_KEY`; Codex in `codex exec` `CODEX_API_KEY`), so with one name
per variable a rig could hold one account per provider. An account is a named
secret a sandbox receives under the variable the harness reads:

```yaml
secrets:
  - ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN
  - OPENAI_PERSONAL as CODEX_API_KEY@api.openai.com
  - GITHUB_TOKEN
```

- **Syntax.** `NAME as ENV[@host1,host2]`. `NAME` is the entry in
  `secrets.env`, `ENV` the guest variable. Without `as`, `ENV` is `NAME`, as
  before, so `NAME` and `NAME@hosts` mean what they meant. `ENV` must look
  like an environment variable name (`[A-Z_][A-Z0-9_]*`), and an account
  may not share its `ENV` with another entry of the spec; both are spec
  validation errors that quote the entry. A plain `NAME` listed twice is
  left as it was before accounts, so no spec that validated stops
  validating.
- **Hosts.** From the entry when it names them, else the table above for
  `ENV`, else the table for `NAME`, else creation fails asking for
  `NAME as ENV@host`. So `ANTHROPIC_WORK as CLAUDE_CODE_OAUTH_TOKEN` is bound
  to `api.anthropic.com` with no hosts written. `CODEX_API_KEY` is in the
  table, bound to `api.openai.com`.
- **Which variable.** `GET /api/secrets` returns, besides the names,
  `harnesses: [{ id, reads }]`: each harness in the catalog
  (`runtime/harnesses/catalog.go`) with the variables it takes its
  credentials from, in order of preference, so a client can offer "use this
  secret as the variable claude_code reads".
- **Persistent agents.** A role's template (`agents/<role>.yaml`,
  `sandbox.secrets`, ADR 0016) takes the same entries, so a role runs on a
  given account. A per-agent override at `agc agent create` is not offered
  yet; the template decides.
- **Still never readable.** Nothing else changes: the guest variable `ENV`
  holds a placeholder, and the proxy swaps in the value of `NAME` only in
  requests to the bound hosts. No request returns the value.
- A schedule's poll command reads its secrets on the host by their own
  names, so `as` is refused there.
- **Managed accounts.** [ADR 0018](0018-accounts.md) keeps accounts as rows
  plus `ACCOUNT_<NAME>` lines of `secrets.env`, with a default per provider
  bound as `<secret> as <variable>` when a spec names no credential of its
  harness.

## Rationale

- An agent with shell access and a plain secret in its environment can leak
  it with one command. Placeholders make the common leak paths (echo, log,
  commit, post to a webhook) harmless by construction, at zero cost to the
  programs that use the credential correctly.
- Names-not-values in specs means specs are safe to commit, share and show
  in the UI, consistent with [ADR 0001](0001-git-and-plain-text-for-all-artifacts.md).
- A shipped host table means the first Task needs no secrets configuration
  beyond pasting two values into one file.
- Environment references instead of raw SDK values keep secret material out
  of `~/.microsandbox` entirely; the only copy is the 0600 file.

## Alternatives considered

- **Plain environment variables, trust the agent.** What the host executor
  does today. Acceptable on a developer's own machine, not in a system meant
  to run many agents unattended. Kept only as the `env` tier for
  non-secrets.
- **A secrets service agents call at runtime (Vault-style).** Agents would
  still hold the value after fetching it; adds a server and a token to
  bootstrap the token. The proxy approach needs neither.
- **Per-Task short-lived tokens minted by agc.** Right for GitHub (installation
  tokens) and worth adding later for `GITHUB_TOKEN` specifically; does not
  exist for Anthropic or OpenAI subscriptions, so it cannot be the base
  mechanism.
- **Mount the host's `~/.claude` and `~/.config/gh` into the Task.** Gives
  the agent the user's full identity and rotating credentials in writable
  form; one bad `cat` away from leaking. Rejected.

## Consequences

- The base image must trust microsandbox's proxy CA, which the runtime
  handles for standard TLS stacks; a tool that pins certificates will fail
  inside Tasks and needs its host added as passthrough or its credential
  moved to `files`.
- Changing a secret's value or adding a secret to a running Task requires a
  Task restart. The UI says so on the control that does it.
- `secrets.env` is the single file to back up or move between hosts; it must
  never enter git. The repo `.gitignore` only covers `.env` today and must
  also exclude `.agent_composer/` for the project-local data dir case.
- When a new API host is needed, the first failure is a logged violation
  naming the host, which is the prompt to extend the allow list.
- The `host` executor ignores `secrets` and `files` and warns that it is
  running with the host's own credentials.
