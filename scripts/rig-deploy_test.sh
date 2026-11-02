#!/usr/bin/env bash
# Runs the remote half of rig-deploy.sh (its --dry-run output) on this
# machine against stub commands, and checks the user unit it writes and
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

# run CASE SYSTEMD LINGER: one deploy in a fresh home. SYSTEMD 1 makes
# systemctl --user work, LINGER is what loginctl says. msb sits outside the
# unit's default PATH, as it may on the rig.
run() {
  local case_dir="$1"
  mkdir -p "$case_dir/home/.local/bin" "$case_dir/opt/msb/bin" "$case_dir/src" "$case_dir/run"
  local bin="$case_dir/home/.local/bin"
  local msbbin="$case_dir/opt/msb/bin"
  local log="$case_dir/calls"
  : > "$log"

  for name in git make curl pkill agc; do
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
  printf '#!/usr/bin/env bash\nexit 0\n' > "$msbbin/msb"
  chmod +x "$bin"/* "$msbbin/msb"

  local script
  script="$(cd "$here/.." && AGC_RIG_SRC="$case_dir/src" AGC_RIG_RUN="$case_dir/run" AGC_RIG_KNOWLEDGE="$case_dir/src" "$here/rig-deploy.sh" --force --dry-run)"
  HOME="$case_dir/home" XDG_CONFIG_HOME="" PATH="$bin:$msbbin:/usr/bin:/bin" bash -c "$script" > "$case_dir/out" 2>&1
  # nohup runs in the background; give the stub a moment to log.
  for _ in 1 2 3 4 5; do
    grep -q "^agc rest" "$log" && break
    grep -q "restart agc-rest" "$log" && break
    sleep 0.2
  done
}

tmp="$(mktemp -d)"
trap '[ -n "${KEEP:-}" ] || rm -r "$tmp"' EXIT

# Lingering on: the agc unit written, enabled and restarted by systemd.
c="$tmp/linger"
run "$c" 1 yes
units="$c/home/.config/systemd/user"
check "agc unit written" test -f "$units/agc-rest.service"
check "no msb unit: msb has no server" test ! -e "$units/agc-msb.service"
check "agc unit runs agc rest" grep -qx "ExecStart=$c/home/.local/bin/agc rest" "$units/agc-rest.service"
check "agc unit orders after nothing" bash -c "! grep -qE '^(After|Wants)=' '$units/agc-rest.service'"
check "agc unit restarts on failure" grep -qx "Restart=on-failure" "$units/agc-rest.service"
check "agc unit keeps its msb machines on restart" grep -qx "KillMode=process" "$units/agc-rest.service"
check "agc unit's PATH reaches msb" grep -q "^Environment=PATH=$c/opt/msb/bin:" "$units/agc-rest.service"
envs="$(grep '^Environment=' "$units/agc-rest.service" | cut -d= -f2 | tr '\n' ' ')"
check "agc unit sets only PATH and the deploy's two settings" test "$envs" = "PATH AGC_KNOWLEDGE_DIR AGC_DEFAULT_OWNER "
check "agc unit keeps the deploy's values" grep -qx "Environment=AGC_DEFAULT_OWNER=franco" "$units/agc-rest.service"
check "agc unit keeps the knowledge dir" grep -qx "Environment=AGC_KNOWLEDGE_DIR=$c/src" "$units/agc-rest.service"
check "no secret in the unit" bash -c "! grep -qiE 'secret|token|key|password' '$units/agc-rest.service'"
check "agc enabled" grep -q "systemctl --user enable agc-rest.service" "$c/calls"
check "agc restarted by systemd" grep -q "systemctl --user restart agc-rest.service" "$c/calls"
check "systemd touches only agc-rest" bash -c "! grep -q 'agc-msb' '$c/calls'"
check "no nohup agc" bash -c "! grep -q '^agc rest' '$c/calls'"
check "no linger note" bash -c "! grep -q 'enable-linger' '$c/out'"

# The same deploy again rewrites nothing.
run "$c" 1 yes
check "second deploy writes no unit" bash -c "! grep -q 'wrote' '$c/out'"

# Lingering off: the unit written but not enabled, nohup, and the sudo line.
c="$tmp/nolinger"
run "$c" 1 no
check "unit written without lingering" test -f "$c/home/.config/systemd/user/agc-rest.service"
check "nothing enabled without lingering" bash -c "! grep -qE 'enable|restart|start agc' '$c/calls'"
check "agc started with nohup" grep -q "^agc rest" "$c/calls"
check "linger command printed" grep -q "sudo loginctl enable-linger $(id -un)" "$c/out"

# No user manager: nohup as before, no units.
c="$tmp/nosystemd"
run "$c" 0 no
check "no unit without systemd" test ! -e "$c/home/.config/systemd/user/agc-rest.service"
check "agc started with nohup without systemd" grep -q "^agc rest" "$c/calls"
check "says nothing restarts it" grep -q "nothing restarts it after a reboot" "$c/out"

exit "$fail"
