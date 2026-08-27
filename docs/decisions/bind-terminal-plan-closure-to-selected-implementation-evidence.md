---
format: current-state-v4
slug: bind-terminal-plan-closure-to-selected-implementation-evidence
status: Accepted
date: 2026-08-27
---
# ADR-bind-terminal-plan-closure-to-selected-implementation-evidence: Bind Terminal Plan Closure to Selected Implementation Evidence


## Context

Implementation plans remain amendable while Proposed and become terminal history when Implemented.
The workflow already requires the terminal transaction to reconcile what actually landed, including
material route deviations, but that contract was prose-only. A status-only staged diff cannot prove
the implementation paths accumulated before closure, and substring markers cannot prove a complete
reconciliation. Once terminal, a status regression or later body edit rewrites history even if one
portion of the representation happens to remain byte-identical.

The staged checker already compares selected repository states and parses plans. Terminal enforcement
belongs at that owner. It must preserve plan flexibility: actual outcome and authorization drift are
reconciled without turning original task paths, phases, or commit choreography into implementation
requirements.

## Decision

1. `decision: implemented-plan-is-frozen-history` Treat an Implemented plan's bytes, including its terminal status, as frozen history. Later status regression or content change is invalid.
2. `decision: closure-is-bound-to-selected-implementation-evidence` Permit Proposed-to-Implemented closure only when selected before-and-after repository evidence proves the complete actual touched-path set and the plan explicitly reconciles that set and every material route deviation. The contemporaneous status-only diff and unparsed marker presence are not sufficient evidence.
3. `decision: terminal-validation-fails-closed` Refuse terminal closure when comparison or reconciliation evidence is absent, malformed, ambiguous, or incomplete. Preserve ordinary Proposed-plan amendments and do not require original planned path, task, phase, or commit identity.

## State changes

- add `adr-system/plan-artifacts:terminal-plan-history-frozen`

## Consequences

Terminal plans become reliable historical records rather than editable workflow advice. Closure may
require an explicit evidence selector and a complete reconciliation even when the final staged change
only flips status. Repositories that cannot provide the selected comparison evidence must restore it
or defer closure rather than silently accepting an unverifiable record.

The check validates what landed, not whether implementation followed the plan's original
choreography. Proposed plans remain mutable throughout execution. The selected evidence and parsed
reconciliation gain durable repository-backed tests because prose markers alone cannot back the
invariant.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep terminal immutability as workflow prose | It permits silent historical rewrites and cannot close the known issue. |
| Validate only the status-flip diff | A normal terminal transaction contains the plan, not the preceding implementation range. |
| Compare landed paths with original task paths | It would contradict plan flexibility and confuse historical reconciliation with choreography compliance. |

## Status history

- 2026-08-27: Proposed
- 2026-08-27: Accepted; content-sha256: d525169771294fe88081b4bb71ed77764258886be13f085e33112008133ff24a
