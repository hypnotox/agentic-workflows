---
format: current-state-v4
slug: retire-unrepresented-live-compatibility-recognition
status: Proposed
date: 2026-08-25
---
# ADR-retire-unrepresented-live-compatibility-recognition: Retire Unrepresented Live Compatibility Recognition


## Context

ADR-0297 permits removal only after the complete managed corpus no longer depends on a compatibility
surface. RF-008B removed the main below-floor migration, bridge, effort-memory, and plan-selector
stack, but its census overlooked two live recognition paths.

The live manifest parser still accepts and discards four retired ADR routing keys on a schema-30-or-
earlier lock even though the live floor is schema 46 and audit owns all represented pre-31 decoding.
The output pruner still recognizes the retired co-owned runner template identity and gives that
outgoing file a special one-time backup path. Every current lock in the ADR-0297 managed corpus is
schema 46, contains none of the routing keys, and contains no file with the retired runner identity.
Historical pre-31 routing remains represented in reachable Git history and therefore stays inside
audit's read-only decoder rather than the live manifest model.

The affected consumer boundary is closed:

| Consumer | Disposition |
|---|---|
| Config | Current semantic config remains schema 46 input; retired lock routing fields never enter config decoding. |
| Manifest | Live parsing keeps schema-first admission and rejects every retired routing key after admission. |
| Migrate | Schema-only classification and live-floor validation continue to refuse schemas below 46; the ordered future migration seam begins at 46 and needs no retired routing value. |
| Render and sync | Admitted operations continue to read the current lock before publication; no managed current lock presents a retired routing key or co-owned runner identity. |
| Audit | The separate historical decoder retains read-only schemas 3 through 46 and treats pre-31 routing fields as historical evidence rather than live authority. |

Other RF-014B candidates remain represented or current. They include authored ADR and plan formats,
schema-2 effort residents, missing initialization provenance, punctuation exemptions, historical
schemas, and generic journal recovery. Their consumers do not justify retaining these two overlooked
live paths.

## Decision

1. `decision: reject-retired-live-lock-routing` Live manifest parsing rejects the retired pre-31 ADR routing keys at every schema. Audit remains the sole decoder for represented historical routing fields and live operations retain their schema-floor refusal before authority dispatch.
2. `decision: remove-retired-runner-prune-recognition` Ordinary output pruning no longer recognizes or specially backs up the retired co-owned runner identity. Current local-document recovery, foreign-output backup, confinement, and prune reporting remain unchanged.

## State changes

- remove `rendering/companion-scripts:runner-prune-backup`
- update `rendering/project-output-plan:template-id-single-derivation`

## Consequences

The live manifest model no longer carries audit-only field tolerance, and the production template-ID
universe no longer includes a recognition-only retired runner. A source below the live floor still
receives the existing refusal and recovery direction before decoding; historical audit retains its
managed schema horizon. A stale unrepresented live lock that still names the old runner receives
ordinary managed-output pruning without the former one-time backup. This is intentionally unsupported
because no managed current lock contains that identity.

RF-014B can close after implementation and focused census evidence. The compatibility lane then
unblocks RF-010. Retained compatibility continues to follow ADR-0297's managed-consumer gate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Retain both paths indefinitely | Neither protects a current managed input, so retention contradicts the corpus-bounded removal rule. |
| Remove pre-31 historical routing decoding too | Reachable managed audit history still contains those fields. |
| Keep the retired runner backup as generic safety | It is identity-specific compatibility for an absent input; current local-document and foreign-output safety have separate owners. |

## Status history

- 2026-08-25: Proposed
