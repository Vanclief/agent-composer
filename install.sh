#!/usr/bin/env bash
set -euo pipefail

# ASCII banner
cat <<'BANNER'
    ___                    __     ______                                          
   /   | ____ ____  ____  / /_   / ____/___  ____ ___  ____  ____  ________  _____
  / /| |/ __ `/ _ \/ __ \/ __/  / /   / __ \/ __ `__ \/ __ \/ __ \/ ___/ _ \/ ___/
 / ___ / /_/ /  __/ / / / /_   / /___/ /_/ / / / / / / /_/ / /_/ (__  )  __/ /    
/_/  |_\__, /\___/_/ /_/\__/   \____/\____/_/ /_/ /_/ .___/\____/____/\___/_/     
      /____/                                       /_/
BANNER

echo
echo "Installing Agent Composer..."

INSTALLATION_DIR="${HOME}/.agent_composer/bin"
INSTALLED_BIN_DIR="${INSTALLATION_DIR}/agc"
SERVER="https://raw.githubusercontent.com/vanclief/agent-composer/master/bin"

# Step 1: Check for curl
command -v curl >/dev/null 2>&1 || {
    echo "curl not found"
    exit 1
}

# Step 2: Pick binary by OS/arch
case "$(uname -s)" in
Linux) FILENAME="linux/agc" ;;
Darwin)
    case "$(uname -m)" in
    x86_64) FILENAME="darwin/agc-amd64" ;;
    arm64) FILENAME="darwin/agc" ;;
    *)
        echo "Unsupported macOS arch: $(uname -m)"
        exit 1
        ;;
    esac
    ;;
*)
    echo "Unsupported OS: $(uname -s)"
    exit 1
    ;;
esac

# Step 3: Ensure target directory exists
mkdir -p "${INSTALLATION_DIR}"

# Step 4: Download to a temp file in the directory and then do an atomic rename
tmp="$(mktemp "${INSTALLATION_DIR}/.agc.tmp.XXXXXX")"
trap 'rm -f "$tmp"' EXIT
echo "Downloading ${SERVER}/${FILENAME} ..."
curl --fail --location --retry 3 --connect-timeout 10 --max-time 120 \
    -o "${tmp}" "${SERVER}/${FILENAME}"
chmod 0755 "${tmp}"
mv -f "${tmp}" "${INSTALLED_BIN_DIR}"
echo "Installed agc -> ${INSTALLED_BIN_DIR}"

# Step 5: If Codex CLI is not installed, add it for apply patch and other OpenAI commands
if command -v codex >/dev/null 2>&1; then
    echo "codex already installed at $(command -v codex). Skipping npm install."
else
    # Only require Node/npm if we actually need to install codex
    if ! command -v npm >/dev/null 2>&1; then
        echo "npm not found. Install Node.js (>=18) or install 'codex' via your package manager, then re-run." >&2
        exit 1
    fi
    if command -v node >/dev/null 2>&1; then
        node_major="$(node -v | sed 's/^v//' | cut -d. -f1)"
        if [ "${node_major}" -lt 18 ]; then
            echo "Node.js >= 18 required (found $(node -v))" >&2
            exit 1
        fi
    fi
    echo "Installing @openai/codex@latest ..."
    npm install -g "@openai/codex@latest" || {
        echo "npm install failed" >&2
        exit 1
    }
    if ! command -v codex >/dev/null 2>&1; then
        echo "Warning: 'codex' not on PATH after install. You may need to add '$(npm bin -g)' to PATH." >&2
    fi
fi

# Step 6: Detect shell and profile
shell_name="$(basename "${SHELL:-sh}")"
PROFILE=""
case "${shell_name}" in
zsh) PROFILE="${ZDOTDIR:-$HOME}/.zshrc" ;;
bash) PROFILE="${HOME}/.bashrc" ;;
fish) PROFILE="${HOME}/.config/fish/config.fish" ;;
ash | sh) PROFILE="${HOME}/.profile" ;;
*) PROFILE="" ;;
esac

# Step 7: Add INSTALLATION_DIR to PATH
if [ "${shell_name}" = "fish" ]; then
    if command -v fish >/dev/null 2>&1; then
        # Avoid aborting script if fish_add_path is missing
        fish -c "type -q fish_add_path; and fish_add_path -U '${INSTALLATION_DIR}'" || true
    fi
    # Fallback to config.fish only if it doesn't already mention INSTALLATION_DIR
    if [ -n "${PROFILE}" ] && [ -f "${PROFILE}" ]; then
        if ! grep -qF "${INSTALLATION_DIR}" "${PROFILE}" 2>/dev/null; then
            echo "set -U fish_user_paths \$fish_user_paths ${INSTALLATION_DIR}" >>"${PROFILE}"
        fi
    fi
else
    if [ -n "${PROFILE}" ]; then
        if ! grep -qF "${INSTALLATION_DIR}" "${PROFILE}" 2>/dev/null; then
            printf '\n# Agent Composer path\nexport PATH="$PATH:%s"\n' "${INSTALLATION_DIR}" >>"${PROFILE}"
        fi
    else
        echo "Unknown shell (${SHELL:-}). Add ${INSTALLATION_DIR} to PATH manually."
    fi
fi

# Step 8: Create aliases for apply-patch functions
if [ "${shell_name}" = "fish" ]; then
    mkdir -p "${HOME}/.config/fish/functions"
    FUNC_FILE="${HOME}/.config/fish/functions/apply_patch.fish"
    if ! grep -q "agent-composer apply_patch functions" "${FUNC_FILE}" 2>/dev/null; then
        cat >"${FUNC_FILE}" <<'EOF'
# --- agent-composer apply_patch functions ---
function apply_patch
    if test (count $argv) -eq 1
        codex --codex-run-as-apply-patch "$argv[1]"
    else
        set patch (cat)
        codex --codex-run-as-apply-patch "$patch"
    end
end
functions -e apply-patch 2>/dev/null; function apply-patch; apply_patch $argv; end
functions -e applypatch  2>/dev/null; function applypatch;  apply_patch $argv; end
# --- end agent-composer apply_patch functions ---
EOF
    fi
else
    if [ -n "${PROFILE}" ] && ! grep -q "agent-composer apply_patch functions" "${PROFILE}" 2>/dev/null; then
        cat >>"${PROFILE}" <<'EOF'

# --- agent-composer apply_patch functions ---
apply_patch() {
  if [ "$#" -eq 1 ]; then
    codex --codex-run-as-apply-patch "$1"
  else
    p="$(cat)"
    codex --codex-run-as-apply-patch "$p"
  fi
}
apply-patch() { apply_patch "$@"; }
applypatch()  { apply_patch "$@"; }
# --- end agent-composer apply_patch functions ---
EOF
    fi
fi

echo
echo "Successfully installed Agent Composer!"
echo "Open a new terminal (or 'source' your profile) to run agc"
