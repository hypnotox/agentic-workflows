# 2026-07-07: Decimal-degrees CLI

**Goal:** implement [ADR-0002](../decisions/0002-cli-accepts-coordinates-as-decimal-degrees-only.md): positional decimal-degree parsing in `cmd/sundial`.

**Architecture summary:** see the ADR. One file changes; the model is untouched.

**Tech stack:** Go 1.26, stdlib only.

**File structure:** modified `cmd/sundial/main.go`.

## Phase 1: parse and gate

- [x] Parse `os.Args[1]`/`os.Args[2]` with `strconv.ParseFloat`; on error print
      `sundial: latitude and longitude must be decimal degrees` to stderr, exit 2.
- [x] Print the usage line and exit 2 when the argument count is not 2.
- [x] `./x gate` green; commit: `feat(cli): parse coordinates as decimal degrees`.

## Phase 2: record

- [x] Flip ADR-0002 to Implemented; `./x sync` regenerates `docs/decisions/ACTIVE.md`.
- [x] Commit: `docs(docs): flip ADR-0002 to Implemented`.
