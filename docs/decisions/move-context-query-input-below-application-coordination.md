---
format: current-state-v4
slug: move-context-query-input-below-application-coordination
status: Proposed
date: 2026-08-23
---
# ADR-move-context-query-input-below-application-coordination: Move Context Query Input Below Application Coordination


## Context

ADR-0195 and ADR-0234 placed context-query assembly behind a project-owned context-state seam with two project constructors. RF-005's ADR-0296 extraction instead separates application coordination from immutable state and lower semantic owners. The implemented context path selects one working or staged universe through focused current-state coordination, reuses Publisher-produced semantic corpora where Publisher participates, and passes a neutral immutable input to `internal/contextq`. The old claim still requires the superseded project seam and is therefore false despite the implementation preserving context behavior and universe isolation.

## Decision

1. `decision: neutral-context-query-input` Context queries consume one neutral immutable context input below application coordination. Focused current-state coordination selects the operation universe and assembles that input from lower semantic values, including Publisher-produced corpora where Publisher participates. The context-query owner performs classification, projection, and semantic presentation mapping without importing project or application coordination; bounded project compatibility adapters may delegate to the focused coordinator without restoring the former project-owned context-state seam.

## State changes

- update `tooling/context-and-topic:context-query-boundary`

## Consequences

Current-state authority matches the implemented dependency direction and no longer requires obsolete project constructors. Context operations retain independently selected working and staged universes, Publisher remains the producer of its semantic corpora, and `internal/contextq` remains the classification and projection owner. The neutral input becomes a load-bearing boundary whose immutability and import direction require focused structural proof.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Restore the project-owned context-state seam and two constructors | It would preserve stale wording by reversing the approved extraction and recreating obsolete coordination. |

## Status history

- 2026-08-23: Proposed
