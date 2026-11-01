# agc-agent image

The base image every Task boots from (ADR 0003): Debian bookworm with Node 22,
git, tmux, ripgrep, jq and a few shell tools, plus Claude Code and Codex CLI.
It runs as user `agent` in `/home/agent`, and git authenticates over HTTPS
through a credential helper that reads `GITHUB_TOKEN` (ADR 0004).

Build it with `images/agc-agent/build.sh [tag]` (default tag `dev`). The script
needs `docker` and the microsandbox `msb` CLI, on `PATH` or at
`~/.local/bin/msb`; it runs `docker build`, then `docker save | msb load` so the
image is available to sandboxes as `agc-agent:<tag>`.
