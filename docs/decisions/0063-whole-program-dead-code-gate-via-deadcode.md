---
status: Implemented
date: 2026-07-05
supersedes: []
superseded_by: ""
tags: [dead-code-gate]
related: [2, 12, 44, 53]
domains: [tooling]
---
# ADR-0063: Whole-Program Dead-Code Gate via deadcode

## Context

awf's gate (`./x gate`) already runs a strong static-analysis stack: `go test ./...`
with `-coverpkg`, then `cmd/covercheck` (a hard 100% statement-coverage floor,
ADR-0012), then `go vet ./...`, then `go tool golangci-lint run`, whose enabled set
includes `unused` (staticcheck U1000), `unparam`, `ineffassign`, and `wastedassign`.

That stack does **not** catch one class of waste: production code that is unreachable
from any `main` but kept alive by tests. golangci-lint's `unused` is package-scoped and
conservative about exported identifiers; it will not flag an exported function that no
production path calls as long as a test calls it. And the 100% coverage gate actively
*masks* this case: such a function is fully covered, so coverage says nothing is wrong.

A concrete instance exists today. `internal/project.Project.Sync` is a thin wrapper:
`func (p *Project) Sync() error { _, err := p.SyncReport(); return err }`. Production
(`cmd/awf/sync.go:runSync`) calls `p.SyncReport()` directly; **no** production path calls
`Sync()`. Eleven test files call it for ergonomics. It is 100%-covered, passes every
enabled linter, and is dead from production's point of view: exactly "kept alive because
it's tested."

Whole-program reachability analysis closes this gap. `golang.org/x/tools/cmd/deadcode`
performs rapid-type-analysis (RTA) reachability from each `main` and reports unreachable
functions. `golang.org/x/tools v0.44.0` is already an indirect dependency, so adopting the
tool is cheap. Running deadcode against the current tree confirms the analysis is precise
here: **without** `-test` it reports exactly `Project.Sync`, `internal/adr.SetNowForTest`,
and the sixteen helpers in `internal/testsupport` (incl. `internal/testsupport/gitfixture`):
nothing else; **with** `-test` it reports nothing.

Two forces shaped the design:

1. **`-test` mode is redundant here.** deadcode's `-test` flag makes test files entry
   points too. Under the 100% coverage gate every statement-bearing function is executed by
   some test, so `deadcode -test ./...` is empty and stays empty: it can never catch the
   target class. This redundancy is a *coupling to ADR-0012*, not an independent property:
   loosen the coverage floor and `-test` could start reporting. The valuable analysis is
   therefore the no-`-test` run, which treats only production `main`s as roots.

2. **The no-`-test` run legitimately flags `internal/testsupport`.** That package is the
   ADR-0044 sanctioned home for cross-package test helpers; its files are ordinary `.go`
   (not `_test.go`) *only because* Go cannot share `_test.go` helpers across packages. It is
   `internal/`, imported solely by `_test.go` files, and (by the ADR-0044 leaf invariant)
   imports no other `internal/*` awf package, so it can never keep a real awf function alive.
   Ignoring it is a structural boundary, not an ad-hoc exemption.

The remaining two findings are both test-only helpers sitting in production files, not
production code: `Project.Sync` is a prod-dead convenience wrapper (only tests call it), and
`SetNowForTest` is a test seam: the one exported `*ForTest` function in the repo (the
project's dominant seam idiom is the unexported package var, e.g. `var now`, `var getwd`,
which production calls and deadcode never flags). `SetNowForTest` only exists exported
because `internal/adr`'s external test package (`package adr_test`) cannot reach the
unexported `now`. The uniform resolution is to relocate each into an in-package `_test.go`
file, which removes it from the production binary entirely, the same principle that puts
cross-package helpers in `internal/testsupport`: test-only code belongs in test files.

deadcode itself always exits 0, even when it reports findings, so a wrapper is required to
turn "any surviving finding" into a non-zero exit, mirroring how `cmd/covercheck` converts
a coverage shortfall into a gate failure.

Scope is this repo's own gate only; promoting dead-code analysis into the rendered awf
*standard* is a separately load-bearing change and is not in scope.

## Decision

1. **Adopt `golang.org/x/tools/cmd/deadcode` as a pinned Go `tool` dependency** in `go.mod`
   (`tool golang.org/x/tools/cmd/deadcode`), run via `go tool deadcode`. This matches the
   ADR-0002 golangci-lint mechanism (pinned in `go.mod`, no separate install). `go mod tidy`
   pulls deadcode's transitive dep `golang.org/x/telemetry` (and `x/sys`) into `go.sum` as
   tool deps: the same kind of tree noise ADR-0002 accepted for golangci-lint.

2. **Run deadcode without `-test`.** The gate analyzes production reachability only. The
   `-test` mode is deliberately not used: it is empty under the ADR-0012 coverage floor and
   cannot catch prod-unused-but-tested code.

3. **Add `cmd/deadcodecheck`**, a small Go program that reads `deadcode -json` output from
   stdin, discards every finding whose `Position.File` begins with the repo-relative prefix
   `internal/testsupport/` (this covers `gitfixture`), prints any survivors, and exits
   non-zero iff any survive. It matches on `Position.File` (repo-relative), **not** the
   module-qualified package path; this assumes deadcode is invoked from the module root,
   which `./x` always is. The stdin→exit shape keeps it a pure, table-testable function,
   mirroring `cmd/covercheck`.

4. **Wire it as the final `./x gate` step**:
   `go tool deadcode -json ./... | go run ./cmd/deadcodecheck`. `./x` runs with
   `set -o pipefail`, so a failure in either stage fails the gate. Add a standalone
   `./x deadcode` alias for the same pipeline.

5. **No per-symbol escape hatch.** `cmd/deadcodecheck` honors no `//deadcode:ignore`
   directive and no allowlist. A surviving finding is a defect, resolved by deleting the dead
   code or refactoring it out, never by exempting it. The only ignore is the structural
   `internal/testsupport/` path boundary of Decision 3. Should a genuine analyzer
   false positive (reflection, interface-only dispatch, `//go:linkname`) ever arise, that is
   the trigger to revisit this decision with a concrete case, not a hatch built in advance.

6. **Resolve the two day-one findings so the gate is green on adoption, both by relocating
   a test-only helper out of production, not by deletion:**
   - Move `internal/project.Project.Sync` (the `func (p *Project) Sync() error { _, err :=
     p.SyncReport(); return err }` convenience wrapper) from `project.go` into a new
     `internal/project/export_test.go` declared `package project`. All 28 in-package test
     files (`package project`) keep calling `p.Sync()` unchanged; only the single
     cross-package caller `internal/evals/fixture_test.go` (which cannot see an in-package
     test helper) migrates to `if _, err := p.SyncReport(); err != nil`, with its adjacent
     doc comment updated. `Sync()` leaves the production binary, so deadcode stops seeing it;
     `SyncReport` remains the sole production sync entry point. The stale comment referencing
     `p.Sync()` at `cmd/awf/run_test.go:280` is updated for accuracy.
   - Move `internal/adr.SetNowForTest` (with its doc comment) from `adr.go` into a new
     `internal/adr/export_test.go` declared `package adr`; the external `adr_test.go`
     (`package adr_test`) calls it unchanged. The unexported `now` var stays in `adr.go`.

   Both fixes apply one rule: a test-only helper belongs in a `_test.go` file, never a
   production one. The gate-wiring step (Decision 4) must land *after* both, or the gate goes
   red mid-series.

7. **`cmd/deadcodecheck` is subject to the ADR-0012 100% coverage gate** like every other
   package; `./x` and the `go.mod` tool directive are hand-maintained repo files outside the
   awf render/lock set (ADR-0002), so they do not affect `awf check`.

## Invariants

- `invariant: deadcode-gate`: `./x gate` runs `deadcode` (no `-test`) over `./...` and fails when
  any reported unreachable function lies outside `internal/testsupport/`; `cmd/deadcodecheck`
  ignores exactly that path prefix and exits non-zero on every other finding. Backed by a
  `// invariant: deadcode-gate` marker on the `cmd/deadcodecheck` test that asserts a
  non-testsupport finding fails and a testsupport finding is ignored (lands with the
  implementation).
- deadcode is invoked without the `-test` flag; the gate never relies on `deadcode -test`,
  whose emptiness is a consequence of the ADR-0012 coverage floor rather than an independent
  guarantee. (textual)
- No `//deadcode:ignore` directive or symbol allowlist exists in `cmd/deadcodecheck`; the
  only suppression is the `internal/testsupport/` path boundary. (textual)
- `golang.org/x/tools/cmd/deadcode` is pinned via the `go.mod` `tool` directive; running the
  gate needs no separate install. (textual)
- No production (`.go`, non-`_test.go`) file outside `internal/testsupport/` contains a
  function unreachable from a `main`, enforced continuously by `inv: deadcode-gate`.

## Consequences

Easier:
- Prod-unused-but-tested code is caught mechanically, closing a gap the coverage gate
  actively masked. The `Project.Sync` class of waste can no longer accumulate.
- The gate now enforces the ADR-0044 intent that test-only helpers live in `internal/testsupport`
  (or in-package `_test.go` seams), not in production files: a leaked test seam fails the gate.

Harder / accepted trade-offs:
- Every `./x gate` runs an additional whole-program RTA analysis over all `main`s
  (`cmd/awf`, `cmd/covercheck`, and the new `cmd/deadcodecheck`), adding a few seconds per
  gate beyond the coverage build.
- `go.mod` / `go.sum` gain `golang.org/x/telemetry` and `x/sys` as tool-dep noise, relevant
  to `go mod tidy` at publish time: the same trade ADR-0002 already accepted.
- With no escape hatch, a future analyzer false positive would hard-block the gate until the
  code is refactored or this ADR is revisited. This is deliberate: the strictness is the
  point, and a real case is the right trigger for a hatch, not speculation.
- `internal/testsupport/` is ignored wholesale, so it becomes a dead-code blind spot: an
  exported helper there that no test calls anymore is flagged by neither this gate nor
  golangci-lint's package-scoped `unused` (the package is imported by `_test.go` files, so its
  exported funcs are never reported). Keeping that package's helpers all-live stays a manual
  discipline this gate does not enforce.

Ruled out (for now):
- A `//deadcode:ignore` directive or central allowlist (Decision 5).
- `deadcode -test` as any part of the gate (Decision 2).
- Promoting dead-code analysis into the rendered awf standard (out of scope, deferred).

Downstream work unblocked: an implementation plan covering: add the tool dep + `go mod tidy`;
write `cmd/deadcodecheck` with tests and the `// invariant: deadcode-gate` backing; relocate
`Project.Sync` to `internal/project/export_test.go` and migrate the single `internal/evals`
caller to `SyncReport`; move `SetNowForTest` to `internal/adr/export_test.go`; wire the
`./x gate`/`./x deadcode` step; and refresh the three generated narratives that name
`Project.Sync` as the eval entry point by editing their `.awf/` sources (`.awf/agents-doc.yaml`
(→ AGENTS.md), `.awf/docs/parts/testing/layout.md` (→ docs/testing.md), and
`.awf/domains/parts/tooling/current-state.md` (→ docs/domains/tooling.md)) to say `SyncReport`,
then re-render with `awf sync` (never hand-edit the rendered files); the `Project.Sync` mentions
inside the append-only ADR-0053 stay as historical record. When this ADR flips to Implemented,
the same commit backs the tagged invariant and regenerates `docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Gate on `deadcode -test ./...` | Always empty under the ADR-0012 coverage floor; catches only code unreachable even from tests, never the prod-unused-but-tested class that motivates this ADR. |
| Advisory `deadcode` report (no gate) | Relies on humans reading it; does not enforce "no unused production code." The findings are few and fixable, so a blocking gate is affordable. |
| Ship a `//deadcode:ignore` escape hatch up front | Both day-one findings are resolved by fixing the code, so the hatch would ship used zero times. A reason-required hatch invites tolerating dead code; defer until a real analyzer false positive justifies one. |
| Delete `Project.Sync` outright and migrate all 58 `p.Sync()` call sites | 57 of the 58 are in-package (`package project`) tests that legitimately use the wrapper's ergonomics; deleting churns eleven files for no design gain and treats `Sync` inconsistently with `SetNowForTest` (also a test-only helper, relocated not deleted). Relocating to `internal/project/export_test.go` removes it from production with a single-site (`internal/evals`) migration. |
| Rely on golangci-lint `unused` alone | Package-scoped and conservative about exported identifiers; it does not flag `Project.Sync` (a test calls it). Only whole-program reachability closes the gap. |
| Exclude `internal/testsupport` from deadcode's root/package set | deadcode needs the whole program to compute reachability correctly; the right lever is report-time path filtering, not shrinking the analysis scope. |
