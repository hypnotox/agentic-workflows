---
status: Implemented
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [tooling, testing]
related: [2, 8]
domains: [tooling]
---
# ADR-0012: Full Coverage Gate and the `// coverage-ignore` Convention

## Context

`awf` is a tooling library: small, pure-Go, no network, no platform branches, no
generated code. Its own guide (`AGENTS.md`) makes coverage a standing
responsibility — "coverage gaps are yours" — and states that awf "is both the tool
that publishes the standard and the first adopter of it, so its own setup must model
what it generates." Yet nothing enforces coverage. `./x gate` runs
`go test ./... && go vet ./... && go tool golangci-lint run` (ADR-0002 Decision item 4);
none of those measure statement coverage, so it can regress silently.

The measured baseline is **80.3%** of statements (`go test ./... -cover`). The gap is
concentrated, not pervasive:

- `cmd/awf` is the worst at 47.4%. Six functions are 0%: `main`, `fatal`, `fatalIf`
  (all `os.Exit`-bound, so uncoverable without a refactor), and the handlers
  `runInvariants`, `runList`, `runUpgrade` (simply untested).
- The internal packages sit at 71–94%; the residue is overwhelmingly unhit **error
  branches** (malformed YAML, missing files, IO failure) plus a few entirely-untested
  functions (`project.CheckInvariants`, `invariants.Detail`).
- `templates` is pure `embed.FS` with zero coverable statements; it never appears in a
  coverprofile and contributes nothing to the aggregate.

Three couplings shaped the decision:

1. **The `os.Exit`-bound entrypoint is structurally uncoverable.** `cmd/awf/main.go`
   dispatches `os.Args[1]` and routes every error through `fatal`/`fatalIf`, which call
   `os.Exit`. A test cannot exercise that wiring without either re-executing the binary
   as a subprocess or a refactor that returns an exit code instead of calling `os.Exit`.

2. **Literal 100% collides with defensive branches.** A handful of error paths are
   practically unreachable in a test — `os.Getwd()` failing, marshalling a known-good
   struct failing. Go has no built-in coverage-ignore. Reaching a true 100% therefore
   needs either contrived "mock-the-world" seams or a sanctioned ignore mechanism.

3. **Invariant backing is Go-only.** `.claude/awf/config.yaml` sets `invariants.sources`
   to the `*.go` glob with marker `//` (ADR-0008). A coverage invariant can only be
   machine-backed if the thing that enforces it is Go code carrying a `// invariant:`
   comment — a bash/awk filter inline in `./x` could not be backed.

Scope is this repo's gate only. Promoting a coverage gate into the rendered awf
*standard* (a shipped template + catalog entry) is a separate, larger change and is not
decided here.

## Decision

1. **Add a coverage step to the gate.** `./x gate` runs the suite with
   `-coverprofile`, then runs a coverage checker that computes the total over
   non-ignored statements and **exits non-zero if that total is below 100%**. This
   extends ADR-0002 Decision item 4; the gate becomes
   `go test ./... -coverprofile=… && <checker> && go vet ./... && go tool golangci-lint run`
   (ordering is an implementation detail; the checker must run after the profiled test).

2. **Implement the checker as a Go program in the module**, not a bash/awk script —
   a `cmd/covercheck` entrypoint over an `internal/coverage` package. Rationale: it is
   then itself measured by `go test ./...` (and must meet the same 100% bar — awf
   dogfoods its own rule), it is unit-testable against fixture profiles, and its
   enforcement logic can carry the `// invariant:` backing comment the `*.go` glob
   requires. The package parses Go's coverprofile format
   (`file:startLine.col,endLine.col numStmt count`) and, because the profile carries no
   comment text, reads each profiled source file to locate ignore markers; it then drops
   blocks tagged for ignore (see item 3) and reports `Σcovered / Σstatements` over the
   survivors.

3. **Establish the `// coverage-ignore: <reason>` convention** as the last-resort escape
   hatch for genuinely-unreachable defensive branches. A block whose start line (the
   `startLine` the profile reports for that block) — or the source line immediately above
   that start line — carries the marker is dropped from **both** numerator
   and denominator — an ignored line neither inflates nor deflates the total. The reason
   is **mandatory**: a bare `// coverage-ignore` with no non-empty reason fails the
   checker. Use is audited as a repo-local review practice during implementation review
   (the `awf-reviewing-impl` pass), not as a new obligation added to the rendered standard
   skill; the marker is greppable so every use is auditable. It is reserved for branches no
   fixture can reach (e.g. marshalling a static struct), never a substitute for a missing
   test.

4. **Refactor the `cmd/awf` entrypoint into a testable seam.** Extract dispatch into
   `run(args []string, stdout, stderr io.Writer) int` returning an exit code; `main`
   becomes `os.Exit(run(os.Args, os.Stdout, os.Stderr))`. Delete `fatal` and `fatalIf` —
   error→exit-code mapping moves inside `run`. Handlers thread the injected writer for all
   user-facing output, **including `runSetup`'s `exec.Command` stdout/stderr** (currently
   hardcoded to `os.Stdout`/`os.Stderr`), so output is assertable in tests. A package-level
   `var getwd = os.Getwd` seam makes the cwd-failure path in `run` testable.

5. **Coverage is measured across the whole module.** `go test ./...` with a single
   merged profile; zero-statement packages (`templates`) do not contribute. Tests favour
   real fixtures (`t.TempDir`, malformed inputs, missing files) over permission tricks and
   mock-the-world seams. The concrete test tactics for otherwise-hard-to-reach branches
   (e.g. when `chmod`-based injection is acceptable and how it is guarded under `root`) are
   detailed in the implementation plan, not fixed here.

## Invariants

- `inv: coverage-gate-100` — the coverage checker exits non-zero when the total over
  non-ignored statements in a coverprofile is below 100%, and zero when it is exactly
  100%. Backed by a checker test over fixture profiles.
- `inv: coverage-ignore-reason` — a `// coverage-ignore` marker with no non-empty reason
  causes the checker to fail rather than silently dropping the block. Backed by a checker
  test over a fixture carrying a reasonless marker.
- `inv: single-os-exit` — within `cmd/awf`, `os.Exit` appears only in `main.go`'s `main`,
  whose body is solely `os.Exit(run(...))`; no `fatal`/`fatalIf` helpers remain. Backed by
  a tree-walking test over the `cmd/awf` package sources.
- Every `// coverage-ignore` marker in the tree carries a justification reviewed under
  `awf-reviewing-impl`; the marker is never used in place of a writable test. (Textual
  contract.)

## Consequences

Easier:
- Coverage can no longer regress silently: a new uncovered line fails `./x gate` (and the
  pre-commit hook) until it is tested or explicitly, justifiably ignored.
- awf models the rule it preaches — its own suite is the reference adopter of a 100% gate.
- The `run()` seam makes the entire CLI dispatch and error-mapping unit-testable, not just
  the individual handlers.

Harder / accepted trade-offs:
- Every future change carries a test obligation: new statements must be covered or
  justifiably ignored. This is the intended cost.
- The gate gains a profiled test run and a checker pass — marginally slower than the
  bare suite.
- `cmd/covercheck` + `internal/coverage` are new code to maintain, and are themselves held
  to 100%.
- The `// coverage-ignore` marker is a discretionary escape hatch. It is mitigated by the
  mandatory-reason check (machine-enforced) and the impl-review audit (human-enforced), but
  it remains a lever that must be policed rather than trusted.

Ruled out (for now):
- A coverage gate in the rendered awf *standard* (deferred to a future ADR).
- A sub-100 numeric floor; per-package thresholds.

Downstream work unblocked: an implementation plan covering — build `internal/coverage` +
`cmd/covercheck` with its own tests, wire it into `./x gate`, refactor the `cmd/awf`
entrypoint into the `run()` seam, then drive every package to 100% via fixture tests. When
this ADR flips to Implemented, the same commit backs the tagged invariant slugs,
records the 100% gate and the `// coverage-ignore: <reason>` escape hatch in `AGENTS.md`
(the repo-local agent guide — not the rendered standard, which is out of scope here) so
contributors know the convention exists and how to use it, and regenerates
`docs/decisions/ACTIVE.md` via `./x sync`. No `docs/decisions/README.md` index row is
owed (this repo's README is a how-to guide; `ACTIVE.md` is the generated index).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Bash/awk coverage filter inline in `./x` | Not itself covered, not unit-testable, and cannot carry the `// invariant:` backing the `*.go` glob requires. A Go checker dogfoods the rule and is testable. |
| Aggregate floor below 100% (e.g. 95%) | Lets a well-covered package mask a regressed one and tolerates silent erosion; the user chose a true 100% target. |
| Per-package 100% instead of aggregate | At a 100% target the two are identical — the aggregate hits 100% only if every package does — so per-package adds script for no extra strictness. |
| Subprocess exec tests for `main`/`fatal` | Re-executing the test binary to assert exit codes is slower and clunkier than a `run()` seam that returns an `int`; the seam also makes dispatch directly unit-testable. |
| True 100% via injectable seams everywhere, no ignore marker | Forces contrived mock-the-world seams into production code for the few genuinely-unreachable lines; the hybrid (fixtures first, one `getwd` seam, marker for the residue) keeps tests real. |
| No ignore mechanism, exclude the entrypoint from the target | Leaves a permanent coverage hole and an honesty gap in the "100%" claim; the `run()` seam closes the entrypoint instead. |
