#!/usr/bin/env bash
set -euo pipefail

# Build agc for the local machine and install it where PATH resolves it.
# Matches the dotfile: export PATH="$HOME/.agent_composer/bin:$PATH"
# Follows the same bin/<goos>/agc layout as build.sh.

cd "$(dirname "$0")"

GOOS="$(go env GOOS)"
OUT="bin/${GOOS}/agc"
DEST="${HOME}/.agent_composer/bin"

echo "Building agc for ${GOOS}..."
mkdir -p "bin/${GOOS}" "${DEST}"
go build -o "${OUT}" main.go

# Install atomically: write to a temp file (new inode) then rename.
# Overwriting a signed binary in place invalidates macOS's cached code
# signature and the kernel SIGKILLs it ("killed: 9"), so never cp in place.
tmp="$(mktemp "${DEST}/.agc.tmp.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
cp "${OUT}" "$tmp"

# Re-apply an ad-hoc signature on macOS so the fresh file is valid to run.
if [ "${GOOS}" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
    codesign --force --identifier agc --sign - "$tmp" >/dev/null 2>&1 || true
fi

chmod 0755 "$tmp"
mv -f "$tmp" "${DEST}/agc"
trap - EXIT

echo "Installed $("${OUT}" --version) -> ${DEST}/agc"
