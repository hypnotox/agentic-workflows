---
format: current-state-v4
slug: make-publisher-the-output-plan-construction-owner
status: Implementing
date: 2026-08-21
---
# ADR-make-publisher-the-output-plan-construction-owner: Make Publisher the Output Plan Construction Owner


## Context

ADR-0296 assigns output planning and rendering coordination to Publisher, requires project state,
domain owners, checkers, and mechanism owners not to import application coordination, and permits
semantic output declarations and plans to cross boundaries as immutable values. The active
`rendering/project-output-plan:check-report-single-plan` claim still describes the pre-extraction
shape: `Project.CheckReport` constructs the operation plan and threads it through repository-check
projections.

Moving plan construction to Publisher creates a dependency constraint. Repository checks, staged
checks, context projection, and other residual consumers must reuse the Publisher-produced plan, but
they cannot import the application coordinator without reversing ADR-0296's dependency direction.
Leaving a second construction path in project would preserve the import direction only by splitting
output-planning ownership and making single derivation unenforceable.

## Decision

1. `decision: publisher-constructs-operation-plan` Publisher is the sole construction owner for an output plan. It derives one immutable plan per operation and that same plan, or a semantic projection of it, is threaded to every output, drift, tracking, advisory, staged, and context consumer that participates in the operation.
2. `decision: neutral-plan-values-below-coordination` Output declarations and plan values have a neutral semantic owner below application coordination. Publisher produces those values, while residual check and current-state consumers receive them without importing Publisher or reconstructing output-planning policy.

## State changes

- update `rendering/project-output-plan:check-report-single-plan`

## Consequences

Output planning has one producer even when its plan serves consumers that remain outside Publisher.
Repository-check and current-state policy can stay with their existing owners without creating a
reverse dependency on application coordination. The active single-plan claim can describe the
post-extraction direction without weakening its operation-scoped reuse guarantee.

A neutral boundary adds a distinct value owner and translation surface. Its values must remain
semantic and immutable rather than carrying rendering, filesystem, snapshot, or application
coordination behavior. Consumers may project or inspect the produced plan for their own policy, but
may not rebuild it or introduce a compatibility planner.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep plan construction in project checks | It would retain split ownership after Publisher becomes responsible for output planning. |
| Let check and current-state packages import Publisher | It would reverse the approved dependency direction by making lower policy owners depend on application coordination. |
| Let Publisher own both construction and consumer-facing value types | Consumers outside application coordination would still need a Publisher dependency merely to receive the plan. |

## Status history

- 2026-08-21: Proposed
- 2026-08-21: Implementing; content-sha256: 5087f4d9abff126b57f87d68ab25503b1de541b70f30390171d7c2d06dba6d4f
- 2026-08-21: Applied; operations: update `rendering/project-output-plan:check-report-single-plan`
