# agc-agent image

The base image every Task boots from (ADR 0003). It contains:

- Debian bookworm with Node 22 (`node:22-bookworm`), git, tmux, ripgrep, jq,
  make, curl, ssh, less, procps and vim-tiny.
- Claude Code and Codex CLI.
- Go from the official tarball in `/usr/local/go`, at the version in
  `go.mod` (`gofmt` comes with it). `GOPATH` is `/home/agent/go`; `PATH` has
  `/usr/local/go/bin` and `$GOPATH/bin`, also in login shells through
  `/etc/profile.d/go.sh`. `GOFLAGS` is not set.
- The system libraries Playwright's Chromium needs (`playwright install-deps
  chromium`, same Playwright version as `panel/package.json`). The browser
  itself is not in the image: it is ~300 MB and `npx playwright install
  chromium` puts it in `~/.cache/ms-playwright` when a Task needs it.
- `agc` (for `agc msg`) and the messaging readme at `/etc/agc/README.md`.

It runs as user `agent` in `/home/agent`, and git authenticates over HTTPS
through a credential helper that reads `GITHUB_TOKEN` (ADR 0004).

When agc prepares a Task it merges first-run answers into the `agent` user's
config after the clone and before `setup` (onboarding done, dark theme,
bypass-permissions accepted, the repo or `/home/agent` trusted, in
`~/.claude.json`, `~/.claude/settings.json` and `~/.codex/config.toml`), so
`claude` and `codex` start working without waiting for a key press; the image
needs `jq` for this.

## What an agent can run

In a clone of agent-composer inside a Task:

- `go build ./... && go vet ./... && go test ./...`
- `gofmt -l .` (or `scripts/check-gofmt.sh`)
- `make types`
- in `panel/`: `npm ci`, `npx playwright install chromium`, `npm run check`
  (`make panel-check` does the last two)

## Rebuilding and loading it on the rig

On the rig, from the repo root: `images/agc-agent/build.sh [tag]`. The default
tag is `latest`, which is what the Task default image `agc-agent` resolves to.
The script needs `go`, `docker` and the microsandbox `msb` CLI, on `PATH` or
at `~/.local/bin/msb`. It cross-compiles agc for linux (`GOARCH`, default
`amd64`) into this directory with the messaging readme, so the image ships
`agc msg` and `/etc/agc/README.md` (ADR 0005). Then it runs `docker build`,
then `docker save | msb load` so sandboxes can use the image as
`agc-agent:<tag>`.

The Dockerfile pins Go (`GO_VERSION` and the two `GO_SHA256_*` sums from
https://go.dev/dl/) and Playwright (`PLAYWRIGHT_VERSION`). `build.sh` stops
if `GO_VERSION` differs from the `go` line in `go.mod`; when go.mod moves,
bump the three Go args together. Bump `PLAYWRIGHT_VERSION` with
`@playwright/test` in `panel/package.json`.
