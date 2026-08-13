`./x gate` must be green before every commit. It independently selects profiled Go tests with coverage and Pi runtime smoke from one NUL-safe, rename-disabled staged-index path snapshot: a nonempty exact documentation-only transaction (`docs/**`, `README.md`, `changelog/CHANGELOG.md`, `.awf/docs/parts/**`, or `templates/docs/**`) skips both; Pi-only paths run Pi only; Go-only paths run Go only; and overlapping or uncertain, empty, unreadable, malformed, or unrecognized snapshots run both (ADR-0276). Each skipped suite prints a notice. Commands still test the working tree. The gate always runs `go vet`, released-platform cross-compilation, `golangci-lint`, whole-program dead-code checking (ADR-0063), and `cmd/pincheck` (ADR-0079). It writes `coverage.out` when it runs profiled Go tests, enforces 100% **statement** coverage outside `// coverage-ignore` blocks (ADR-0012), and its Pi lane runs Pi-extension strict type checks and 100% statement/branch/function/line coverage and descriptor parity.

| Command | Use |
|---|---|
| `./x gate` | Complete deterministic transaction. |
| `./x gate timings` | Same selected transaction with elapsed time only for executed stages. |
| `./x test` | Go suite without the Docker-backed Pi smoke. |
| `./x pi-test run` | Pi lane alone. |

The gate enables `TestPiRealRuntimeSmoke` once with test caching disabled; `./x test` and verbose direct Go tests explain its omission. Plain-punctuation (`awf check repo prose`) and effort-memory (`awf check repo memory`) scans are hook and CI checks, not gate steps. Commit-provenance tests use native Git, SSH-signed commits, disposable refs and remotes, both `core.hooksPath` forms, and linked worktrees. A red gate blocks the commit: fix the cause or revert.

### Coverage

`./x gate` is the sole hard coverage gate and measures statement coverage. Codecov reports line coverage, so its figure cannot equal `go tool cover`'s statement figure (ADR-0065).

| Codecov flag | Meaning |
|---|---|
| `raw` | Whole-tree line coverage. |
| `covered` | Line coverage after `covercheck --emit-filtered` removes `// coverage-ignore` blocks. |

Codecov is informational; the gate blocks merges.

### Mutation testing

Coverage proves execution, not useful assertions. Change a condition, comparison, or constant and confirm a test fails; otherwise add the missing assertion.

`./x mutants` (ADR-0066) runs deterministic `gremlins` mutation testing against the production diff from `main`; pass a package, such as `./x mutants ./internal/refs`, for a focused run. It is advisory, never a gate step. A timed-out mutant exits nonzero because it can hide survivors; raise `.gremlins.yaml`'s timeout coefficient and rerun. Triage every survivor as a missing assertion or an equivalent mutant.

The strict container lane also covers standalone context usage, including formatting, unavailable model-window output, active-branch compaction, per-call refresh, and silent supported operation.
