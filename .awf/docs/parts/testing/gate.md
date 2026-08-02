## The gate

`./x gate` runs the project's checks and must be green before every commit: the test suite
with a coverage profile (written durably to `coverage.out` for CI's Codecov upload, ADR-0196),
a 100% **statement**-coverage floor over non-`// coverage-ignore`
blocks (ADR-0012), containerized Pi-extension strict type checks and 100% line/function/branch
coverage across all seven generated Pi TypeScript files, including the selection-gated effort client and association state machine, descriptor cross-runtime parity, `go vet`,
a cross-compile of `./...` for every released non-host platform,
`golangci-lint`, a whole-program dead-code check (ADR-0063), and
the workflow supply-chain pin check (`cmd/pincheck`, ADR-0079). The plain-punctuation scan
(`awf check repo prose`, ADR-0119, opt-in for adopters and enabled in this repo) and the effort-owned-memory
citation scan (`awf check repo memory`, ADR-0158 updated by ADR-0175, opt-in for adopters and enabled in this repo)
are not gate steps: the pre-commit hook payload runs them locally and CI backstops them (ADR-0196). A red gate blocks the commit: fix the cause or revert.

The example adopter is required to produce no notes. Repository regression tests keep bare `awf context internal/project cmd/awf` and bare `awf context cmd/awf/context.go` within direct delivery, while explicitly requested detail remains complete and spill-capable. Catalog and render tests require native skill discovery, advisory workflow relationships, pruning of disabled outputs, and non-Pi parity. Container tests cover handoff runtime guards; exact effort-owned path, slug, ownership, hard-link, bounded UTF-8, stable identity, and repository confinement; cancellation; editor fallback; and Pi-only explicit effort association, same-conversation replacement transfer, owner-checked activity recovery, advisory heartbeat, and Remote Pi metadata/name degradation. Non-Pi output-plan tests prove those artifacts remain absent. Go suites cover schema-2 resident ordering and crash states, one-winner slug reservation, finish tombstones, real-Git add/integrate/remove topology, protocol output, and generated lifecycle coverage. The TypeScript floor is 100% in statements, branches, functions, and lines; reachable paths are tested through injected dependencies, and exclusions remain only for reasoned unreachable runtime guards.
The example adopter is required to produce no notes. Repository regression tests keep bare `awf context internal/project cmd/awf` and bare `awf context cmd/awf/context.go` within direct delivery, while explicitly requested detail remains complete and spill-capable. Catalog and render tests require native skill discovery, advisory workflow relationships, pruning of disabled outputs, and non-Pi parity. Container tests cover handoff runtime guards, the bounded kickoff schema, UTF-16 length, exact prose propagation, queue and pending-request behavior, the best-effort continuation disposition after successful queueing, the five-second countdown, cancellation, persisted-session revalidation, lineage, cleanup, recovery notices, no-silent-retry behavior, and editor fallback. Go suites cover schema-2 resident ordering and crash states, one-winner slug reservation, finish tombstones, real-Git add/integrate/remove topology, protocol output, and generated lifecycle coverage. The TypeScript floor is 100% in statements, branches, functions, and lines; reachable paths are tested through injected dependencies, and exclusions remain only for reasoned unreachable runtime guards.

### Coverage: statement gate vs line reporting

`./x gate` is the **sole hard coverage gate**, and it measures **statement** coverage. CI
also uploads to Codecov, which measures **line** coverage (a different metric) so
Codecov's raw figure does not and cannot equal `go tool cover`'s statement figure;
the gap is line-vs-statement, not a defect (ADR-0065).

CI publishes two Codecov numbers as flags:

- **`raw`**: line coverage over the whole tree: the honest reality, which climbs only as
  real branches get covered.
- **`covered`**: line coverage over the profile with `// coverage-ignore` blocks dropped
  (~100%): exactly the blocks the gate holds accountable. The filtered profile is emitted
  by `covercheck --emit-filtered`, reusing the same ignore logic as the gate, so reporter
  and gate never disagree on what "ignored" means.

Both Codecov statuses are informational: Codecov never blocks a merge; the gate does.

### Coverage is not verification

The 100% floor proves every statement **runs** under test; it does not prove any test
would **fail** if that statement were wrong. A line can be covered by a test that never
asserts on its effect: the gate stays green while a broken result slips through. When you
add or change logic, spot-check it by hand: flip a condition, negate a comparison, or
change a constant in the source, and confirm a test turns red. If nothing fails, the gap is
a missing assertion, not missing coverage; add the assertion, then revert the edit. This
is a deliberate manual habit. `./x mutants` (ADR-0066) makes it reproducible: it runs
`gremlins` mutation testing under a deterministic config (`.gremlins.yaml`:
`integration: true`, `workers: 1`, `timeout-coefficient: 20`) and prints the survived
mutants for you to triage; run it
with no arguments to check your diff against `main`, or pass a package path (e.g. `./x
mutants ./internal/refs`) for a deep dive. A timed-out mutant makes the whole run
untrustworthy (it can hide a real survivor), so the command itself exits non-zero when any
mutant times out. Raise the timeout coefficient and rerun; you never need to eyeball the
`Timed out:` count. It stays advisory (never part of the gate) and every survivor still
needs you to judge whether it is a real gap or an unkillable equivalent mutant.

The strict container lane includes the standalone context-usage output and covers its local formatting, unavailable model-window form, active-branch compactions, per-call refresh, and silent supported operation.
