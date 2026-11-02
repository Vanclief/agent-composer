#!/bin/sh
# Lists tracked files that spell a microsandbox secret placeholder
# literally ($MSB_<SECRET NAME>). The placeholders are deterministic, so
# any file that spells one poisons the context of every sandbox agent that
# reads it: the proxy refuses the model call that carries it and the
# session is dead (2026-11-02, peer-servers, twice). MSB_HOME and MSB_PATH
# are microsandbox's own settings, not placeholders. Run from the repo root.
set -eu

found=$(
  git ls-files -z |
    xargs -0 grep -n -E '(^|[^A-Za-z0-9_])\$?MSB_[A-Z][A-Z0-9_]*' -- 2>/dev/null |
    grep -v -E 'MSB_(HOME|PATH)\b' |
    grep -v -E '^scripts/check-placeholders.sh:' ||
    true
)

if [ -n "$found" ]; then
  echo "secret placeholders spelled literally (write 'the placeholder' instead):"
  echo "$found"
  exit 1
fi
