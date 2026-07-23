---
format: current-state-v2
status: Proposed
date: 2026-07-23
---
# ADR-0154: Trajectory-close-owned anchor claims for causal resolution

## Context

The Pi telemetry causal integrity checker treats every envelope `piAnchorId` as a unique
ownership claim on a Pi session-tree entry: any entry claimed by more than one ledger event is
flagged `ambiguous-anchor` at violation severity and deleted from anchor resolution
(`internal/telemetry/causal.go`, mirrored in the dashboard's TypeScript causal projection
rendered from `templates/pi/awf-dashboard/index.ts.tmpl`). The dashboard producer, however,
stamps `piAnchorId` on essentially every passive observation as location metadata: parallel
tool observations in one batch share the leaf entry, the final-turn usage observation shares it
with session end, a resumed session start shares it with the prior session end, and every tree
navigation emits `session_associated` plus `trajectory_resumed` anchored at the same
destination entry. The producer and checker disagree about what the field means.

The consequences are structural, not cosmetic. Every real session accretes violation-severity
diagnostics (119 across the resident ledger when the ADR-0149 smoke surfaced the conflict on
2026-07-23, an observed count from that ledger). Worse, fork resolution is routinely defeated
by the ambiguity: `trajectory_forked` co-claims the fork entry with its own `trajectory_closed`,
so the fork edge the anchor map exists to provide is dropped on every fork. The roadmap deferred
this as "Anchor uniqueness is contested between producer and checker" because reconciling the
semantics is a load-bearing protocol choice, not a patch.

Grounding against the code established the full consumer set of the anchor map: resolution of
the four payload references (`trajectory_resumed.anchorId`, `trajectory_closed.anchorId`,
`trajectory_forked.forkAnchorId`, `effort_reopened.anchorId`) into causal frontier edges, the
`ambiguous-anchor` diagnostic, and one hidden consumer: the `trajectory_resumed`
association-invalidation clause in `internal/telemetry/lifecycle.go`, which detaches a session
association when the event claiming the resume anchor causally precedes the association event.
That clause only appears to work because co-anchoring usually empties the lookup: under any
claim restriction it inverts, firing on every normal tree-resume (the destination tip's old
`trajectory_closed` always precedes the freshly re-asserted association) while the
pre-association case it guards is claimed by kinds that would no longer claim. Causal order is
an invalid proxy for tree position. Separately, `trajectory_closed` events appended through the
lifecycle request path never receive an envelope `piAnchorId`; only passive-path closes do, so
an envelope-keyed claim rule would silently exclude lifecycle-path closes from resolution.

The three legitimate co-anchor patterns named by the roadmap item are confirmed in the
producer, with one wording correction: the "shutdown usage observation" is actually the
final-turn `usage_observed` sharing the leaf with `session_ended`.

## Decision

1. The envelope `piAnchorId` is observation-location metadata: "this event was observed while
the Pi tree leaf was entry X". The producer continues to stamp it on passive observations, and
the causal checker never reads it. It carries no uniqueness obligation and can never be the
subject of an `ambiguous-anchor` finding.

2. An anchor claim, the ability to be the target of a payload anchor reference, is made
exclusively by a `trajectory_closed` event and is keyed on that event's `payload.anchorId`.
This covers both the passive path and the lifecycle request path, which never stamps the
envelope field. Closing a tree position claims it; `trajectory_resumed`, `trajectory_forked`,
and `effort_reopened` reference positions, never claim them.

3. The claiming set is declared once in the embedded protocol descriptor
(`internal/telemetry/protocol.json`) as a new vocabulary, `anchorClaimKinds`, initially exactly
`["trajectory_closed"]`. Descriptor validation enforces that `anchorClaimKinds` is a subset of
`eventKinds`. The Go checker reads the set from the parsed descriptor and the TypeScript causal
projection reads it from the generated descriptor, so the two cannot drift. The protocol
version stays 2.0: event shapes are unchanged, and the inter-extension handshakes pin
`minor === 0`, so a minor bump would break compatible consumers rather than protect them.

4. Ambiguity is defined only within the claiming set: two `trajectory_closed` events claiming
the same entry remain an `ambiguous-anchor` violation, and the contested anchor is still
dropped from resolution. The known residual is repeat forking from one entry (fork at X, tree
back, fork at X again), which stays flagged; two forks genuinely branched from the same entry,
and this record accepts that as honest ambiguity rather than adding causal tie-breaking.

5. The `trajectory_resumed` association-invalidation clause that compares the anchor claimant's
causal position against the association event is removed from lifecycle validation in both the
Go projection and the TypeScript mirror. The trajectory-family membership check remains. The
pre-association-anchor scenario stays enforced where it is actually decidable: the producer
explicitly appends `session_detached` with reason `pre-association-anchor` (ADR-0146 decision
9); the ledger-side clause was an unsound causal proxy for tree position and is not replaced.

6. The resulting resolution shifts in both directions are intended. Fork and resume edges that
ambiguity silently dropped (including the fork edge on every fork today) are restored, and
references that previously resolved to an arbitrary unambiguous observation at the referenced
entry no longer resolve. Because diagnostics and causal order are projections over the resident
ledger, the accumulated `ambiguous-anchor` findings clear retroactively and canonical metrics
and doctor output over existing ledgers may shift accordingly; no historical event is rewritten.

7. The deferred roadmap item "Anchor uniqueness is contested between producer and checker"
graduates: this decision resolves it, and the roadmap entry is removed in the implementing
change, with the usage-observation wording corrected by this record's Context.

## State changes

- add `tooling/workflow-telemetry:anchor-claims-and-location-metadata`

## Consequences

- The checker stops manufacturing violations out of correct producer behavior, and fork
ancestry resolution starts working on real ledgers instead of being self-defeating. The 119
accumulated findings disappear from doctor output without any history rewrite.
- This is a real protocol-semantics change for any external writer, not a pure checker fix: an
event of a non-claiming kind can no longer be an anchor-resolution target, and the existing
test that anchors a `phase_started` event and expects resume and reopen references to resolve
to it must be rewritten to the new semantics.
- Dropping the association-invalidation clause narrows ledger-side defense in depth: a writer
that resumes a pre-association anchor without the producer's explicit `session_detached` is no
longer caught by the projection. This is accepted because the clause was unsound under either
semantics, only ever fired when co-anchoring happened not to suppress it, and the producer
contract from ADR-0146 decision 9 remains the enforced boundary. The corresponding lifecycle
test is removed or rewritten with the clause.
- `effort_reopened` anchor references lose nothing in practice: the causal link to the origin
effort travels in the explicit predecessor frontier. A hypothetical event relying solely on the
anchor edge with an empty predecessor list is protocol-legal but unused; its edge now resolves
only if the referenced entry was closed.
- Canonical metrics and doctor output over existing resident ledgers shift: cleared findings,
restored happens-before pairs, and removed observation-targeted edges. Consumers of those
projections observe the corrected semantics immediately after upgrading the binary.
- The descriptor gains a vocabulary, so all rendered Pi outputs re-render and the lock changes
with them; the change ships as one staged transaction.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Producer-side unique anchoring (stamp `piAnchorId` only on fork-resolvable events) | Leaves the accumulated violations in place until retention prunes those efforts, discards location telemetry that costs nothing to keep, and still flags legitimate repeat-fork navigation. |
| Causally-latest claimant resolution across all kinds | Most faithful in principle, but anchor resolution feeds the causal order being built, forcing two-pass construction; real complexity for marginal benefit over restriction. |
| Restricted claims plus latest-claimant tie-break within the set | Eliminates the repeat-fork residual at the cost of tie-break machinery; the residual is rare and genuinely ambiguous, so it stays a violation. |
| Hardcoding the claiming set in Go and TypeScript | Two lists that must never drift, when the descriptor already exists to be the single source of truth for both sides. |
| Keeping the association-invalidation clause on an alternative signal | No sound in-ledger signal distinguishes a pre-association anchor from normal tree-resume; entry ids are opaque, and causal order is a proven-invalid proxy for tree position. |
| Envelope-keyed claims (`piAnchorId` of `trajectory_closed`) | Lifecycle-path closes never carry the envelope field and would silently stop claiming; the payload anchor is the semantic content of the close. |

## Status history

- 2026-07-23: Proposed
