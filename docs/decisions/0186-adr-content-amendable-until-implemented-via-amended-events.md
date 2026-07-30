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
`HistoryEvent.Digest`; the existing corpus holds 42 V2 records carrying 31 non-terminal
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
   after a terminal event. An Amended event may not sit between the final Applied event and
   an explicit Implemented status event: the existing rule that an explicit Implemented
   transition follows its final Applied event immediately is preserved.
3. Digest validation becomes a stamp chain and drops per-event equality with current
   content. Only an Amended event introduces a new digest, and its digest must differ from
   the immediately preceding stamped digest. A status event never introduces a new digest:
   it repeats the immediately preceding stamped digest, or, when no stamped event precedes
   it (the Proposed scaffold carries no stamp), establishes the record's first stamp, which
   must equal the digest of the record's current content. The latest stamped digest must
   equal the digest computed from the record's current content. Each appended stamp is
   verified against actual content once, by the transition check of the commit that appends
   it, and is trusted retained history thereafter, the same trust model as Applied events.
   This is the canonical change behind the update of
   `adr-system/adr-lifecycle:adr-status-enum-and-matrix`: the recognized V2 history-event
   kinds gain Amended, and the digest transition becomes a chain rather than per-event
   equality with current content.
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
   before side and refused when the before side is terminal. The doc comments on
   `FrozenContentEqual` and `ContentDigest`, which state the freeze-at-Accepted rule this
   decision replaces, are corrected in the same change.
7. The Implemented flip moves out of its current owners into terminal review: the final
   Applied batch and the Implemented status event land after the terminal review that
   settles last, immediately before managed-worktree removal and retrospective. When a
   divergent merge forces a renewed post-merge review, that renewed review is the one that
   settles the flip, so the flip commit lands after integration and every review finding,
   including a post-merge one, rides an amendment commit under the same still-amendable
   record. The owning surfaces are the
   `procedure-adr-final-commit` section of executing-plans, the `final-task-adr-flip`
   section of subagent-driven-development, the no-plan direct-implementation path in
   adr-lifecycle, and the ownership statement in reviewing-adr's `status-flip` section.
8. Amendment routing is a workflow rule in the rendered skills: an amendment that changes a
   Decision item, the State changes operation set, or the meaning of an invariant claim is
   raised as a user-decision before landing, unless it corrects a clear defect with a
   no-brainer fix. Prose-only clarification stays autonomous. A load-bearing amendment
   additionally re-dispatches the ADR reviewer over the amended sections before landing,
   per the existing retrospective lesson; the pitfalls entry recording that lesson is
   updated to the widened amendment window in the same change.
9. The published standard's prose travels with the change: the embedded templates for the
   decisions guide and the decisions template, which state the V1 and V2 amendment rules
   side by side just as the guide already states the format cutoffs; the adr-lifecycle
   skill's description and intro (which name amendment-while-Proposed), its states table
   data (the `adrStates` Accepted and Implementing rows in internal/catalog/standard.go,
   whose meaning and mutability strings assert the freeze), its transitions section, its
   amendment section (renamed amendment-until-terminal, carrying items 5 and 8), and its
   notes append-only bullet; the four flip surfaces named in item 7; the three plan-freeze
   surfaces whose co-flip prose item 7 retimes (the writing-plans skill's plan-lifecycle
   section, the plans guide, and the plans template, which say the plan freezes in the
   implementation's final commit); this project's
   agent-guide append-only bullet and the adr-system domain narrative; the glossary term
   `State changes`, whose opening word `frozen` item 5 falsifies for unapplied operations;
   and the pitfalls entry and deferred-roadmap item named in Consequences. The rendered
   prose says amendable until terminal; this record's title keeps the shorter
   until-Implemented phrasing it was authored under.
10. The change is purely additive: no corpus migration and no schema-generation bump. Every
    existing record, and any new record that is never amended, carries equal stamps and is
    valid unchanged under the chain rule; legacy non-terminal stamps become ordinary
    snapshot history rather than a validated freeze assertion. Residual cost accepted: the
    binary-version gate keys off the schema generation and the lock's `awfVersion`, and an
    amendment neither re-renders nor bumps the lock, so an older binary can meet an
    Amended-carrying record and fail with a malformed-history parse error rather than a
    version diagnostic; the lock advances at the next render, and adopters that never amend
    see no difference.

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
- An amendment while Accepted or Implementing can invalidate artifacts already derived from
  the record: a plan mid-execution, a dispatched implementation brief quoting a Decision
  item, or a worktree branched before the amendment. Item 8's routing and the existing
  plan-resync step are the mitigation: a decision-material amendment is deliberate and
  re-reconciled, never silent.
- The chain rule plus item 4 close most of the deferred-roadmap hole "A frozen-state ADR
  flip can smuggle unreviewed section content": a flip commit that also mutates content now
  fails validation everywhere except the record's first stamp (a direct flip out of
  Proposed), which still verifies content only at append time. The deferred audit-rule idea
  narrows to that residual case and is updated in the same change.
- Commit choreography changes at the end of implementation: the record is Implementing
  while terminal review runs, and the final Applied batch plus Implemented flip form the
  post-review commit. The generated decision index lists the record as in flight until
  then.
- A heavily amended record accumulates history lines; the routing rule in Decision item 8
  keeps decision-changing amendments deliberate.
- Two stale coverage annotations in `validateV2History` are corrected: the
  unknown-event-kind catch-all premised on exactly two event kinds gains the Amended branch
  before it, and the explicit-Implemented guard premised on no intervening event after the
  final Applied becomes a reachable, test-covered branch under item 2's rule.
- Records that use Amended events require a current awf binary to parse; item 10 states the
  accepted residual failure mode for an older binary meeting such a record.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep freeze-after-Proposed | Every post-freeze correction spawns a successor ADR; the corpus shows chains of three. |
| Append-only `## Amendments` section adding prose instead of editing the body | Splits one decision's current meaning across sections, forcing every reader to reconstruct it, and the digest would still need a chain to stay valid. |
| Drop non-terminal digests, tolerate legacy stamps unvalidated | Loses in-record amendment provenance and leaves a permanent unvalidated-legacy carve-out in the grammar. |
| Strip non-terminal digests via an upgrade migration | Mutates retained history events and costs migration machinery plus a schema-generation bump for no semantic gain. |
| Re-stamp non-terminal digests on each amendment | Routinely mutates retained events; a stamp stops recording anything true about its date. |

## Status history

- 2026-07-30: Proposed
