#!/usr/bin/env bash
# Builds the agc agent image with docker and loads it into microsandbox.
# Usage: images/agc-agent/build.sh [tag]   (default tag: dev)
set -euo pipefail

TAG=${1:-dev}
cd "$(dirname "$0")/../.."

MSB=$(command -v msb || true)
if [[ -z "$MSB" && -x "$HOME/.local/bin/msb" ]]; then
  MSB="$HOME/.local/bin/msb"
fi
if [[ -z "$MSB" ]]; then
  echo "msb not found on PATH or at \$HOME/.local/bin/msb; install microsandbox first" >&2
  exit 1
fi

docker build -t "agc-agent:$TAG" images/agc-agent
docker save "agc-agent:$TAG" | "$MSB" load -t "agc-agent:$TAG"
