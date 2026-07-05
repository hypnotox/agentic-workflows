## The gate

`./x gate` runs the project's checks — the test suite with a coverage profile, a 100%
**statement**-coverage floor over non-`// coverage-ignore` blocks (ADR-0012), `go vet`,
`golangci-lint`, and a whole-program dead-code check (ADR-0063) — and must be green before
every commit. A red gate blocks the commit: fix the cause or revert.

### Coverage: statement gate vs line reporting

`./x gate` is the **sole hard coverage gate**, and it measures **statement** coverage. CI
also uploads to Codecov, which measures **line** coverage — a different metric — so
Codecov's figure (~94%) does not and cannot equal `go tool cover`'s statement figure
(~96.5%); the gap is line-vs-statement, not a defect (ADR-0065).

CI publishes two Codecov numbers as flags:

- **`raw`** — line coverage over the whole tree: the honest reality, which climbs only as
  real branches get covered.
- **`covered`** — line coverage over the profile with `// coverage-ignore` blocks dropped
  (~100%): exactly the blocks the gate holds accountable. The filtered profile is emitted
  by `covercheck --emit-filtered`, reusing the same ignore logic as the gate, so reporter
  and gate never disagree on what "ignored" means.

Both Codecov statuses are informational — Codecov never blocks a merge; the gate does.
