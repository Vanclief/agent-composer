#!/usr/bin/env bash
# Builds the agc agent image with docker and loads it into microsandbox.
# Usage: images/agc-agent/build.sh [tag]   (default tag: latest)
set -euo pipefail

TAG=${1:-latest}
cd "$(dirname "$0")/../.."

MSB=$(command -v msb || true)
if [[ -z "$MSB" && -x "$HOME/.local/bin/msb" ]]; then
  MSB="$HOME/.local/bin/msb"
fi
if [[ -z "$MSB" ]]; then
  echo "msb not found on PATH or at \$HOME/.local/bin/msb; install microsandbox first" >&2
  exit 1
fi

# The image pins Go (version and checksums) in its Dockerfile; it must match
# the go directive in go.mod so agents in the sandbox build with the same Go.
GOMOD_GO=$(awk '$1 == "go" { print $2 }' go.mod)
IMAGE_GO=$(sed -n 's/^ARG GO_VERSION=//p' images/agc-agent/Dockerfile)
if [[ "$GOMOD_GO" != "$IMAGE_GO" ]]; then
  echo "go.mod says go $GOMOD_GO but the Dockerfile pins GO_VERSION=$IMAGE_GO;" >&2
  echo "update GO_VERSION and both GO_SHA256_* (from https://go.dev/dl/)" >&2
  exit 1
fi

# The guest agc is only a messaging client (agc msg), so it needs no cgo.
# GOARCH must match the host's VMs; override it on arm64 hosts.
CGO_ENABLED=0 GOOS=linux GOARCH=${GOARCH:-amd64} go build -o images/agc-agent/agc .
cp messaging/guest-readme.md images/agc-agent/README.guest.md

docker build -t "agc-agent:$TAG" images/agc-agent
docker save "agc-agent:$TAG" | "$MSB" load -t "agc-agent:$TAG"
