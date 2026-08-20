`./x gate` must be green before every commit. It independently selects profiled Go tests with coverage and Pi runtime smoke from one NUL-safe, rename-disabled staged-index path snapshot: a nonempty exact documentation-only transaction (`docs/**`, `README.md`, `changelog/CHANGELOG.md`, `.awf/docs/parts/**`, or `templates/docs/**`) skips both; Pi-only paths run Pi only; Go-only paths run Go only; and overlapping or uncertain, empty, unreadable, malformed, or unrecognized snapshots run both (ADR-0276). Each skipped suite prints a notice. Commands still test the working tree. The gate always runs `go vet`, released-platform cross-compilation, blocking defect lint, advisory style and heuristic lint, whole-program dead-code checking (ADR-0063), and `cmd/pincheck` (ADR-0079). Advisory lint findings print an explicit warning and succeed; either linter's execution or configuration failure blocks. The gate writes `coverage.out` when it runs profiled Go tests, enforces 100% **statement** coverage outside `// coverage-ignore` blocks (ADR-0012), and its Pi lane runs Pi-extension strict type checks and 100% statement/branch/function/line coverage and descriptor parity.

| Gate class | Protected property and exit behavior |
|---|---|
| Error | Version/schema compatibility, tests, coverage, vet, builds, defect lint, production reachability, and workflow pins protect correctness, safety, authority, or reproducibility and exit nonzero. |
| Warning | Style, wording, formatting, preferred idiom, speculative performance, possible cohesion, and heuristic maintainability lint remains visible and exits zero. |
| Information | Skipped-lane and successful operation notes exit zero. |

### Check severity

`awf check` uses the same three visible categories. Error and Warning are the two fixed finding ranks; Information is an unranked note. A complete report exits nonzero exactly when it contains an Error.

| Category | Current checks and protected property |
|---|---|
| Error | Invalid config, locks, sidecars, ADRs, plans, topics, frontmatter, and declared references protect correctness and authority. Generated and staged drift, tracking membership, residue, binary/schema compatibility, current-state transitions, memory citations, commit policy, and unavailable required verification protect reproducibility, safety, or authority. |
| Warning | Prose punctuation, glossary length, tag health, plan assignment/detail, current-state fan-out, and guide size are style, readability, cohesion, or review heuristics. They remain visible and exit zero. |
| Information | Unused or unset render vocabulary, stub content, marker suggestions, tracking or staged-universe availability, non-blocking compatibility, context suggestions, and successful operation notes are optional guidance and exit zero. |

Direct checks retain the same classification as their aggregate. Operational inability to load or scan a declared universe remains an Error even when that universe's produced findings are Warning or Information.

| Command | Use |
|---|---|
| `./x gate` | Complete deterministic transaction. |
| `./x gate timings` | Same selected transaction with elapsed time only for executed stages. |
| `./x test` | Go suite without the host Pi smoke. |
| `./x pi-test run` | Pi lane alone. |

The gate enables `TestPiRealRuntimeSmoke` once with test caching disabled; `./x test` and verbose direct Go tests explain its omission. The deterministic Pi lane uses a protocol-v2 contract double to prove generated adapter negotiation, native awf skill discovery and routing delivery, and retained effort integration. It pins source-only `pi-tools/testing` v0.3.0 for generic recorder seams, while it does not install, pin, or behavior-test an external adopter `pi-tools` runtime. Plain-punctuation (`awf check repo prose`) and effort-memory (`awf check repo memory`) scans are hook and CI checks, not gate steps. Commit-provenance tests use native Git, SSH-signed commits, disposable refs and remotes, both `core.hooksPath` forms, and linked worktrees. A red gate blocks the commit: fix the cause or revert.

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

The strict host lane covers awf's rendered profile contract, routing and Git policy, handshake outcomes, generated-output boundary, and retained effort behavior. General context usage, handoff, scheduling, subprocess supervision, and progress rendering are assured by `pi-tools`, not duplicated in awf tests.
