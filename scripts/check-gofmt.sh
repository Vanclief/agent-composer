#!/bin/sh
# Lists tracked Go files gofmt would change, except the three known
# unformatted files AGENTS.md says to leave alone. Run from the repo root.
set -eu

found=$(
  git ls-files -z '*.go' |
    xargs -0 gofmt -l |
    grep -v -x \
      -e core/resources/hooks/create.go \
      -e core/resources/hooks/update.go \
      -e core/resources/workflow/compose.go ||
    true
)

if [ -n "$found" ]; then
  echo "gofmt would reformat:"
  echo "$found"
  exit 1
fi
