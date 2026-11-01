# agc-agent image

The base image every Task boots from (ADR 0003): Debian bookworm with Node 22,
git, tmux, ripgrep, jq and a few shell tools, plus Claude Code and Codex CLI.
It runs as user `agent` in `/home/agent`, and git authenticates over HTTPS
through a credential helper that reads `GITHUB_TOKEN` (ADR 0004).

Build it with `images/agc-agent/build.sh [tag]`. The default tag is `latest`,
which is what the Task default image `agc-agent` resolves to. The script needs
`docker` and the microsandbox `msb` CLI, on `PATH` or at `~/.local/bin/msb`. It
runs `docker build`, then `docker save | msb load` so sandboxes can use the
image as `agc-agent:<tag>`.
