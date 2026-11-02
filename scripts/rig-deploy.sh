#!/usr/bin/env bash
# Deploy the current local branch to the rig (Linux dev machine) over SSH,
# rebuild agc there and restart `agc rest`. When `systemctl --user` works
# on the rig it installs the user unit agc-rest, so agc comes back after a
# reboot and starts its crashed VMs again itself (ADR 0017). There is no msb
# server to start: every VM is an `msb machine` child process of agc.
set -euo pipefail

AGC_RIG_HOST="${AGC_RIG_HOST:-192.168.0.45}"
AGC_RIG_SRC="${AGC_RIG_SRC:-~/Projects/agc-src}"
AGC_RIG_RUN="${AGC_RIG_RUN:-~/Projects/agc-tasks}"
AGC_RIG_PORT="${AGC_RIG_PORT:-1202}"
# Knowledge directory the server reads (AGC_KNOWLEDGE_DIR). The checkout
# itself by default, so Think mode shows this repo's beliefs and rules.
AGC_RIG_KNOWLEDGE="${AGC_RIG_KNOWLEDGE:-$AGC_RIG_SRC}"
# The person tasks escalate to and are owned by when none is given (ADR 0009).
AGC_RIG_OWNER="${AGC_RIG_OWNER:-franco}"

usage() {
  cat <<EOF
Usage: $(basename "$0") [--force] [--dry-run] [-h|--help]

Deploys the current local branch to the rig: fetches origin/<branch> in
\$AGC_RIG_SRC, runs 'make install', restarts 'agc rest' in \$AGC_RIG_RUN and
waits for http://127.0.0.1:\$AGC_RIG_PORT/api/tasks to answer.

When 'systemctl --user' works on the rig it installs and enables the user
unit agc-rest.service and restarts agc with 'systemctl --user restart' once
lingering is on; otherwise
it starts 'agc rest' with nohup as before. Starting them at boot without a
login needs lingering, which only Franco turns on, once:
  sudo loginctl enable-linger <user>
The deploy prints that line while lingering is off.

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
  AGC_RIG_KNOWLEDGE  knowledge dir  (${AGC_RIG_KNOWLEDGE})
  AGC_RIG_OWNER  default task owner  (${AGC_RIG_OWNER})
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
KNOWLEDGE=$(printf '%q' "$AGC_RIG_KNOWLEDGE")
OWNER=$(printf '%q' "$AGC_RIG_OWNER")
$(
  cat <<'REMOTE'
export PATH="$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin:$PATH"
SRC="${SRC/#\~/$HOME}"
RUN="${RUN/#\~/$HOME}"
KNOWLEDGE="${KNOWLEDGE/#\~/$HOME}"

# The rig's global git config rewrites GitHub HTTPS to SSH and it has no key,
# so every git network command runs with GIT_CONFIG_GLOBAL=/dev/null.
echo "==> fetching"
cd "$SRC"
GIT_CONFIG_GLOBAL=/dev/null git fetch -q origin
if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
  git checkout -q -f "$BRANCH"
else
  git checkout -q -f -b "$BRANCH" --track "origin/$BRANCH"
fi
GIT_CONFIG_GLOBAL=/dev/null git reset -q --hard "origin/$BRANCH"

echo "==> building"
make install

# The user unit of ADR 0017. It carries paths and the two settings below,
# never a secret: agc reads its secrets from its own files. KillMode=process
# because every VM is an `msb machine` child process of agc, in agc's
# cgroup: the default control-group kill would take every running VM down
# with a restart of agc. Its PATH reaches msb, which agc runs.
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
UNIT_PATH="$HOME/.local/bin:$HOME/go/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"
msb_bin="$(command -v msb || true)"
if [ -n "$msb_bin" ]; then
  msb_dir="$(dirname "$msb_bin")"
  case ":$UNIT_PATH:" in
    *":$msb_dir:"*) ;;
    *) UNIT_PATH="$msb_dir:$UNIT_PATH" ;;
  esac
fi

agc_unit() {
  cat <<UNIT
# Installed by scripts/rig-deploy.sh (ADR 0017). Do not edit: the next deploy rewrites it.
[Unit]
Description=agc rest

[Service]
Type=simple
WorkingDirectory=$RUN
ExecStart=$1 rest
Environment=PATH=$UNIT_PATH
Environment=AGC_KNOWLEDGE_DIR=$KNOWLEDGE
Environment=AGC_DEFAULT_OWNER=$OWNER
StandardOutput=append:$RUN/agc-rest.log
StandardError=append:$RUN/agc-rest.log
Restart=on-failure
RestartSec=5
KillMode=process

[Install]
WantedBy=default.target
UNIT
}

# install_unit NAME TEXT writes the unit when it changed; it says whether
# it did through the exit status of the comparison.
install_unit() {
  local file="$UNIT_DIR/$1"
  if [ -f "$file" ] && [ "$(cat "$file")" = "$2" ]; then
    return 0
  fi
  printf '%s\n' "$2" > "$file"
  echo "    wrote $file"
}

# The unit needs a user manager (systemctl --user) and, to run at boot and
# outlive this SSH session, lingering. Without lingering the user manager
# stops when the last session ends and takes its units along, so the unit
# is only written, not enabled, and agc starts with nohup as before.
units=0
if systemctl --user show-environment > /dev/null 2>&1; then
  units=1
fi
linger=0
if [ "$(loginctl show-user "$(id -un)" -p Linger --value 2>/dev/null)" = "yes" ]; then
  linger=1
fi

mkdir -p "$RUN"
cd "$RUN"

if [ "$units" -eq 1 ]; then
  echo "==> user unit"
  mkdir -p "$UNIT_DIR"
  install_unit agc-rest.service "$(agc_unit "$(command -v agc)")"
  systemctl --user daemon-reload
fi

# Never pkill -f: it matches and kills this SSH session's shell.
echo "==> restarting"
if [ "$units" -eq 1 ] && [ "$linger" -eq 1 ]; then
  systemctl --user enable agc-rest.service 2> /dev/null
  # An agc an earlier deploy started with nohup is not the unit's.
  if ! systemctl --user is-active --quiet agc-rest.service; then
    pkill -x agc || true
  fi
  systemctl --user restart agc-rest.service
else
  pkill -x agc || true
  (AGC_KNOWLEDGE_DIR="$KNOWLEDGE" AGC_DEFAULT_OWNER="$OWNER" nohup agc rest > agc-rest.log 2>&1 < /dev/null &)
fi

# What is left for a human: lingering needs sudo, which agents never run.
linger_note() {
  if [ "$units" -eq 0 ]; then
    echo "==> systemctl --user does not work here: agc rest runs with nohup and nothing restarts it after a reboot."
  elif [ "$linger" -eq 0 ]; then
    echo "==> lingering is off: agc-rest.service is written but not enabled, and agc rest runs with nohup."
    echo "    Run once, in a terminal on the rig, then deploy again: sudo loginctl enable-linger $(id -un)"
  fi
}

for _ in $(seq 20); do
  if curl -fsS -o /dev/null "127.0.0.1:$PORT/api/tasks" 2>/dev/null; then
    echo "==> ok $(git -C "$SRC" rev-parse --short HEAD) on $HOST"
    agc --version
    linger_note
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
