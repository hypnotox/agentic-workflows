---
format: current-state-v4
slug: make-repository-check-results-owner-classified
status: Proposed
date: 2026-08-22
---
# ADR-make-repository-check-results-owner-classified: Make Repository Check Results Owner-Classified


## Context

ADR-0295 fixes the repository check ranks at Error and Warning, keeps Information unranked, and
classifies every existing check by protected property. ADR-0296 assigns each check one semantic
owner and reserves policy-free ordered aggregation for RepositoryChecker. ADR-0299 supplies one
Publisher-produced output plan and derived corpus set to every participating consumer. RF-004 must
realize those decisions without changing working or staged universes, result order, rendered output,
Error-only exit behavior, failure propagation, direct-child suppression, or compatibility
multiplicity.

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

- Top-level callers: `internal/project/operations.go`, `internal/project/staged_drift.go`, `internal/project/glossary.go`, `internal/publisher/glossary.go`, `cmd/awf/checkrepo.go`, `cmd/awf/init.go`, and `cmd/awf/publishing.go`; policy definitions currently remain concentrated in `internal/project/check.go`.
- Sibling tests: N=77, M=94
- Subpackage imports: `internal/contextq`, `internal/publisher`, `cmd/awf`, `cmd/releasecheck`, and `cmd/versioncheck` import `internal/project`; new check owners cannot import project or RepositoryChecker without preserving broad coupling or violating ADR-0296 direction.
- Cross-package methods / init(): no implicated package has an `init()` chain; exported `CheckReport`, `BuildCheckReport`, `AdvisoryNotes`, and `CheckStagedDrift` have command, evaluation, Publisher-test, and project-test callers whose compatibility projections must remain until migrated or proven unsupported.

## Decision

1. `decision: owners-classify-results` Each individual check owner emits immutable semantic results.
   Every ranked finding carries its fixed Error or Warning severity and the protected property that
   justifies that rank. Optional Information remains a separate unranked result and never becomes a
   third severity. Compatibility and presentation adapters may project those results into existing
   drift, warning, and note forms, but no consumer infers classification from a drift kind,
   presentation category, or slice selection.

2. `decision: repository-checker-aggregates-results` RepositoryChecker consumes completed owner
   results and performs only explicit deterministic aggregation. It does not implement individual
   check policy, prepare or rediscover inputs, rebuild Publisher plans or corpora, select policy
   through a registry or kind switch, or introduce configurable severity. Working and staged
   composition retain their distinct universes, established semantic order, output, failure versus
   finding behavior, compatibility multiplicity, and Error-only exit mapping.

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
bounded compatibility code. Current-state coordination, command use-case extraction, and
compatibility deletion remain assigned to later authorized work.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep classification implicit in aggregate slices | It leaves severity and protected property dependent on the coordinator and makes new checks edit unrelated aggregate policy. |
| Register checks through a plugin framework | The check set is closed and repository-owned; registration adds speculative indirection and contradicts ADR-0296. |
| Move all policy into RepositoryChecker files | File splitting would reduce line length without establishing one semantic owner or a policy-free aggregator. |
| Collapse working and staged preparation | Their snapshots and failure contracts are distinct, and current-state coordination remains reserved for RF-005. |

## Status history

- 2026-08-22: Proposed
