# agc-agent image

The base image every Task boots from (ADR 0003): Debian bookworm with Node 22,
git, tmux, ripgrep, jq and a few shell tools, plus Claude Code and Codex CLI.
It runs as user `agent` in `/home/agent`, and git authenticates over HTTPS
through a credential helper that reads `GITHUB_TOKEN` (ADR 0004).

When agc prepares a Task it merges first-run answers into the `agent` user's
config after the clone and before `setup` (onboarding done, dark theme,
bypass-permissions accepted, the repo or `/home/agent` trusted, in
`~/.claude.json`, `~/.claude/settings.json` and `~/.codex/config.toml`), so
`claude` and `codex` start working without waiting for a key press; the image
needs `jq` for this.

Build it with `images/agc-agent/build.sh [tag]`. The default tag is `latest`,
which is what the Task default image `agc-agent` resolves to. The script needs
`go`, `docker` and the microsandbox `msb` CLI, on `PATH` or at
`~/.local/bin/msb`. It cross-compiles agc for linux (`GOARCH`, default
`amd64`) into this directory with the messaging readme, so the image ships
`agc msg` and `/etc/agc/README.md` (ADR 0005). Then it runs `docker build`,
then `docker save | msb load` so sandboxes can use the image as
`agc-agent:<tag>`.
