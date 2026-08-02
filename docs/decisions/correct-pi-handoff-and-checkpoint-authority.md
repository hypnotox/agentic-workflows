---
format: current-state-v3
slug: correct-pi-handoff-and-checkpoint-authority
status: Implemented
date: 2026-08-02
---
# ADR-correct-pi-handoff-and-checkpoint-authority: Correct Pi handoff and checkpoint authority

## Context

ADR-0209 made `handoff_session` a kickoff-only session-replacement mechanism. ADR-associate-pi-sessions-with-efforts-and-live-checkout-context later
introduced effort memory metadata and association support, but its applied handoff claim updates
incorrectly restored the removed optional-memory handoff contract. That contradiction expanded a
runtime boundary that ADR-0209 deliberately kept independent of effort lifecycle and checkpoint
policy.

The same integration left checkpoint guidance with competing metadata-writing instructions and
the minimum-runtime claim with both scoped capability detection and a contradictory universal
pre-registration floor. The binary now owns structured memory metadata updates, while workflow
guidance remains responsible for checkpoint eligibility, plan projection, handoff boundaries, and
append-only logs. Active memories require temporary identity compatibility between canonical YAML
and the legacy header.

## Decision

1. Restore ADR-0209 as the current handoff authority. `handoff_session` accepts only bounded
   kickoff prose and retains its single-use queue, countdown, pending-request identity guard,
   lineage, cleanup, editor fallback, execution-time UTF-16 bound, and isolated optional Remote Pi
   continuation-disposition emission. It accepts no memory path or effort input.
2. Make exactly one `awf effort memory update` invocation the sole Phase, Next, and Updated writer
   at each checkpoint. Guidance may separately append decisions, observations, and the actual
   handoff boundary to `## Handoff log`.
3. Keep executable plan projection non-boundary-forming. During migration, guidance accepts
   canonical YAML `effort: <slug>` and deprecated legacy `Effort: <slug>` identity; remove the
   legacy form after active efforts finish.
4. Scope the minimum-runtime pre-registration floor to APIs required by the retained subagent and
   handoff contracts. `using_effort` has no foreign package-version floor and feature-detects
   `changeCwd` at its call boundary, visibly refusing without state mutation when it is absent.

## State changes

- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-runtime:pi-minimum-runtime`

## Consequences

Handoff remains cohesive session replacement, workflow guidance preserves checkpoint policy, and
`using_effort` degrades at its own optional call boundary rather than inheriting a universal
pre-registration floor. The follow-up corrects current authority without changing terminal
ADR-0209 or ADR-associate-pi-sessions-with-efforts-and-live-checkout-context history. Legacy effort memories remain usable for their active migration
window.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Restore optional memory validation in handoff | It contradicts ADR-0209 and duplicates workflow policy at the runtime boundary. |
| Keep direct checkpoint metadata edits beside structured update | It creates competing writers for the same fields. |
| Rewrite ADR-0217 | Terminal ADR history is frozen; this follow-up changes current-state claims forward. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Implemented; content-sha256: 823607f2f1c93297f55efb1e8fe587bd4bc93c4ce46fb73190ab72fc1f3e56e1
