#!/usr/bin/env bash
# Deploy the current local branch to the rig (Linux dev machine) over SSH,
# rebuild agc there and restart `agc rest`.
set -euo pipefail

AGC_RIG_HOST="${AGC_RIG_HOST:-192.168.0.45}"
AGC_RIG_SRC="${AGC_RIG_SRC:-~/Projects/agc-src}"
AGC_RIG_RUN="${AGC_RIG_RUN:-~/Projects/agc-tasks}"
AGC_RIG_PORT="${AGC_RIG_PORT:-1202}"

usage() {
  cat <<EOF
Usage: $(basename "$0") [--force] [--dry-run] [-h|--help]

Deploys the current local branch to the rig: fetches origin/<branch> in
\$AGC_RIG_SRC, runs 'make install', restarts 'agc rest' in \$AGC_RIG_RUN and
waits for http://127.0.0.1:\$AGC_RIG_PORT/api/tasks to answer.

The rig builds origin/<branch>, so the branch must be pushed first.

Options:
  --force     skip the clean-tree and pushed-branch checks
  --dry-run   print the remote script instead of running it (no SSH)
  -h, --help  show this help

Environment (current value):
  AGC_RIG_HOST  ssh host            (${AGC_RIG_HOST})
  AGC_RIG_SRC   git checkout on rig (${AGC_RIG_SRC})
  AGC_RIG_RUN   server working dir  (${AGC_RIG_RUN}, log: agc-rest.log)
  AGC_RIG_PORT  server port         (${AGC_RIG_PORT})
EOF
}

force=0
dry_run=0
for arg in "$@"; do
  case "$arg" in
    --force) force=1 ;;
    --dry-run) dry_run=1 ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      usage >&2
      exit 2
      ;;
  esac
done

die() {
  echo "error: $*" >&2
  exit 1
}

branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" != "HEAD" ] || die "detached HEAD; check out a branch first"

if [ "$force" -eq 0 ]; then
  [ -z "$(git status --porcelain)" ] || die "uncommitted changes; commit them or use --force"
  upstream="$(git rev-parse '@{u}' 2>/dev/null)" || die "branch '$branch' has no upstream; push it first or use --force"
  [ "$(git rev-parse '@')" = "$upstream" ] || die "branch '$branch' differs from its upstream; push it first or use --force"
fi

# Values are quoted locally with %q; the body below is a quoted heredoc so
# nothing in it expands on this machine.
remote_script="set -euo pipefail
BRANCH=$(printf '%q' "$branch")
SRC=$(printf '%q' "$AGC_RIG_SRC")
RUN=$(printf '%q' "$AGC_RIG_RUN")
PORT=$(printf '%q' "$AGC_RIG_PORT")
HOST=$(printf '%q' "$AGC_RIG_HOST")
$(
  cat <<'REMOTE'
export PATH="$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin:$PATH"
SRC="${SRC/#\~/$HOME}"
RUN="${RUN/#\~/$HOME}"

# The rig's global git config rewrites GitHub HTTPS to SSH and it has no key,
# so every git network command runs with GIT_CONFIG_GLOBAL=/dev/null.
echo "==> fetching"
cd "$SRC"
GIT_CONFIG_GLOBAL=/dev/null git fetch -q origin
if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
  git checkout -q "$BRANCH"
else
  git checkout -q -b "$BRANCH" --track "origin/$BRANCH"
fi
GIT_CONFIG_GLOBAL=/dev/null git reset -q --hard "origin/$BRANCH"

echo "==> building"
make install

# Never pkill -f: it matches and kills this SSH session's shell.
echo "==> restarting"
pkill -x agc || true
cd "$RUN"
(nohup agc rest > agc-rest.log 2>&1 < /dev/null &)

for _ in $(seq 20); do
  if curl -fsS -o /dev/null "127.0.0.1:$PORT/api/tasks" 2>/dev/null; then
    echo "==> ok $(git -C "$SRC" rev-parse --short HEAD) on $HOST"
    agc version
    exit 0
  fi
  sleep 1
done

echo "agc rest did not answer on 127.0.0.1:$PORT/api/tasks within 20s; last 40 lines of $RUN/agc-rest.log:" >&2
tail -n 40 agc-rest.log >&2
exit 1
REMOTE
)"

if [ "$dry_run" -eq 1 ]; then
  printf '%s\n' "$remote_script"
  exit 0
fi

echo "==> deploying $branch to $AGC_RIG_HOST"
ssh "$AGC_RIG_HOST" 'bash -s' <<<"$remote_script"
