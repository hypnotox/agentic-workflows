---
format: current-state-v4
slug: performance-budgeted-parallel-verification
status: Implementing
date: 2026-08-28
---
# ADR-0314: Performance-budgeted parallel verification


## Context

ADR-0313 separates a fast commit gate from exhaustive verification, but it deliberately serializes
Go package execution after concurrent sync-heavy fixture packages exceeded the ten-minute package
timeout on the local Btrfs host. The resulting commit gate is fast while terminal assurance remains
too slow for comfortable use.

On the stable Go 1.26.4 toolchain with warm caches and an exact empty mutation universe, the landed
commit gate completed in 1.204 seconds. The full gate completed in 625.426 seconds, of which the
serial profiled Go lane consumed 605 seconds. The dominant package times were 241.931 seconds for
`cmd/awf`, 143.529 seconds for `internal/project`, and 92.477 seconds for `internal/publisher`.
`internal/evals` consumed 23.146 seconds and `internal/contextop` consumed 15.137 seconds. Coverage
policy evaluation itself took one second, the Pi runtime smoke took six seconds, and every remaining
reported stage together took about thirteen seconds.

The suite repeats heavyweight project and Git fixtures across broad command matrices, runs benchmark
loops in an ordinary correctness test, invokes some exported tests both directly and through umbrella
tests, renders full evaluation fixtures redundantly, and constructs a GoReleaser snapshot inside the
ordinary Go suite even though CI constructs another snapshot. The command package also swaps mutable
package globals, which prevents safe test parallelism and makes the largest package the critical
path. These costs are not unique behavioral assurance.

The coverage representation is independently wasteful. A measured whole-module profile occupied
about 105 MB and contained 1,316,328 rows for only 16,876 unique block identities, roughly 78 copies
per identity. A prior warm comparison measured 174.9 seconds without coverage and 179.3 seconds with
the gate-shaped profile, so removing coverage would not solve the dominant test cost. ADR-0302 also
requires exact raw identities, six critical selector projections, directive evidence, and one
whole-derived policy result. Any faster collection recipe must preserve those semantics rather than
replace them with an aggregate percentage.

The host has twelve logical CPUs but the reliable gate currently leaves most of them idle. Earlier
sharding of the unrefactored dominant packages reached only about a 2.6x improvement because fixture
and filesystem contention limited scaling. Performance therefore requires removing unnecessary work
and repairing ownership boundaries before applying bounded concurrency. It also requires separating
the common local feedback workload, ordinary full assurance, hosted critical path, and exceptional
range-selected mutation path rather than hiding them behind one elapsed number.

## Decision

1. `decision: qualified-performance-contract` Make verification latency an explicit acceptance
   property with separately qualified workloads. On the current twelve-logical-CPU local reference
   host with the stable toolchain and warm caches, the fast gate plus affected-package behavioral
   feedback targets 10 to 15 seconds. An ordinary full gate without selected mutation must improve
   by at least 4x from the 625.426-second baseline, at or below 156 seconds, and retains 55 seconds as
   the stronger target. The hosted CI critical path targets 60 seconds on its declared runner class.
   Exceptional `cmd/covercheck` mutation has a separate budget. A repository-owned, versioned
   qualification record identifies the local CPU, OS, architecture, filesystem, memory, exact
   toolchain, cache preparation, and sample method, plus the hosted runner image and architecture.
   Qualification uses repeatable timing evidence and component regressions rather than making one
   noisy wall-clock observation a flaky correctness result. The performance contract lands as a rule
   verified through that qualification record, not as a per-run test-backed timing invariant.

2. `decision: oracle-preserving-consolidation` Remove behaviorally redundant execution while
   retaining every distinct safety, recovery, rollback, topology, mutation-boundary,
   invariant-backing, and CLI exit or output oracle. A broad matrix remains only when its cells prove
   distinct behavior; shared policy is tested once at its owner with representative boundary cases
   at callers. Performance work must not weaken an assertion, expected behavior, coverage admission,
   or required platform assurance.

3. `decision: affected-package-feedback` Keep ADR-0313's commit gate unchanged and add a separate
   fail-closed affected-package behavioral feedback path. It selects changed Go packages, reverse
   dependents, and declared repository meta-suites. Shared generators, templates, configuration,
   tooling, or uncertain change evidence widen to the required safe universe. This path is the
   common local test-feedback workload; exhaustive full verification remains mandatory at terminal,
   pre-push, CI, and release boundaries. Selection and conservative widening land as a test-backed
   invariant.

4. `decision: bounded-parallel-assurance` Execute necessary full assurance through deterministic,
   contention-qualified package shards and independent stage dependencies rather than an
   unconditional package-parallelism flag. Hosted CI may spend more aggregate compute to run native
   Go shards, Pi behavior, analysis and policy, platform compilation, coverage aggregation, and
   release verification concurrently. Stable required conclusions depend on every required lane for
   the exact revision.

5. `decision: equivalent-coverage-collection` Keep ADR-0302's current whole-module recipe
   authoritative until a replacement proves exact equivalence. A sharded, dependency-scoped, binary,
   or other compact recipe is acceptable only when its deterministic merged result preserves the
   complete instrumented universe, exact covered and uncovered block identities, all six critical
   selector projections, directive execution evidence, and filtered-profile semantics. Profile
   merging has one owner and rejects mixed modes, malformed inputs, path ambiguity, or any identity
   mismatch. Coverage remains blocking in ordinary full verification during qualification.

6. `decision: ci-owned-release-artifact-validation` Remove GoReleaser construction and production
   archive portability validation from the ordinary local Go suite. The stable release-configuration
   CI lane constructs one exact snapshot and runs the production validator against those same bytes.
   Synthetic archive tests remain local only for fast validator behavior; they do not substitute for
   the exact artifact proof.

7. `decision: instance-owned-test-seams` Make mutable command-process dependencies instance-owned at
   the existing `cmd/awf` composition boundary rather than package-global or testing-only production
   policy. The runner remains unexported, receives explicit one-operation dependencies, and continues
   to derive command membership and policy from `internal/clispec`; it is not a universal dependency
   bag or a second command registry. This ownership lands as a test-backed invariant.

8. `decision: immutable-fixture-seeds` Expensive reusable fixtures cache immutable representations or
   read-only seeds and clone explicitly for mutation. Shared live mutable roots are forbidden. This
   fixture-ownership boundary lands as a test-backed invariant in the test-infrastructure topic.

## State changes

- add `tooling/quality-gates:verification-performance-contract`
- add `tooling/quality-gates:affected-package-feedback`
- update `tooling/quality-gates:gate-tier-cadence`
- update `tooling/quality-gates:coverage-raw-identity-ratchet`
- update `tooling/quality-gates:exact-revision-repository-acceptance`
- update `tooling/changelog-and-release:release-gate-on-tag`
- add `tooling/cli:cli-runner-instance-ownership`
- add `tooling/test-infrastructure:immutable-fixture-seeds`

## Consequences

Performance becomes a reviewable system property rather than an occasional timing complaint. The
fast gate remains a narrow commit boundary, while developers gain behaviorally meaningful common
feedback and terminal verification becomes usable again. CI latency can fall through parallel jobs
even when aggregate hosted compute rises.

The implementation must distinguish unique behavior from historical coverage-shaped enumeration.
Moving an invariant marker, consolidating an umbrella, or replacing a command matrix requires
evidence that the retained owner still proves the claim. Parallel fixture reuse also requires
immutable state or explicit cloning; sharing live mutable roots would trade latency for races and
nondeterminism.

Affected-package selection adds a maintained policy surface. Conservative false positives are
intentional, and shared or uncertain changes may widen beyond the 10 to 15 second common-feedback
target or require the full universe. The qualification record reports those widened workloads
separately rather than misclassifying them as ordinary common feedback.

Coverage collection may change representation but not policy meaning. Exact equivalence prototypes
add temporary implementation work, and deterministic shard aggregation becomes a new trusted
boundary. The current collection remains the fallback until the replacement proves the entire
universe and every projection, so a failed experiment creates no assurance gap.

Release packaging defects may reach CI after local tests and lint are green. This is intentional:
CI becomes the single owner of the expensive production artifact construction and validation, while
local synthetic tests retain quick validator feedback. Required exact-revision conclusions continue
to block acceptance and release.

A 55-second full gate and 60-second hosted critical path are stronger targets, not claims that
unrefactored work will scale linearly. The 4x full-gate threshold remains the minimum accepted
outcome. If contention-aware restructuring cannot reach it without weakening evidence, the design
must return for a new decision rather than silently relaxing assurance.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Remove coverage or `-coverpkg` first | Warm coverage overhead was small relative to test execution, and losing exact cross-package identities would weaken ADR-0302. |
| Delete `-p=1` without restructuring | The complete gate already timed out under concurrent sync-heavy fixtures; a flag change does not fix contention or mutable ownership. |
| Shard the existing suite only | Measured sharding improved about 2.6x and remained above the minimum full-gate budget because duplicated fixtures still contended. |
| Perform only obvious duplicate cleanup | Lower risk, but it leaves the dominant command composition boundary and CI critical path serial and is unlikely to guarantee 4x. |
| Introduce a broad unit/integration taxonomy immediately | Adds classification policy and drift before evidence shows that bounded ownership-based shards are insufficient. |
| Move hard coverage policy to CI only | Saves little warm time and weakens local terminal authority before an equivalent compact collector is proven. |
| Keep constructing release snapshots in ordinary tests and CI | Repeats expensive artifact work instead of validating the exact CI-produced bytes once. |

## Status history

- 2026-08-28: Proposed
- 2026-08-28: Implementing; content-sha256: 0f3755ac6673043c4f5407821e5591f04503a4f3d26446bfb8f20017771d2249
- 2026-08-28: Applied; operations: add `tooling/quality-gates:verification-performance-contract`
- 2026-08-28: Applied; operations: add `tooling/cli:cli-runner-instance-ownership`, add `tooling/test-infrastructure:immutable-fixture-seeds`
- 2026-08-28: Applied; operations: update `tooling/quality-gates:coverage-raw-identity-ratchet`
- 2026-08-28: Applied; operations: add `tooling/quality-gates:affected-package-feedback`, update `tooling/quality-gates:gate-tier-cadence`
- 2026-08-28: Reapplied; operations: add `tooling/quality-gates:affected-package-feedback`
- 2026-08-28: Reapplied; operations: add `tooling/quality-gates:affected-package-feedback`
- 2026-08-28: Amended; content-sha256: c3a5d0ee473870aa6bf37245d877e0639f5cddd4ace22bc07f12215d61d32bfb
- 2026-08-28: Applied; operations: update `tooling/quality-gates:exact-revision-repository-acceptance`, update `tooling/changelog-and-release:release-gate-on-tag`
