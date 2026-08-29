---
format: current-state-v4
slug: decompose-repeated-assurance-findings-by-semantic-owner
status: Proposed
date: 2026-08-29
---
# ADR-decompose-repeated-assurance-findings-by-semantic-owner: Decompose repeated assurance findings by semantic owner


## Context

A coherent implementation transaction can still be too broad for effective independent assurance.
When one phase or settlement spans several semantic owners, a reviewer must reason about unrelated
models, dependencies, and proof surfaces at once. A large digest can then produce another broad
settlement and another broad review shape, even when the findings cluster by owner and could be
resolved independently.

The workflow already treats plan choreography as mutable route detail, permits commit-capable owners
to regroup phases while the protected contract holds, and limits ordinary implementation review to
one initial review plus at most one verify pass. It also requires fresh integration coverage after
separate work is composed. Those rules provide the mechanism for smaller assurance units without
adding a numeric size threshold, weakening review, or turning every task into a commit boundary.

The approved outcome is to split oversized implementation and review units by semantic owner and to
treat repeated findings of the same class as evidence that the current assurance boundary is too
broad. Unrelated blockers continue to follow the existing rule: record and route them separately
without expanding the active outcome.

## Decision

1. `decision: semantic-owner-assurance-units` Before assigning or reviewing a broad implementation
   unit, the parent identifies its semantic owners and separates independently verifiable owners into
   distinct implementation, settlement, and assurance units. Work remains together only when its
   cross-owner composition is itself one coherent transaction or protected contract.

2. `decision: repeated-findings-trigger-decomposition` When an initial review or its bounded verify
   pass returns repeated findings of the same class across separable semantic owners, the parent
   treats that pattern as evidence of an oversized assurance boundary. Remaining work and subsequent
   assurance are decomposed by owner rather than sent through another same-shaped broad settlement
   or review loop.

3. `decision: preserve-composition-assurance` Decomposition never drops cross-owner composition
   coverage. The parent retains focused evidence for each fresh unit, reviews integration effects
   after composition, and preserves the existing single verify-pass bound, complete-range audit, and
   terminal assurance obligations.

4. `decision: semantic-not-numeric` No file, line, commit, task, finding-count, or elapsed-time
   threshold defines an oversized unit. The boundary follows semantic ownership, dependency and
   representation boundaries, independent verifiability, and the concrete finding pattern. The exact
   regrouping remains implementation route detail and is reconciled into a mutable plan only when
   another phase or reviewer can rely on it.

## State changes

- add `rendering/workflow-skill-templates:semantic-owner-assurance-decomposition`

## Consequences

Reviewers receive narrower, more cohesive proof surfaces and parents can settle one owner without
reopening unrelated correctness. Repeated finding patterns become actionable feedback about the
assurance boundary rather than justification for another broad loop.

The parent must spend more effort identifying owners and preserving composition coverage. Some
changes cannot be decomposed because their value lies in one cross-owner transaction; the decision
allows those to remain together when the dependency is explicit and reviewable. Smaller units may
create more focused commits or reviews, but they do not create task-level checkpoints or relax the
one-concern rule.

The rule is intentionally qualitative. It cannot mechanically classify semantic ownership, so
rendered workflow contracts and deterministic scenarios prove the routing while code-design and
current-state authority determine the boundary in each implementation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep broad units and rely only on the one-verify-pass limit | It bounds review count but does not reduce repeated cross-owner reasoning or settlement breadth. |
| Split after a fixed finding or file count | Counts are weak proxies for ownership and would fragment cohesive work while missing small but cross-cutting units. |
| Make every task an assurance unit | Tasks are ordered route detail, not necessarily coherent green transactions or review boundaries. |
| Drop integration review after focused owner reviews | Separate correctness does not prove cross-owner composition. |

## Status history

- 2026-08-29: Proposed
