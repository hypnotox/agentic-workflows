---
format: current-state-v4
slug: correct-phase-owner-path-latitude-claim-scope
status: Implemented
date: 2026-08-07
---
# ADR-0249: Correct Phase-Owner Path-Latitude Claim Scope


## Context

ADR-0248 deliberately grants omitted-path latitude only to commit-capable phase owners and keeps
commit-disabled helpers path-confined. Its direct implementation used the phrase "complete-
transaction owner" in the shared autonomy claim. Implementation review correctly found that this
phrase can include inline direct, bug-fix, test-first, and review-remediation owners even though the
approved decision names only delegated phase owners.

ADR-0248 is terminal, so its applied claim cannot be corrected through an amendment or Reapplied
event. The append-only lifecycle requires a follow-up update operation even though this correction
introduces no new design choice.

## Decision

1. `decision: exact-phase-owner-scope` State omitted-path latitude as belonging only to a
   commit-capable phase owner. The owner reports every added path as a reasoned deviation. Other
   implementation owners receive no new path authority, and commit-disabled helpers remain confined
   to assigned paths and report an unassigned-path need to the parent.

## State changes

- update `rendering/workflow-skill-templates:authority-guided-implementation-autonomy`

## Consequences

The active claim, shared autonomy contract, implementer contract, and ADR-0248 agree on the same
narrow authority boundary. Direct, bug-fix, test-first, and review-remediation consumers may render
the shared policy statement without acquiring omitted-path latitude themselves.

A small follow-up ADR is required solely because terminal claim provenance is append-only. This adds
lifecycle history but avoids silently mutating an already-applied operation or leaving current-state
authority broader than the approved decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave "complete-transaction owner" in the active claim | It broadens the authority boundary beyond the approved ADR-0248 decision. |
| Edit ADR-0248 or its terminal application | Terminal ADR content and applied claim provenance are frozen. |
| Narrow implementation prose but retain the broad claim | The rendered behavior would contradict active current-state authority. |

## Status history

- 2026-08-07: Proposed
- 2026-08-07: Implemented; content-sha256: e3c5d35d0563b2d2ffa068c78dcb88629fcce27ac0ff0a9c3a2f29cdf0ab2190
