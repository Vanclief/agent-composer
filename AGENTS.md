# AGENTS.md

## Go style rules (strict)

### No explicit semicolons outside `for`

- Never write an explicit semicolon `;` in Go code.
- Exception (only): semicolons are allowed _only_ inside a 3-clause `for` header:
  `for init; condition; post { ... }`
- All other explicit-semicolon forms are forbidden, including:
  - multiple statements on one line: `a(); b()`
  - `if init; condition { ... }` (e.g. `if err := f(); err != nil { ... }`)
  - `switch init; { ... }` and type-switch init forms

### Required rewrite style

- One statement per line.
- If you need an initializer for `if`/`switch`, move it to its own line above the statement.

### Self-check before final output

- Before returning code, scan what you wrote:
  if it contains `;` outside `for init; condition; post {}`, rewrite until compliant.

## Review guidelines

- Flag any explicit semicolon `;` usage outside `for init; condition; post { ... }` as an issue.
- Flag any `if init; cond {}` / `switch init; {}` usage as an issue (requires rewrite to multi-line).
