#!/usr/bin/env bash
# Runs the remote half of rig-deploy.sh (its --dry-run output) on this
# machine against stub commands, and checks the user units it writes and
# how it restarts agc: with systemctl when lingering is on, with nohup
# otherwise (ADR 0017). Nothing here touches systemd or the rig.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
fail=0

check() {
  local what="$1"
  shift
  if "$@"; then
    echo "ok   $what"
  else
    echo "FAIL $what" >&2
    fail=1
  fi
}

# run CASE SYSTEMD LINGER PGREP: one deploy in a fresh home. SYSTEMD 1
# makes systemctl --user work, LINGER is what loginctl says, PGREP 0 says
# an msb server already runs.
run() {
  local case_dir="$1"
  mkdir -p "$case_dir/home/.local/bin" "$case_dir/src" "$case_dir/run"
  local bin="$case_dir/home/.local/bin"
  local log="$case_dir/calls"
  : > "$log"

  for name in git make curl pkill agc msb; do
    cat > "$bin/$name" <<STUB
#!/usr/bin/env bash
echo "$name \$*" >> "$log"
case " \$* " in *" rev-parse "*) [ "$name" = git ] && echo abc1234 ;; esac
exit 0
STUB
  done
  cat > "$bin/systemctl" <<STUB
#!/usr/bin/env bash
echo "systemctl \$*" >> "$log"
[ "\$2" = "show-environment" ] && [ "$2" != "1" ] && exit 1
[ "\$2" = "is-active" ] && exit 3
exit 0
STUB
  cat > "$bin/loginctl" <<STUB
#!/usr/bin/env bash
echo "$3"
STUB
  cat > "$bin/pgrep" <<STUB
#!/usr/bin/env bash
exit $4
STUB
  chmod +x "$bin"/*

  local script
  script="$(cd "$here/.." && AGC_RIG_SRC="$case_dir/src" AGC_RIG_RUN="$case_dir/run" AGC_RIG_KNOWLEDGE="$case_dir/src" "$here/rig-deploy.sh" --force --dry-run)"
  HOME="$case_dir/home" XDG_CONFIG_HOME="" PATH="$bin:/usr/bin:/bin" bash -c "$script" > "$case_dir/out" 2>&1
  # nohup runs in the background; give the stub a moment to log.
  for _ in 1 2 3 4 5; do
    grep -q "^agc rest" "$log" && break
    grep -q "restart agc-rest" "$log" && break
    sleep 0.2
  done
}

tmp="$(mktemp -d)"
trap '[ -n "${KEEP:-}" ] || rm -r "$tmp"' EXIT

# Lingering on: units written, enabled, agc restarted by systemd, msb started.
c="$tmp/linger"
run "$c" 1 yes 1
units="$c/home/.config/systemd/user"
check "agc unit written" test -f "$units/agc-rest.service"
check "msb unit written" test -f "$units/agc-msb.service"
check "agc unit runs agc rest" grep -qx "ExecStart=$c/home/.local/bin/agc rest" "$units/agc-rest.service"
check "agc unit starts after msb" grep -qx "After=agc-msb.service" "$units/agc-rest.service"
check "agc unit restarts on failure" grep -qx "Restart=on-failure" "$units/agc-rest.service"
check "agc unit keeps the VMs on restart" grep -qx "KillMode=process" "$units/agc-rest.service"
check "msb unit runs the msb server" grep -qx "ExecStart=$c/home/.local/bin/msb server start" "$units/agc-msb.service"
check "msb unit restarts on failure" grep -qx "Restart=on-failure" "$units/agc-msb.service"
envs="$(grep '^Environment=' "$units/agc-rest.service" | cut -d= -f2 | tr '\n' ' ')"
check "agc unit sets only PATH and the deploy's two settings" test "$envs" = "PATH AGC_KNOWLEDGE_DIR AGC_DEFAULT_OWNER "
check "agc unit keeps the deploy's values" grep -qx "Environment=AGC_DEFAULT_OWNER=franco" "$units/agc-rest.service"
check "no secret in a unit" bash -c "! grep -qiE 'secret|token|key|password' '$units/agc-rest.service' '$units/agc-msb.service'"
check "msb enabled and started" grep -q "systemctl --user start agc-msb.service" "$c/calls"
check "agc enabled" grep -q "systemctl --user enable agc-rest.service" "$c/calls"
check "agc restarted by systemd" grep -q "systemctl --user restart agc-rest.service" "$c/calls"
check "no nohup agc" bash -c "! grep -q '^agc rest' '$c/calls'"
check "no linger note" bash -c "! grep -q 'enable-linger' '$c/out'"

# The same deploy again rewrites nothing.
run "$c" 1 yes 1
check "second deploy writes no unit" bash -c "! grep -q 'wrote' '$c/out'"

# An msb server started by hand keeps running.
c="$tmp/byhand"
run "$c" 1 yes 0
check "msb by hand is not started again" bash -c "! grep -q 'start agc-msb' '$c/calls'"
check "msb never restarted" bash -c "! grep -q 'restart agc-msb' '$c/calls'"

# Lingering off: units written but not enabled, nohup, and the sudo line.
c="$tmp/nolinger"
run "$c" 1 no 1
check "units written without lingering" test -f "$c/home/.config/systemd/user/agc-rest.service"
check "nothing enabled without lingering" bash -c "! grep -qE 'enable|restart|start agc' '$c/calls'"
check "agc started with nohup" grep -q "^agc rest" "$c/calls"
check "linger command printed" grep -q "sudo loginctl enable-linger $(id -un)" "$c/out"

# No user manager: nohup as before, no units.
c="$tmp/nosystemd"
run "$c" 0 no 1
check "no units without systemd" test ! -e "$c/home/.config/systemd/user/agc-rest.service"
check "agc started with nohup without systemd" grep -q "^agc rest" "$c/calls"
check "says nothing restarts it" grep -q "nothing restarts it after a reboot" "$c/out"

exit "$fail"
