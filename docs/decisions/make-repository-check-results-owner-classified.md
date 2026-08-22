---
format: current-state-v4
slug: make-repository-check-results-owner-classified
status: Implementing
date: 2026-08-22
---
# ADR-make-repository-check-results-owner-classified: Make Repository Check Results Owner-Classified


## Context

ADR-0295 fixes the repository check ranks at Error and Warning, keeps Information unranked, and
classifies every existing check by protected property. ADR-0296 assigns each check one semantic
owner and reserves policy-free ordered aggregation for RepositoryChecker. ADR-0299 supplies one
Publisher-produced output plan to every participating consumer; the RF-004 preparation-reuse
boundary and current Publisher preparation seam also preserve reuse of derived corpora. RF-004 must
realize those decisions without changing working or staged universes, finding contents, category
membership, Error-only exit behavior, failure propagation, direct-child suppression, or compatibility
multiplicity. Aggregation and presentation remain deterministic, but relative placement among items in
one Warning list is not a compatibility boundary.

The current implementation does not yet carry that meaning at its boundary. `internal/project/check.go`
combines individual policies, compatibility projection, and aggregate construction. Ranked errors
are represented as `manifest.Drift`, ranked warnings as strings, and protected property is implicit.
Aggregate presentation later infers Error from drift and obtains Warning from a separate slice.
Information is correctly unranked but is mixed with legacy Notes projections. This makes a check's
classification depend on its aggregation path rather than the semantic owner that produced it.

Working and staged checking also have intentionally distinct preparation universes. The working
aggregate retains separate report, current-state, and index preparations and executes drift, state,
prose, then memory. Staged checking retains lock, state, then Publisher drift. Each operation must
reuse its Publisher plan and corpora without rebuilding them, while RF-005 retains current-state
coordination. The extraction therefore needs neutral immutable result values below application
coordination, not a reverse import, mutable shared state, or a check registry.

### Coupling audit

The moved surface comprises `CheckReport`, `BuildCheckReport`, `AdvisoryNotes`,
`CheckStagedDrift`, and the private generated-output, reference, plan, pitfall, vocabulary,
configuration, advisory, and compatibility functions in `internal/project/check.go`.

- Top-level callers: `internal/project/check.go:56`, `internal/project/check.go:139`, `internal/project/check.go:474`, `internal/project/check.go:505`, `internal/project/check.go:526`, `internal/project/check.go:543-559`, `internal/project/check.go:844`, and `internal/project/check.go:908`; exported facade definitions are in `internal/project/operations.go:39-44` and `internal/project/staged_drift.go:15`.
- Sibling tests: N=77, M=94; test references occur in `internal/evals/fixture_test.go`, project check, drift, inplace, note, surface, export, helper, and project tests, Publisher catalog, config-reference, glossary, inplace, and helper tests, and command check-group, check-report, aggregate, and run tests.
- Subpackage imports: no internal production subpackage imports the moved exported result or operation symbols; production command callers are `cmd/awf/checkrepo.go:39-92`, `cmd/awf/init.go:177`, and `cmd/awf/publishing.go:62`. Evaluation and Publisher coupling is test-only. New check owners cannot import project or RepositoryChecker without preserving broad coupling or violating ADR-0296 direction.
- Cross-package methods / init(): no implicated package has an `init()` chain; `CheckReport` methods at `internal/project/check.go:407-432` supply command compatibility projections. RF-004 preserves every exported result and operation projection; deletion remains later authorized compatibility work.

## Decision

1. `decision: owners-classify-results` Each individual check owner emits immutable semantic results.
   Every ranked finding carries its fixed Error or Warning severity and the protected property that
   justifies that rank. Optional Information remains a separate unranked result and never becomes a
   third severity. Compatibility and presentation adapters may project those results into existing
   drift, warning, and note forms, but no consumer infers classification from a drift kind,
   presentation category, or slice selection. GeneratedOutputChecker emits the managed agent-guide
   size Warning with its fixed protected property, while RepositoryChecker only preserves that
   result's aggregate-only placement.

2. `decision: repository-checker-aggregates-results` RepositoryChecker consumes completed owner
   results and performs only explicit deterministic aggregation. It does not implement individual
   check policy, prepare or rediscover inputs, rebuild Publisher plans or corpora, select policy
   through a registry, or introduce configurable severity. Working and staged composition retain
   their distinct universes, established semantic operation order, rendered finding content and
   categories, failure versus finding behavior, direct-child suppression, plan-note deduplication,
   compatibility projections and output multiplicity, and Error-only exit mapping. It preserves
   semantic operation and presentation-category order and produces deterministic lists without
   reproducing legacy relative placement among Warning items.

## State changes

- update `tooling/cli:repo-check-capability-plan`
- update `tooling/cli:check-severity-by-protected-property`
- update `rendering/project-output-plan:check-report-single-plan`
- update `rendering/sync-and-drift:agent-guide-size-advisory`

## Consequences

A finding's classification becomes part of the owner-produced semantic boundary, so adding or
moving a check does not require a second severity switch in aggregation. RepositoryChecker becomes a
small ordered composition point, and tests can prove that every ranked result declares the property
it protects. Existing command presentation and compatibility projections remain adapters over the
new model rather than competing policy homes.

The extraction moves policy and tests across package boundaries and requires explicit translations
for existing `manifest.Drift`, warning, and note callers. Preserving those projections costs some
bounded compatibility code. Warning contents, category membership, multiplicity, and deterministic
presentation remain stable, but consumers and tests cannot rely on legacy relative placement among
Warning items. Current-state coordination, command use-case extraction, and compatibility deletion
remain assigned to later authorized work.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep classification implicit in aggregate slices | It leaves severity and protected property dependent on the coordinator and makes new checks edit unrelated aggregate policy. |
| Register checks through a plugin framework | The check set is closed and repository-owned; registration adds speculative indirection and contradicts ADR-0296. |
| Move all policy into RepositoryChecker files | File splitting would reduce line length without establishing one semantic owner or a policy-free aggregator. |
| Collapse working and staged preparation | Their snapshots and failure contracts are distinct, and current-state coordination remains reserved for RF-005. |
| Reproduce legacy Warning-item order through compatibility partitions | Relative placement within one Warning list is not a protected compatibility boundary, so preserving it does not justify additional compatibility machinery. |

## Status history

- 2026-08-22: Proposed
- 2026-08-22: Implementing; content-sha256: bf4cf6ba626fa793c16a2e55945b265e551e11bf13ab9fe0ae0d89bec30acde1
- 2026-08-22: Applied; operations: update `tooling/cli:repo-check-capability-plan`, update `tooling/cli:check-severity-by-protected-property`, update `rendering/project-output-plan:check-report-single-plan`, update `rendering/sync-and-drift:agent-guide-size-advisory`
- 2026-08-22: Amended; content-sha256: bbd8c48cc788494f8e6f3c6d663c5dc8f20c504afdf9bd4e371a386e8f758a02
