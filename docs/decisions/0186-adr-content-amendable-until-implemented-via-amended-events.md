---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0186: ADR content amendable until Implemented via Amended events

## Context

A current-state-v2 ADR freezes its canonical content the moment it leaves Proposed. Two
mechanisms enforce this: `adr.FrozenContentEqual` (internal/adr/format.go) admits a content
change across a checked pair only while the before side is Proposed, and parse-level
validation (`validateV2StatusEntry`) requires every stamped `content-sha256`, on Accepted,
Implementing, Implemented, and Abandoned events alike, to equal the digest recomputed from
the record's current content. A correction that arrives after the freeze, however small,
therefore needs a whole successor ADR.

The 2026-07-30 severity work showed the cost: ADR-0184 exists only to correct a claim
ADR-0183 established, ADR-0185 exists only to correct ADR-0184, and ADR-0182 still owes a
successor for a single claim-wording correction. Each of these records was forced into
existence because review-driven corrections arrived after its predecessor left Proposed.
Plans already model the desired shape: a plan stays amendable through execution and locks
when the work completes.

Grounded facts shaping the design, verified at main b3069c5a: `FrozenContentEqual` has one
production caller (internal/currentstate/transition.go); `parseAppliedOperations` rejects an
Applied operation that is not declared in State changes or out of declaration order, and
`historiesEqual` compares Applied events' operation lists, so applied batches are already
immutable across transitions; no production code outside internal/adr consumes
`HistoryEvent.Digest`; the existing corpus holds 45 V2 records carrying 32 non-terminal
digest stamps, every one equal to its record's other stamps; and the Implemented flip is
owned today by the execution skills and happens before terminal review, so review findings
land after the record froze.

## Decision

1. A current-state-v2 ADR's canonical content (the five digest-covered sections) is
   amendable while its status is Proposed, Accepted, or Implementing, and freezes
   permanently when the record reaches a terminal status (Implemented or Abandoned). A
   terminal record is a locked historical record exactly as today. current-state-v1 records
   keep the existing freeze-after-Proposed rule unchanged.
2. current-state-v2 Status history gains a third event kind, the amendment event, with the
   exact grammar `- <date>: Amended; content-sha256: <64 lowercase hex characters>`. An
   Amended event is legal only at a point in the event stream where the current status is
   Accepted or Implementing. Amendment while Proposed stays event-free, and nothing appends
   after a terminal event.
3. Digest validation becomes a stamp chain and drops per-event equality with current
   content. Only an Amended event introduces a new digest, and its digest must differ from
   the immediately preceding stamped digest. A status event never introduces a digest: it
   must repeat the immediately preceding stamped digest, or establishes the record's first
   stamp when no stamped event precedes it. The latest stamped digest must equal the digest
   computed from the record's current content. Each appended stamp is verified against
   actual content once, by the transition check of the commit that appends it, and is
   trusted retained history thereafter, the same trust model as Applied events.
4. In the authored-commit transition contract, an amendment is one authoring step: a
   same-status pair at Accepted or Implementing may append exactly one Amended event, and a
   commit that appends an Amended event appends nothing else to the Status history. The
   merge-aggregate contract is unchanged; prefix preservation plus parse-level chain
   validation already admit any legal ordered mix.
5. An amendment must not alter or remove a State-changes operation already referenced by an
   Applied event. Adding new operations and rewording operations not yet applied stay
   legal. This is enforced by the existing history-prefix rule and the
   declared-and-declaration-ordered requirement on Applied operations; the implementation
   pins it with tests rather than new machinery.
6. `FrozenContentEqual` becomes format-aware: for V1 a content change remains legal only
   from a Proposed before side; for V2 a content change is legal from any non-terminal
   before side and refused when the before side is terminal.
7. The Implemented flip moves out of the execution skills into terminal review: the final
   Applied batch and the Implemented status event land after the implementation review
   settles and before worktree integration and retrospective. Review findings ride
   amendment commits under the same record before the flip.
8. Amendment routing is a workflow rule in the rendered skills: an amendment that changes a
   Decision item or the meaning of an invariant claim is raised as a user-decision before
   landing, unless it corrects a clear defect with a no-brainer fix. Prose-only
   clarification stays autonomous.
9. The published standard's prose travels with the change: the embedded templates for the
   decisions guide and decisions template, the adr-lifecycle skill (the
   amendment-while-Proposed section becomes amendment-until-Implemented, carrying items 5
   and 8), the execution and reviewing skills (item 7), and the workflow doc, plus this
   project's agent-guide append-only bullet and the adr-system domain narrative.
10. The change is purely additive: no corpus migration and no schema-generation bump. Every
    existing record, and any new record that is never amended, carries equal stamps and is
    valid unchanged under the chain rule; legacy non-terminal stamps become ordinary
    snapshot history rather than a validated freeze assertion.

## State changes

- add `adr-system/adr-lifecycle:adr-amendable-until-terminal`
- update `adr-system/adr-lifecycle:adr-status-enum-and-matrix`

## Consequences

- A correction that arrives while a record is Accepted or Implementing rides the same
  record as an amendment commit; successor chains like ADR-0183 to ADR-0185 stop being
  created. Records already terminal stay frozen, so corrections to them are still made
  forward through a later ADR's State changes.
- The digest stops asserting one frozen value and becomes a content version chain. A middle
  stamp is a historical snapshot verified once at append time; it is no longer
  re-verifiable from the file alone, and git history holds the superseded content versions.
- The bookkeeping is self-enforcing: amending content without appending the matching
  Amended event leaves the latest stamp mismatching the computed digest, and the record
  fails to parse.
- Commit choreography changes at the end of implementation: the record is Implementing
  while terminal review runs, and the final Applied batch plus Implemented flip form the
  post-review commit. The generated decision index lists the record as in flight until
  then.
- A heavily amended record accumulates history lines; the routing rule in Decision item 8
  keeps decision-changing amendments deliberate.
- `validateV2History`'s unknown-event-kind catch-all is annotated on the premise that only
  two event kinds exist; the Amended branch lands before it and the stale annotation is
  corrected in the same change.
- Records that use Amended events require a current awf binary to parse; the existing
  binary-version gate covers this, and adopters that never amend see no difference.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep freeze-after-Proposed | Every post-freeze correction spawns a successor ADR; the corpus shows chains of three. |
| Drop non-terminal digests, tolerate legacy stamps unvalidated | Loses in-record amendment provenance and leaves a permanent unvalidated-legacy carve-out in the grammar. |
| Strip non-terminal digests via an upgrade migration | Mutates retained history events and costs migration machinery plus a schema-generation bump for no semantic gain. |
| Re-stamp non-terminal digests on each amendment | Routinely mutates retained events; a stamp stops recording anything true about its date. |

## Status history

- 2026-07-30: Proposed
