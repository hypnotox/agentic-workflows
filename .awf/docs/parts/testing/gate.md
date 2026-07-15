## The gate

`./x gate` runs the project's checks and must be green before every commit: the test suite
with a coverage profile, a 100% **statement**-coverage floor over non-`// coverage-ignore`
blocks (ADR-0012), `go vet`, `golangci-lint`, a whole-program dead-code check (ADR-0063),
and the workflow supply-chain pin check (`cmd/pincheck`, ADR-0079). A red gate blocks the commit: fix the cause or revert.

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
