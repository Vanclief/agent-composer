#!/usr/bin/env bash
set -e

echo "Installing agent composer..."

TARGET_DIR="$HOME/.agent_composer/bin"
TARGET_DIR_BIN="$TARGET_DIR/agc"

SERVER="https://raw.githubusercontent.com/vanclief/agent-composer/master/bin"

# Detect the platform (similar to $OSTYPE)
OS="$(uname)"
if [[ "$OS" == "Linux" ]]; then
    # Linux
    FILENAME="linux/agc"
elif [[ "$OS" == "Darwin" ]]; then
    # MacOS, should validate if Intel or ARM
    UNAMEM="$(uname -m)"
    if [[ "$UNAMEM" == "x86_64" ]]; then
        FILENAME="darwin/agc-amd64"
    else
        FILENAME="darwin/agc"
    fi
else
    echo "unrecognized OS: $OS"
    echo "Exiting..."
    exit 1
fi

# Check if ~/.agent_composer/bin exists, if not create it
if [[ ! -e "${TARGET_DIR}" ]]; then
    mkdir -p "${TARGET_DIR}"
fi

# Download the appropriate binary
echo "Downloading $SERVER/$FILENAME..."
rm -f "${TARGET_DIR_BIN}"
curl -# -L "${SERVER}/${FILENAME}" -o "${TARGET_DIR_BIN}"
chmod +x "${TARGET_DIR_BIN}"
echo "Installed under ${TARGET_DIR_BIN}"

# Store the correct profile file (i.e. .profile for bash or .zshenv for ZSH).
case $SHELL in
*/zsh)
    PROFILE=${ZDOTDIR-"$HOME"}/.zshenv
    PREF_SHELL=zsh
    APPEND_COMMAND="export PATH=\"\$PATH:$TARGET_DIR\""
    ;;
*/bash)
    PROFILE=$HOME/.bashrc
    PREF_SHELL=bash
    APPEND_COMMAND="export PATH=\"\$PATH:$TARGET_DIR\""
    ;;
*/fish)
    PROFILE=$HOME/.config/fish/config.fish
    PREF_SHELL=fish
    APPEND_COMMAND="set -U fish_user_paths \$fish_user_paths $TARGET_DIR"
    ;;
*/ash)
    PROFILE=$HOME/.profile
    PREF_SHELL=ash
    APPEND_COMMAND="export PATH=\"\$PATH:$TARGET_DIR\""
    ;;
*)
    echo "could not detect shell, manually add ${TARGET_DIR} to your PATH."
    exit 1
    ;;
esac

# Only add if it isn't already in PATH.
if [[ ":$PATH:" != *":${TARGET_DIR}:"* ]]; then
    echo >>"$PROFILE"
    echo "$APPEND_COMMAND" >>"$PROFILE"
fi

# Reload the profile
if [[ "$PREF_SHELL" == "fish" ]]; then
    # For fish, we need to reload the fish_user_paths
    fish -c "source $PROFILE"
else
    # For other shells, source the profile
    source "$PROFILE"
fi

echo
echo "Detected your preferred shell is ${PREF_SHELL} and added Agent Composer to PATH. You can now run the command agc."
echo "agc (Agent Composer) successfully installed!"
