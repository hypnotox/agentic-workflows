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
but this decision does not prescribe their file inventories or perform their extractions.
`docs/architecture.md` remains the high-altitude architecture authority and receives only the concise
allowed direction. Current-state claims own the detailed extraction map and forbidden reverse edges.
Existing code-design authority still requires immutable long-lived state, operation-owned
derivation, consumer-owned dependencies, one semantic home, owner-rendered results, and direct
concrete composition unless real substitution requires more.

## Decision

1. `decision: dependency-direction` Direct dependencies from `cmd/awf` through focused application operations to immutable project state and domain services, and from those owners to semantic Git, snapshot, filesystem, publication, and rendering mechanisms. Command code may compose and invoke `Loader` and the focused operations. Application operations may invoke `Publisher`, `RepositoryChecker`, and `CurrentStateCoordinator`; those coordinators may consume `ProjectState`, domain services, and only the semantic mechanisms their operation needs. `Loader` may consume configuration, catalog, repository, and filesystem opening mechanisms to construct `ProjectState`. `Publisher` may consume rendering, filesystem, and atomic publication mechanisms. `CurrentStateCoordinator` may consume ADR, topic, plan, current-state, snapshot, and Git owners. `RepositoryChecker` may consume individual checker results, while individual check owners never depend on the aggregator. No internal package imports `cmd/awf`; project state, domain, checker, and mechanism owners never import application coordination; `ProjectState` never imports `Loader`; and `internal/project` never imports `internal/contextq` while the existing reverse edge remains. Foundational mechanism chains such as snapshot to Git and filesystem to atomic file publication remain legal. Cheap, real reverse edges receive focused import-test backing rather than a configurable dependency framework.
2. `decision: extraction-owners` Assign one semantic owner to each planned extraction: `ProjectState` owns immutable loaded facts; `Loader` owns opening and validation; `Publisher` owns output planning, rendering coordination, backup decisions, and publishing; `RepositoryChecker` owns policy-free ordered aggregation only; `CurrentStateCoordinator` owns ADR and topic transition coordination; focused application operations own command-level use cases; and `cmd/awf` owns parse, compose, invoke, result rendering, stream choice, and exit mapping. Individual check policy remains with one concern owner: generated-output conformance with `GeneratedOutputChecker`; managed reference validity with `ReferenceChecker`; plan validity with `PlanChecker`; pitfall validity with `PitfallChecker`; glossary and tag vocabulary with `VocabularyChecker`; configuration and command-spec consistency with `ConfigurationChecker`; current-state validity with `CurrentStateCoordinator`; punctuation restraint with `internal/prosegate`; memory-citation refusal with `internal/memorycite`; commit authorization with `internal/commitpolicy`; and advisory repository analysis with `internal/audit`.
3. `decision: boundary-values` Cross boundaries with immutable loaded facts; semantic ADR, topic, plan, output-declaration, policy, finding, and operation-result values; owner-produced presentation documents; and immutable semantic snapshots such as `snapshot.Tree`. Keep Git-native objects, mutable repository or index state, filesystem handles and metadata, temporary publication paths, template parse state, and other mechanism representations inside their mechanism boundary. `ProjectState` does not expose mutable configuration, catalog, map, or slice aliases as its public state.
4. `decision: proportional-operations` Represent focused application operations with direct functions or small structs and use direct concrete dependencies where substitution is unnecessary. Do not introduce an all-purpose application object, service locator, check plugin system, provider-owned universal interface, or speculative abstraction.

## State changes

- add `code-design/dependency-composition:repository-layer-direction`
- add `code-design/dependency-composition:repository-extraction-owners`
- add `code-design/dependency-composition:repository-boundary-values`

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

Realizing the target across RF-002 through RF-006 has migration cost. Transitional composition will
exist while extractions land serially, boundary values will need deliberate translation, and tests
must move with their semantic owners without changing behavior. Each issue must keep that transition
bounded and must not preserve old and new coordinators as competing permanent paths.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave ownership implicit in the current package list | It does not give RF-002 through RF-006 a shared extraction target or identify reverse dependencies. |
| Move files first and infer the architecture afterward | It risks circular movement and competing owners before the dependency map exists. |
| Introduce ports, registries, or a plugin framework for every operation and check | No multiple real implementations justify that abstraction, and it would replace broad coordination with framework overhead. |
| Put application coordination into existing domain or mechanism packages | It would mix policy with lower-level ownership and create reverse-import pressure. |

## Status history

- 2026-08-20: Proposed
