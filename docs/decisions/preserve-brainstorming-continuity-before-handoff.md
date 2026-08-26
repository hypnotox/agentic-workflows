---
format: current-state-v4
slug: preserve-brainstorming-continuity-before-handoff
status: Proposed
date: 2026-08-26
---
# ADR-preserve-brainstorming-continuity-before-handoff: Preserve brainstorming continuity before handoff


## Context

The continuity trigger already names preservation of settled decisions as a reason to create an
effort, but discovery explicitly includes analysis, option comparison, and selection. Brainstorming
checks continuity only after clarification and approach presentation, then checks it again after
approval. A material decision can therefore settle while work remains effort-free. If session
replacement becomes useful only then, autonomous creation scaffolds placeholder memory and carries
no obligation to recover the earlier discussion before handoff.

Pi handoff is already effort-only and resumes from repository authority plus owned effort memory.
The failure is therefore not an effort-free continuation mechanism. It is a late-created effort whose
memory omits decisions that the successor is required to treat as its continuity record. ADR-0186
requires settlement-level decision entries with exact user evidence, but checkpoint guidance only
backstops entries after an effort exists. Neither creation nor handoff closes the pre-creation gap.

ADR-0266 deliberately kept discovery effort-free, made creation autonomous, retained immutable
outcome titles and slugs, and required deliberate switching when an outcome changes. Those choices
remain sound. An uncertain final scope is not a reason to delay continuity ownership: a faithful
current outcome can own the discussion, while material outcome drift uses a new fixed-identity effort,
necessary context transfer, and the existing safe finish/archive lifecycle.

## Decision

1. `decision: establish-continuity-before-further-brainstorming` Evaluate continuity when
   brainstorming begins and whenever continuity-relevant facts change. Brainstorming may begin
   effort-free, but if work continues after its first settled material decision, continuity has become
   materially useful and effort-workflow creates or resumes ownership before further decision work.
   This timing rule is load-bearing because it bounds how much settled context can exist only in a
   volatile conversation without making every clarification or single-decision brainstorm create an
   effort.

2. `decision: initialize-late-created-memory` Creation after relevant discussion is incomplete for
   checkpoint and handoff purposes until the one user-managed writer initializes the owned memory
   from retained evidence. Initialization records the current outcome in the Brief, every already
   settled decision with the provenance and `Record:` evidence required by ADR-0186, relevant
   observations, and current phase and next action. If exact required user evidence is unavailable,
   reconfirm it instead of fabricating or weakening the record.

3. `decision: preserve-fixed-outcome-identity` Keep the effort title and slug fixed. Refinements
   inside the owned outcome remain in that effort. Material outcome drift deliberately creates a new
   fixed-identity successor, transfers the necessary still-valid context, verifies that the successor
   is resumable, and closes the obsolete effort through existing topology safety and finish/archive
   lifecycle. Add no retitle operation, effort schema change, or history-deleting lifecycle.

## State changes

- update `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:effort-workflow`
- update `rendering/pi-runtime:pi-session-handoff-workflow`

## Consequences

A multi-decision brainstorm gains durable ownership no later than its first settled decision, and a
late-created effort becomes a truthful continuation record before the volatile session can be
replaced. A single-decision discussion can still remain effort-free through approval and completion,
so brainstorming does not mechanically imply an effort.

Initialization can be verbose and may require another user interaction when exact evidence was lost.
That cost is accepted because reconstructing consent would make the memory appear more authoritative
than its evidence. The writer must distinguish still-relevant context from exploratory material and
record settlements rather than copying a transcript.

Effort names remain faithful to the outcome known when continuity begins rather than to a final
implementation scope. Material drift costs a deliberate successor transition and archival cleanup,
but continuity remains auditable and no routine rename must rewrite resident or Git identity.
Temporary coexistence during a deliberate successor transfer does not authorize unrelated parallel
efforts or silent reuse.

The workflow, rendered skills, current-state claims, and deterministic semantic tests change. The
effort record schema, CLI, worktree implementation, and Pi handoff kickoff remain unchanged.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the current trigger timing and backfill only immediately before handoff | It closes the final boundary but permits an arbitrarily long settled design to remain volatile and makes recovery depend on context still being retained. |
| Create an effort for every brainstorm at entry | It is simple but couples independent triggers and creates continuity state for one-response clarifications that do not need it. |
| Permit effort title or slug renaming after scope becomes clear | Naming was not the loss mechanism, and mutable identity would weaken continuity ownership or require topology migration. |
| Put prior discussion into the handoff kickoff | ADR-0273 intentionally keeps kickoff effort-only; duplicating memory in transient kickoff prose restores competing authority. |

## Status history

- 2026-08-26: Proposed
