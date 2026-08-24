---
format: current-state-v4
slug: hybrid-raw-coverage-ratchet-and-targeted-mutation-regression
status: Accepted
date: 2026-08-24
---
# ADR-hybrid-raw-coverage-ratchet-and-targeted-mutation-regression: Hybrid raw coverage ratchet and targeted mutation regression


## Context

ADR-0012 makes the repository gate block below 100 percent statement coverage after
`// coverage-ignore` filtering. That rule prevents silent coverage loss, but it also makes every
uncovered block either a test obligation or an exclusion candidate. The pressure can reward
production seams or tests shaped for the metric rather than for behavior. It also treats a moved
uncovered block as harmless when another miss disappears, because an aggregate percentage cannot
preserve identity.

The immutable RF-009 evidence profile contains 22,247 statements, of which 21,526 are covered and
721 are raw misses. The profile is stable when merged as one whole-module profile. Six critical
selectors derived from that same whole profile contain 24 misses out of 398 statements for hard
safety, 53 of 3,133 for state authority, 162 of 3,352 for repository and effort lifecycle, 118 of
3,357 for migration and recovery, 115 of 2,978 for publication and application, and 46 of 1,086 for
the `cmd/awf` boundary. The hard-safety packages report 374 of 398 covered from the whole profile
and 373 of 398 from a local-owner profile. The difference demonstrates why local-owner profiles
are useful diagnostics but cannot replace the canonical whole-derived evidence.

The source census contains 806 directive lines: 733 map to blocks in the canonical profile and 73
do not. Four unmapped production directives are Darwin or Windows immediate rollback branches in
`internal/effort/publication_darwin.go` and `internal/effort/publication_windows.go`; a Linux profile
cannot measure them. Seven mapped ignored bodies currently have positive execution counts. Earlier
review found eight false exclusions in aggregate, but that historical number does not establish a
stable identity list or a grandfathered exception.

ADR-0065 keeps raw and filtered Codecov line coverage informational. ADR-0066 keeps mutation testing
advisory because broad mutation is too slow and equivalent-mutant noise is too high for a general
blocker. The narrower `cmd/covercheck` surface is different: three deterministic runs found the same
11 mutants, with 9 killed, 2 genuinely surviving at `main.go:42:13`, and no timeouts. Both survivors
exist because the tests assert only that subtraction reports a failure, so addition produces the
same observed result. This exact surface is small enough to protect within the admitted budget.

## Decision

1. `decision: raw-identity-ratchet` Replace ADR-0012's filtered 100 percent blocker with a raw
   uncovered-block identity ratchet. An identity is the module-relative file, exact start and end
   line and column, and statement count from the merged whole-module profile. The gate blocks any
   raw identity absent from its admitted baseline. It compares sets, not totals, so an unrelated
   removal cannot authorize an addition or a moved span.

2. `decision: generated-baseline-owner` Keep the canonical tracked policy in the generated root
   `coverage-baseline.json`, owned by `internal/coverage` and emitted through `cmd/covercheck`. It
   records repository and selector identities, explicit reasons for admitted additions and moves,
   directive inventories and evidence, the static platform ledger, and exact reviewed equivalent
   mutants. A profile may remove baseline misses without approval, and canonical regeneration drops
   them automatically. Every addition or moved identity requires a stored reason and independent
   review, and the existing repo-local audit registry reports each as Warning evidence. Missing,
   malformed, noncanonical, or unavailable baseline evidence is a blocking verification failure.

3. `decision: critical-selectors` Derive all six blocking selectors from the same whole-module
   profile, with these exact package roots:

   - hard safety: `internal/filepublication`, `internal/commitpolicy`, `internal/coverage`,
     `cmd/covercheck`, and `cmd/mutants`;
   - state authority: `internal/adr`, `internal/currentstate`, `internal/currentstatecoord`, and
     `internal/topic`;
   - repository and effort lifecycle: `internal/git`, `internal/effort`, and `internal/worktree`;
   - migration and recovery: `internal/config`, `internal/migrate`, and `internal/upgrade`;
   - publication and application: `internal/project` and `internal/publisher`;
   - command boundary: `cmd/awf`.

   Local-owner profiles remain diagnostic and never satisfy a blocker.

4. `decision: percentages-report-only` Continue reporting the raw statement percentage and the
   filtered statement percentage, including a filtered 100 percent result when present, but make
   neither percentage a blocker. Preserve ADR-0065's raw and covered Codecov line flags as
   informational reports, and preserve `covercheck --emit-filtered` through the same directive
   interpretation used by the statement report.

5. `decision: retained-ignore-admission` Retain a production `// coverage-ignore: <reason>` only
   for one of four classes: a directly tested process-exit seam, a revalidated impossible state, a
   safely uninducible deterministic fault, or a platform-only branch with explicit evidence. The
   generated baseline inventories production and test-source directives separately, requires an
   admitted class and evidence for every retained production directive, and never lets a test-source
   directive satisfy a production entry.

6. `decision: platform-ledger` Begin with a static, explicitly unmeasured ledger for the four
   Darwin and Windows publication rollback directives. A host profile must not claim to measure
   them. Their source identities, platform constraints, retained-ignore class, and evidence remain
   reviewable even while the canonical Linux profile cannot map them.

7. `decision: executed-ignore-error` Terminal adjudication of the seven currently measured ignored
   bodies must leave no positively executed guarded body ignored. A measured ignored body with a
   positive execution count is an Error because its exclusion claim is false. Keep
   `coverage-ignore-added` as a complementary repo-local Warning. Preserve the historical eight only
   as aggregate context, not as identities or exceptions. The implementation plan owns the concrete
   correction mechanics.

8. `decision: targeted-mutation-blocker` Refine ADR-0066 only for exact `cmd/covercheck` changes.
   A change under the `cmd/covercheck` owned path triggers blocking mutation of `./cmd/covercheck`;
   mutation remains advisory everywhere else. Local staged selection and explicit-range CI selection
   call one shared fail-conservative ownership detector, so unavailable or uncertain change evidence
   runs the blocker rather than silently skipping it.

9. `decision: mutation-trust-contract` Pin gremlins v0.6.0 and explicitly pin the current operator
   set: arithmetic base, conditionals boundary, conditionals negation, increment/decrement, and
   invert negatives. Use the deterministic integration recipe with one worker. Limit a blocking run
   to 900 seconds; a missing, malformed, incomplete, or timed-out result is invalid. A survivor
   passes only when its exact identity is baseline-listed as independently reviewed equivalent.

10. `decision: mutation-renewal` After a gremlins version, operator set, or deterministic recipe
    change, require three complete mutation runs to produce the same trusted status set within a
    total 25-minute renewal budget before accepting renewed baseline evidence.

11. `decision: test-backed-policy` The four added claims, `coverage-raw-identity-ratchet`,
    `coverage-ignore-admission`, `coverage-executed-ignore-errors`, and
    `covercheck-mutation-regression`, are test-backed invariants. Each lands with `Backing: test` and
    valid proof annotations on durable tests owned by its implementation surface.

12. `decision: behavior-first-oracles` Never add a production seam, distort production control flow,
    weaken an assertion, or broaden an ignore merely to improve a coverage or mutation result.
    Behavioral evidence remains the oracle; metrics detect regression in that evidence. Add no
    second profile, rigor mode, threshold layer, or broad mutation gate under this decision.

## State changes

- remove `tooling/quality-gates:coverage-gate-100`
- add `tooling/quality-gates:coverage-raw-identity-ratchet`
- add `tooling/quality-gates:coverage-ignore-admission`
- add `tooling/quality-gates:coverage-executed-ignore-errors`
- add `tooling/quality-gates:covercheck-mutation-regression`
- update `tooling/quality-gates:covered-profile-honors-ignores`
- update `tooling/quality-gates:gate-severity-by-protected-property`

## Consequences

Coverage can fall only through an exact, reasoned, independently reviewed baseline addition. A
covered former miss improves the ratchet without negotiation, and a moved span stays visible even
when the uncovered statement count is unchanged. Critical safety and state surfaces retain their own
identity views without another test run or profile.

Filtered 100 percent stops being an accountability claim. Raw and filtered percentages remain useful
trend reports, while the identity baseline carries enforcement. Codecov behavior and its line-versus-
statement distinction remain unchanged. Existing `coverage-ignore` syntax and the filtered-profile
interface remain compatible.

The baseline becomes durable policy data that must travel with source movement. Canonical generation
reduces hand editing, but admitted regressions and equivalent mutants still require human judgment.
Platform-only entries are honest about being unmeasured on Linux rather than disappearing from the
inventory. Test directives remain visible without diluting production admission rules.

Mutation adds bounded local and CI cost only when `cmd/covercheck` changes. Timeouts, incomplete
reports, and configuration drift fail closed. Broad mutation and the roadmap question of mutation for
nominal invariant proofs remain outside this decision. The exact blocker can later widen only through
a separate decision with measured budget and noise evidence.

Partial enforcement would be unsafe because a baseline without its blocker, or a blocker without its
trusted evidence, cannot prove the policy. No adopter-facing configuration or shipped workflow
contract changes, because the checker, baseline, runner, CI wiring, and audit prompt are
repository-local development tooling.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep filtered 100 percent blocking | Continues metric pressure and hides identity-preserving swaps behind an aggregate pass. |
| Use a lower aggregate percentage | Permits unrelated well-covered code to mask new misses and cannot distinguish a moved block. |
| Add per-package or owner-local blockers | Adds profiles or layers and makes package-local coverage blindness authoritative. |
| Block every raw miss at zero | Recreates 100 percent pressure without the honest retained baseline. |
| Keep ignores without classes or executed-body validation | Leaves false reachability claims mechanically indistinguishable from genuine exclusions. |
| Gate broad mutation | The measured budget and equivalent-mutant noise do not admit a deterministic repository-wide blocker. |
| Keep all mutation advisory | Leaves the two proven weak `cmd/covercheck` assertions without regression control despite an admitted exact budget. |
| Shape production for metric reachability | Makes production serve a metric rather than behavior and weakens the oracle boundary. |

## Status history

- 2026-08-24: Proposed
- 2026-08-24: Accepted; content-sha256: 8aa65de13e681adee3792959abaff17174152a6752c9ed16e12436f391556564
