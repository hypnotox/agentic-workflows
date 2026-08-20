---
format: current-state-v4
slug: define-repository-dependency-and-ownership-target
status: Proposed
date: 2026-08-20
---
# ADR-define-repository-dependency-and-ownership-target: Define Repository Dependency and Ownership Target


## Context

The repository already has focused domain and mechanism packages, but coordination remains broad in
`internal/project` and `cmd/awf`. `Project` combines loaded facts with output planning, repository
checks, current-state transitions, sync, and audit operations. Command code also composes repository
checks from project, snapshot, configuration, report, and presentation representations. This makes
future ownership unclear even though the current Go import graph is acyclic.

Useful lower-level directions are already established. `internal/currentstate` consumes ADR,
configuration, snapshot, and topic models; snapshot implementations consume the semantic Git seam;
and confined filesystem code consumes atomic file publication. `internal/contextq` currently imports
`internal/project`, so a reverse edge from project would cycle. Application coordination placed in a
mechanism or domain package would similarly pull policy into a lower owner and create reverse-import
pressure.

RF-002 through RF-006 will separate state, publication, repository checks, current-state
coordination, and command operations. Those issues need one shared target before any file movement,
but this decision does not prescribe their file inventories or perform their extractions. Existing
code-design authority still requires immutable long-lived state, operation-owned derivation,
consumer-owned dependencies, one semantic home, owner-rendered results, and direct concrete
composition unless real substitution requires more.

## Decision

1. `decision: dependency-direction` Direct dependencies from `cmd/awf` through focused application operations to immutable project state and domain services, and from those owners to semantic Git, snapshot, filesystem, publication, and rendering mechanisms. Lower layers never import application coordination, and project or domain owners never depend on command code.
2. `decision: extraction-owners` Assign one semantic owner to each planned extraction: `ProjectState` owns immutable loaded facts; `Loader` owns opening and validation; `Publisher` owns output planning and publication coordination; `RepositoryChecker` owns ordered check aggregation only; each individual check owner owns one semantic concern; `CurrentStateCoordinator` owns ADR and topic transition coordination; focused application operations own command-level use cases; and `cmd/awf` owns parse, compose, invoke, render, and exit.
3. `decision: boundary-values` Cross boundaries with immutable loaded facts and semantic domain, finding, operation-result, and presentation values. Keep raw Git objects, snapshots, filesystem handles and metadata, temporary publication state, template internals, and other mechanism representations inside their mechanism boundary.
4. `decision: proportional-composition` Use direct functions or small structs for application operations and direct concrete dependencies where substitution is unnecessary. Do not introduce an all-purpose application object, service locator, check plugin system, provider-owned universal interface, or speculative abstraction.
5. `decision: focused-dependency-guard` Mechanically protect cheap, real forbidden reverse dependencies with a focused import guard rather than a configurable dependency framework.

## State changes

- add `code-design/dependency-composition:repository-layer-direction`
- add `code-design/dependency-composition:repository-extraction-owners`

## Consequences

RF-002 through RF-006 gain a common ownership and dependency map, so each extraction can choose a
bounded file route without inventing a competing architecture. The direction prevents application
coordination from settling in domain or mechanism packages and exposes likely cycles before code is
moved. The boundary-value rule keeps volatile representations out of policy while allowing direct
concrete composition.

The map deliberately leaves exact destination files, operation names, constructor shapes, and
extraction order changeable. Individual issues must still ground their own coupling and move tests
with the behavior they relocate. A focused guard can prevent selected reverse imports cheaply, but
it is not a complete architecture verifier and does not replace code review.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave ownership implicit in the current package list | It does not give RF-002 through RF-006 a shared extraction target or identify reverse dependencies. |
| Move files first and infer the architecture afterward | It risks circular movement and competing owners before the dependency map exists. |
| Introduce ports, registries, or a plugin framework for every operation and check | No multiple real implementations justify that abstraction, and it would replace broad coordination with framework overhead. |
| Put application coordination into existing domain or mechanism packages | It would mix policy with lower-level ownership and create reverse-import pressure. |

## Status history

- 2026-08-20: Proposed
