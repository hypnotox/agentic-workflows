---
status: Proposed
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [tooling, mutation-testing, advisory]
related: [66]
domains: [tooling]
---
# ADR-0071: Missing mutation-report path is a caller error

## Context

ADR-0066 Decision item 3 made `cmd/mutants` treat "a missing or empty (zero-byte) output
file as an empty run", on the stated premise that gremlins writes no `-o` file when there is
nothing to report. The premise does not match the shipped wrapper: `./x mutants` pre-creates
the report via `mktemp` before gremlins runs, and `set -euo pipefail` aborts on a non-zero
gremlins exit — so through the only supported call path the file always exists, possibly
empty. The 2026-07-07 deep-dive audit flagged the residue: with missing-tolerance in place, a
typo'd path (`go run ./cmd/mutants /tmp/nope.json`) or a future `./x` edit that stops
pre-creating the file prints "no survived mutants" and exits 0 — a silent false-clean in the
one tool whose whole job is distrusting green runs. The behaviour change landed in commit
867e489 with a regression test; this ADR records the decision the code now embodies, since
ADR-0066 is Implemented and append-only.

## Decision

1. **A nonexistent report path is an error.** `cmd/mutants` exits non-zero with the read
   error on stderr when its argument does not exist (or cannot be read). This narrows
   ADR-0066 Decision item 3's tolerance clause: only a **present-but-empty** (zero-byte or
   whitespace-only) report remains an empty run ("no survived mutants", exit 0) — that is the
   state gremlins leaves the pre-created `mktemp` file in when there is nothing to report.
   Partial-item supersedence: ADR-0066 stays Implemented; every other clause of its Decision
   item 3 (timeout distrust, `LIVED`-only reporting, status filtering) is unchanged.

## Invariants

- `inv: mutants-missing-report-errors` — `cmd/mutants` exits non-zero for a nonexistent
  report path and never prints "no survived mutants" for one; a present-but-empty file still
  reports no survivors with exit 0.
- ADR-0066's `inv: mutants-timeout-untrusted` remains in force with its missing-file phrase
  narrowed by this ADR: its backing test asserts the timeout and `LIVED`-reporting behaviour;
  the missing-file half of its original description is superseded here. (textual)

## Consequences

- A typo'd report path or a `./x` regression that stops pre-creating the file now fails
  loudly instead of masquerading as a clean run — consistent with the tool's own
  trust-nothing stance on timeouts.
- Any caller that relied on missing-file tolerance must pre-create the file, as `./x mutants`
  always has. No such caller exists in this repository.
- ADR-0066's prose retains the outdated clause; readers following its `related:` link land
  here. The living AGENTS.md invariant bullet was updated with the code in commit 867e489.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep missing-file tolerance (ADR-0066 as decided) | Its premise is false under the shipped mktemp wrapper; the tolerance only shields caller errors as false-clean runs. |
| Fully supersede ADR-0066 | Every other clause (deterministic recipe, advisory-only, timeout distrust) stands unchanged; a full rewrite would churn a live decision for one clause. |
| Update only the living docs (no ADR) | This is a reversal of a decided clause in an Implemented, append-only ADR — precedent (da1dac3) covers extensions, not reversals; the record must show where the contract changed. |
