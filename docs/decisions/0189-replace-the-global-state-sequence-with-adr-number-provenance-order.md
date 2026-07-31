---
format: current-state-v2
status: Implementing
date: 2026-07-31
---
# ADR-0189: Replace the global state sequence with ADR-number provenance order

## Context

Every applied claim-operation batch carries `state-sequence: <positive integer>` in its ADR's
Status history, and those integers form one repository-global namespace that must stay unique and
contiguous from 1. ADR-0135 introduced the number on terminal events, ADR-0143 generalized it to
per-batch inheritance and made it the order behind `Revised-by` and claim-history output, and
ADR-0182 extended it across merge aggregates by requiring appended batches to continue the
contiguous run. `internal/currentstate/check.go` enforces uniqueness, contiguity, and
increasing-sequence `Revised-by` order statically; `internal/currentstate/transition.go` requires
each new batch to be exactly the next integer.

The global namespace has no allocator and cannot have a stable one. The author hand-writes the next
integer, and the documented working method is to provoke a check error to learn it, because the
correct value moves whenever any sibling ADR applies a batch. That is the direct coupling this
decision removes: two efforts that touch disjoint claims still contend for the same counter, and
the one that integrates second must rewrite already-applied history lines. ADR-0182's own
Consequences name renumber-before-integration as the normal path for concurrent efforts, and the
in-flight merge-time ADR-numbering design (slug-identified pending ADRs, final numbers assigned by
`awf adr number` at integration) was forced to plan sequence shifting at integration plus a
dedicated transition-check mode, purely because shifting collides with
`adr-system/adr-lifecycle:applied-history-events-append-only`.

The requirement, settled in the remove-global-state-sequence effort, is a mutation model that is
rigid but never blocks completion: independently developed claim mutations must merge in any
integration order without coordinating on a global counter.

A replacement order already exists. Final ADR numbers are unique across the corpus, and under the
merge-time numbering contract they are assigned at integration in merge order and never change
afterward, so ascending final ADR number is per-claim integration chronology once that contract
lands. Until it lands, numbers are still unique at integration (today via manual renumbering of
colliding proposals), so ascending ADR number is deterministic even where it is not chronological.

Existing history is not fully ordered by ADR number. Two claims record ADR-0166 after ADR-0167
because application sequence 77-78 followed sequence 68:
`rendering/pi-workflows:pi-session-handoff-workflow` and
`rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`. These are the only two
inversions in the corpus, they are one integration apart, and current claim text, not provenance
order, is the authority readers consume.

## Decision

1. The repository-global state-sequence namespace is removed. No status-history event carries a
   `state-sequence:` segment, and no uniqueness, contiguity, or expected-next-sequence validation
   exists anywhere. The parser tolerates and discards the retired segment rather than rejecting
   it, so a pre-migration corpus still loads, and `awf check` reports every surviving segment as
   a blocking finding directing to `awf upgrade`: loudness moves from parse time to check time,
   and nothing silently passes.

2. Per-claim provenance order is ascending final ADR number. A claim's canonical chain is its
   Origin ADR followed by its `Revised-by` ADRs sorted ascending and duplicate-free, and every
   `Revised-by` entry is greater than the Origin's number: an update targets a claim whose add has
   already integrated, so the rule holds in both the pre-contract and post-contract windows, and it
   holds across today's corpus. Claim history output sorts revision records the same way. Under the
   merge-time numbering contract this order is integration chronology; until that contract lands it
   is the canonical deterministic presentation and asserts no chronology.

3. Inside one ADR the authored record stays authoritative: Applied events are ordered by their
   position in the append-only Status history, each batch's operations keep declaration order, and
   operations are not required to be sorted by claim identity. No cross-ADR order is derived from
   batch positions.

4. An update to an active claim adds its ADR number to `Revised-by` exactly once, at its canonical
   ascending position. Appending is the common case; inserting before a higher number is legal.

5. Provenance unions commutatively across a merge: the after-side `Revised-by` is the
   duplicate-free ascending union of the before-side list and the chain's updating ADRs. Concurrent
   updates to one active claim may integrate in either order when the surviving claim text is the
   reviewed reconciliation citing both; ADR-0182's net-substance rule keeps applying.

6. An applied remove is an absorbing tombstone. The qualified id is currently absent from the
   moment the remove applies, and a removed id is still never reused. A concurrently developed
   update that integrates after the remove is dominated: its Applied event and operation are
   retained as history, it establishes no current claim and requires no current claim to exist, and
   absence stays attributed to the remove. Update-then-remove and remove-then-dominated-update
   converge to the same current state, absent with full history retained.

7. Merge-aggregate chain validation is restated without the sequence, and the remove is absorbing
   in the chain grammar itself, not merely trailing. A claim's operations across the pair, taken in
   ascending ADR-number order and intra-ADR history order, admit at most one leading add, any
   number of updates, at most one remove, and after the remove any number of dominated updates. Net
   effect derives from the chain as in ADR-0182, extended by absorption: a chain containing a
   remove is a net remove (or, when it also begins with the add, a net no-op) whose claim must be
   absent on the after side regardless of dominated updates following the remove, and a chain of
   updates against a claim already absent on the before side is legal dominated history with net
   effect none. The global contiguity condition is deleted. An authored non-merge commit keeps the
   stricter contract of one new batch per ADR, one operation per claim, and the fixed status-event
   shape.

8. Bidirectionality and atomicity are restated for domination: every applied governed operation
   has its current result, its removed result, or its dominated-history record, and every active
   claim Origin or revision has the inverse applied ADR operation. Dominated operations, like
   Remaining and Canceled ones, provide no authority. A dominated operation's required claim
   mutation set is empty, so the transaction-atomicity contract is satisfied by the batch record
   alone; the update's original mutation remains in its branch's own authored commit, where the
   per-commit contract already validated it.

9. One ordered schema migration retrofits the corpus: it strips every `state-sequence:` segment
   from status-history events and canonicalizes every `Revised-by` list to duplicate-free ascending
   ADR number. The two known inversions named in Context are deliberately reordered, not
   grandfathered; this ADR records that correction, and the original application chronology stays
   recoverable from git history. The retrofit is meaning-preserving at event level: each event
   keeps its date, status, digest, and operations, and only the retired ordering encoding is
   dropped, its meaning carried forward by ADR number. The migration rewrites raw ADR bytes and so
   becomes the third enumerated raw-bytes migration seam, extending the enumeration that
   `adr-system/adr-lifecycle:corpus-raw-access-enumerated` pins. Item 1's tolerant parse is what
   lets the migration load the corpus whose segments it strips; a strict parse here would refuse
   the very files the retrofit exists to fix.

10. Both added claims land as `Backing: test`, with proof markers on `internal/currentstate` tests:
    the provenance-order claim on fixtures proving ascending, duplicate-free, Origin-minimal
    `Revised-by` validation, and the tombstone claim on fixtures proving that remove-then-update
    and update-then-remove integration orders converge to attributed absence with the dominated
    batch retained. Each add lands with ADR-0189 as Origin; each updated claim preserves its Origin
    and prior revisions and gains ADR-0189 in `Revised-by` in the same transaction.

11. Documentation travels with the implementation, edited at the sources, never the rendered
    outputs. The same change that lands the behaviour rewrites the Applied grammar and
    next-sequence allocation instruction in `templates/skills/adr-lifecycle/SKILL.md.tmpl`, the
    grammar lines in `templates/adr-readme/README.md.tmpl` and
    `templates/adr-template/template.md.tmpl`, the sequence guidance in the plan-review focus of
    `.awf/agents/plan-reviewer.yaml`, the Applied grammar in
    `.awf/domains/parts/adr-system/current-state.md`, and every `.awf/docs/pitfalls.yaml` entry
    that mentions the state sequence, the provoked-error allocation and replay-before-integration
    entries included; the rendered outputs, `examples/sundial` copies included, follow from
    `./x render`. A Breaking-changes entry lands under the changelog's Unreleased section, and
    `docs/decisions/INDEX.md` is regenerated at every status flip this ADR reaches.

## State changes

- remove `invariants/current-state-authority:application-batch-sequence-order`
- add `invariants/current-state-authority:provenance-ordered-by-adr-number`
- add `invariants/current-state-authority:applied-remove-absorbing-tombstone`
- update `invariants/current-state-authority:merge-transition-ordered-aggregate`
- update `invariants/current-state-authority:implemented-impact-bidirectional`
- update `invariants/current-state-authority:update-requires-substance`
- update `invariants/current-state-authority:state-impact-transition-atomic`
- update `adr-system/adr-lifecycle:applied-history-events-append-only`
- update `adr-system/adr-lifecycle:corpus-raw-access-enumerated`

## Consequences

Independently developed mutations integrate without touching each other. Nothing in an applied
batch ever needs rewriting at integration, so the renumber-before-integration path that ADR-0182
left as the normal case disappears, `awf effort integrate`'s divergent path stops failing on
sequence collisions, and the merge-time numbering effort drops both its sequence-shifting step and
the transition-check mode it existed to legalize: any future numbering decision inherits a scope of
renaming files and rewriting identities only. Authoring gets simpler in the same stroke: there is
no next-number to discover, so the provoke-the-error allocation pitfall is deleted rather than
documented.

Agent-facing output loses a field. `awf topic` drops its `[state-sequence: N]` suffix on Origin,
Revised-by, and Removed-by lines, `awf context` drops its per-operation state-sequence annotations,
and the `stateSequence` field leaves the `awf topic --json` contract. The ordering signal remaining
in all three outputs is the ADR number already printed, which after this decision carries exactly
the information the sequence used to.

The corpus loses its global total order over applied operations. Per-claim order is fully defined
by ADR number; a cross-claim "which applied first" question has no in-corpus answer and falls back
to git history. That is a real narrowing, accepted deliberately: no validation rule and no reader
workflow consumes cross-claim order, only per-claim provenance.

Until the merge-time numbering contract lands, ascending ADR number is canonical but not guaranteed
chronological, and new inversions between authoring order and numbering order remain possible;
they are simply legal under item 2 instead of being check failures. The numbering contract closes
that gap at integration time. This ADR consumes that contract but does not depend on it to be
correct: uniqueness of final numbers is all items 2 through 9 require.

The absorbing tombstone accepts a quiet outcome: a reviewed, substantive concurrent update can end
up with no current effect because a removal absorbed it. The model makes that outcome deterministic
and fully recorded rather than order-dependent, and catching a genuinely wrong absorption is
explicitly delegated to human and agent review, per the effort's settled decision.

The migration rewrites every governed ADR's status-history lines and two `Revised-by` lists, bumps
the schema generation so the binary-version gate forces `awf upgrade`, and follows the existing
ordered-migration precedent for raw-bytes ADR retrofits. The new and updated claims are re-proved
at implementation time under their declared backing.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the global sequence and add an allocator command | Removes the discovery pitfall but keeps the shared counter, so parallel efforts still collide and rewrite applied history at integration. |
| Relax the namespace to unique-but-not-contiguous | Contiguity is what forces renumbering, but a shared counter without an allocator still collides: two parallel efforts optimistically hand-pick the same integer and one must rewrite applied history at integration, and the surviving number duplicates information the ADR number already carries. |
| Order provenance by the Applied event date, ADR number as tiebreak | Hand-authored dates give no trustworthy total order: same-day batches are the common case for parallel efforts, so the tiebreak would decide most orderings anyway, with a fabricated chronology layered on top. |
| Per-claim sequence counters | A second allocated namespace when ADR number already orders per-claim history; concurrent updates to one claim still contend for the counter. |
| Causal metadata (vector clocks or recorded happened-before) | Deterministic presentation needs no causality; heavy authoring friction for provenance whose authority is current claim text. |
| Grandfather the two historical inversions | Permanent legacy ordering complexity in grammar and checks for two adjacent entries nobody consumes as chronology. |
| Leave historical `state-sequence:` segments in place as inert text | The grammar and every reader carry a retired concept forever; contiguity would be frozen trivia that new tooling must still parse around. |
| Refuse an update whose target was concurrently removed | Blocks completion of finished, reviewed work at integration, violating the effort's core requirement. |
| Let a later-integrating update revive a removed claim | Current state would depend on integration order; removal stops being deterministic and absence stops being trustworthy. |

## Status history

- 2026-07-31: Proposed
- 2026-07-31: Accepted; content-sha256: 6c6dc3d3de5cdc640cf8e8329c50d0f76ec3f2711e18b202d4d5a38a9d3714b9
- 2026-07-31: Implementing; content-sha256: 6c6dc3d3de5cdc640cf8e8329c50d0f76ec3f2711e18b202d4d5a38a9d3714b9
- 2026-07-31: Applied; operations: remove `invariants/current-state-authority:application-batch-sequence-order`, add `invariants/current-state-authority:provenance-ordered-by-adr-number`, add `invariants/current-state-authority:applied-remove-absorbing-tombstone`, update `invariants/current-state-authority:merge-transition-ordered-aggregate`, update `invariants/current-state-authority:implemented-impact-bidirectional`, update `invariants/current-state-authority:state-impact-transition-atomic`, update `adr-system/adr-lifecycle:applied-history-events-append-only`, update `adr-system/adr-lifecycle:corpus-raw-access-enumerated`
