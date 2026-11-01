#!/bin/sh
# Lists explicit semicolons in tracked Go files. AGENTS.md allows them only
# in a 3-clause for header. This is a heuristic, not a parser: it drops
# one-line strings, rune literals and comments, skips lines inside a raw
# string or /* */ comment that spans lines and lines that start with "for ",
# then reports any ; left over. That catches `if init; cond`, `switch init;`
# and `a(); b()`. Run from the repo root.
set -eu

found=$(
  git ls-files '*.go' |
    grep -v -x \
      -e core/resources/hooks/create.go \
      -e core/resources/hooks/update.go \
      -e core/resources/workflow/compose.go |
    grep -v -e '^web/' -e '/node_modules/' |
    while read -r file; do
      awk -v file="$file" '
        {
          line = $0
          # Inside a raw string or block comment opened on an earlier line.
          if (open != "") {
            end = index(line, open)
            if (end == 0) next
            line = substr(line, end + length(open))
            open = ""
          }
          gsub(/`[^`]*`/, "", line)
          gsub(/"([^"\\]|\\.)*"/, "", line)
          gsub(/\047([^\047\\]|\\.)*\047/, "", line)
          gsub(/\/\*([^*]|\*[^\/])*\*\//, "", line)
          sub(/\/\/.*$/, "", line)
          if (index(line, "`") > 0) {
            open = "`"
            line = substr(line, 1, index(line, "`") - 1)
          } else if (index(line, "/*") > 0) {
            open = "*/"
            line = substr(line, 1, index(line, "/*") - 1)
          }
          if (line ~ /^[ \t]*for[ \t]/) next
          if (index(line, ";") > 0) printf "%s:%d: %s\n", file, NR, $0
        }
      ' "$file"
    done
)

if [ -n "$found" ]; then
  echo "explicit semicolons outside a for header:"
  echo "$found"
  exit 1
fi
