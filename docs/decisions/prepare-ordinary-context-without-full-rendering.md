---
format: current-state-v4
slug: prepare-ordinary-context-without-full-rendering
status: Proposed
date: 2026-08-26
---
# ADR-prepare-ordinary-context-without-full-rendering: Prepare ordinary context without full rendering


## Context

An ordinary explicit `awf context <path>...` request currently pays for the complete working-state preparation used by broader repository operations. A warm command on this repository took about 1.3 to 1.45 seconds, and a cold observation took 3.26 seconds. A phase probe attributed about 590 milliseconds to complete working-tree snapshot preparation, 289 milliseconds to Publisher preparation and full output rendering, and 77 milliseconds to eager whole-tree query projection. The migration and version gate took only 2 milliseconds.

The work is largely independent of the requested path. The working snapshot reads every discovered file, Publisher renders the complete output plan to expose semantic corpora and declarations, and the query computes impacts for every file before selecting an exact path. These broad operations preserve one operation universe, but ordinary context needs a narrower set of facts: complete path and mode metadata for classification and directory census, authority and marker inputs, requested artifact bytes where their content affects the answer, semantic corpora, and planned-output declarations.

Context must retain its neutral boundary below application coordination and keep classification and projection in the query owner. Publisher must remain the owner of its semantic corpora and declaration policy. The common ordinary path may stop validating unrelated rendering behavior, while staged, range-selected, and uncovered queries retain their current complete-universe preparation. Successful ordinary output remains unchanged.

## Decision

1. `decision: focused-ordinary-context-universe` Ordinary explicit context consumes one operation-owned, type-distinct repository view containing complete path and mode inventory plus immutable bytes selected for the answer. Selection remains within the current practical sequential filesystem consistency model, without persistent caching, mutation detection, or retry. Staged, range-selected, and uncovered context retain their existing preparation routes.
2. `decision: context-semantic-declaration-projection` Publisher provides ordinary context with its semantic corpora, plans, and output declarations without rendering output bytes or applying validations unrelated to the answer. Successful ordinary context output remains byte-identical, while unrelated rendering failures no longer prevent orientation.
3. `decision: demand-driven-context-projection` Context classification and projection remain owned by the context query boundary and become demand-driven: exact requests compute only their required impacts, and directory requests expand only their matching descendants from indexed inventory.

## State changes

- update `tooling/context-and-topic:context-query-boundary`
- update `tooling/context-and-topic:context-read-only`
- update `rendering/project-output-plan:check-report-single-plan`

## Consequences

Ordinary orientation no longer scales with reading and rendering the whole repository when its answer needs only a focused byte selection. Complete inventory preserves absence checks, symlink and generated-output classification, directory grouping, and selector semantics. Required marker sources, reverse plan references, and requested artifact bytes remain part of the selected input when the answer depends on them.

The repository gains two preparation routes and a new inventory-plus-selection representation. Their behavioral overlap must be protected by differential output tests, routing tests, read-selection tests, and declaration-parity tests. Performance evidence must record latency, allocation, file-read, and byte-read changes on a representative repository.

Ordinary context intentionally stops acting as an incidental render validator. A malformed input needed for its answer still fails, but an unrelated template, generated output, or render validation failure does not. Full checks and the non-ordinary context routes retain their existing validation responsibilities.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Optimize only eager query projection | It addresses roughly 77 milliseconds while leaving the dominant snapshot and full-render work intact. |
| Add a persistent context cache | Invalidation and stale-authority risks would add mutable cross-operation state that ordinary orientation does not require. |
| Reuse historical audit selections | A sparse historical selection cannot represent complete live path inventory or distinguish an unread path from repository absence. |
| Preserve every current Publisher failure | Retaining unrelated render and output validation would keep substantial unnecessary work and preserve an accidental context contract. |

## Status history

- 2026-08-26: Proposed
