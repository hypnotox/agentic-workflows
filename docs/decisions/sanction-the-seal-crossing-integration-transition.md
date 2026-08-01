---
format: current-state-v3
slug: sanction-the-seal-crossing-integration-transition
status: Proposed
date: 2026-08-01
---
# ADR-sanction-the-seal-crossing-integration-transition: Sanction the seal-crossing integration transition

## Context

Three decisions in one lineage made integration the moment an optimistically allocated
value is settled. ADR-0202 made an ADR number provisional until integration. ADR-0203 made
a lock cutoff and a schema generation provisional the same way, adding a re-derivation edge
to permanent lock authority. ADR-0204 made a slugless record pair across a renumber by
content digest, so the rename survives checking.

On 2026-08-01 an effort branch exercised the case none of the three anticipated: a branch
forked before the sealing generation merging an integration branch already past it. The
branch carried one governed record whose number the integration had taken. The numbering
was dense through 0202, `adrFormatV3From` was 203, and `legacyAdrGaps` was empty, so the
only free number lay at or above the cutoff. A record living there must take the
`current-state-v3` encoding, because the cutoff decides a record's format from its number
and a v2 record at or above it is refused for want of a mandatory `slug:` key. The renumber
therefore forced a format retrofit, and three enforcement rules refused that retrofit:

1. `validatePermanentLockTransition` refused the lock transition, because the branch
   carried no `adrFormatV3From` at all and the value arriving from the other parent was
   computed against a corpus neither tree holds.
2. `renumberAliases` refused to pair the two ends, because the digest step considered only
   a record carrying no slug and the renumbered record acquires its slug in the very
   transition that moves it.
3. The governed-format-change rule refused the v2-to-v3 change as an independent edit.

All three were relaxed inline so the integration could land. That is the first thing this
record has to answer for, because of how it landed.

### The relaxations were never a reviewable commit

All three entered as an evil merge. The code is absent from both parents of `728f6695` and
was added in the merge itself, so no commit ever presented it for review as a change. The
first version of the pairing relaxation admitted any record whose slug was new on its own
side, and terminal review found that it accepted four transitions the rule had previously
refused: a numbered record losing its number, a retained v3 slug changed at one number, a
frozen-content amendment escaping through an unrelated pending twin, and a genuine deletion
laundered into a rename. The last of these is exactly the fail-closed promise ADR-0204 item
4 makes explicitly. `1cda10f8` narrowed it afterwards. The gate was green throughout, in
both versions. Review, not mechanism, was the only thing between the corpus and a rule that
read correctly and enforced almost nothing.

### Two premises recorded at the time are wrong

Investigation for this record falsified two claims made when the relaxations landed, and
both are corrected forward here rather than restated.

The first is that the inherited cutoff is taken on trust. It is not.
`validatePermanentLockTransition` admits at `internal/project/currentstate.go:181`, and
`CheckStaged` then reparses the entire staged corpus under the admitted cutoff at
`:189-190`. A wrong cutoff dies there in both directions: too low and a sealed v3 record
reparses as v2 and fails the marker equality, too high and a v3 record's `slug:` key meets
the narrow closed struct under `KnownFields(true)`. A sweep over the live corpus confirms
it: of 200, 202, 203, 204, 205, 206, and 210, only 203 survives. Simulating the inherited
edge itself with 202, 203, 204, and 210 showed the validator admitting all four and the
corpus load rejecting three. The guard is real. It is simply enforced downstream of where
it is admitted, by a load ordering nothing declares, and it reports as an unrelated YAML
unknown-field error rather than as a lock-authority violation.

The second is the account of why one clause of
`adr-system/adr-lifecycle:renumber-digest-paired` is false. The clause says the digest step
re-keys only where the two ends hold different numbers, and it was recorded as never having
been true of the code. Git archaeology refutes that. As ADR-0204 shipped at `028b5c64`, the
index was `uniqueSluglessDigests` and it skipped every slug-carrying record on both sides,
so every indexed record was numbered and slugless, both sides keyed on their numbers, and
the comparison was genuine. The clause was true when it was written. This work's widening
falsified it, because a numbered after-side record whose slug is new now keys on that slug,
and a slug never equals a four-digit number, so the re-key fires whether or not the numbers
moved.

### The rules were written one cutoff wide and one generation deep

The relaxations fixed the branch in front of them rather than the shape. Probing
`validatePermanentLockTransition` directly against an after-lock at generation 30 shows the
cost:

- The inherited edge is `before.SchemaVersion < 29 && after.SchemaVersion > 29`, strictly
  greater. A branch forked at generation 28 merging one at exactly 29 never reaches it: the
  V3 sealing edge is evaluated first, matches `before == 28 && after == 29`, recomputes from
  the before tree, and refuses on the mismatch. Widening the inherited edge alone therefore
  does not close this case, because an earlier edge intercepts it. The real merge escaped
  only because main had already reached generation 30. The regression test pins
  `SchemaVersion: 30` and never probes 29, so the gap is untested and was unnoticed.
- The same edge requires `before.ADRFormatV2From == after.ADRFormatV2From`. A branch forked
  before generation 15 carries `V2From: 0`, fails that clause, and is refused. There is an
  inherited edge for the V3 cutoff and none for V2 or V1.
- `isRenumberRetrofit` is `before.IsV2() && after.IsV3()`, with no v1-to-v2 clause, so a
  slugless record renumbered across cutoff 144 still trips the format-change rule.

Every one of these reproduces at the next seal. A fourth wall sits behind them in the
config port-forward: `retiredKeyRemovals` lists four retired keys and omits the top-level
`invariants` block retired at generation 14 and `audit.baseBranch` retired at generation 11.
`retiredKeyRemovals` is the only list the port-forward strips from, and
`ConfigForCurrentSchema`'s registry loop exists solely to seed `integrationBranch`, so
neither key is stripped on that path and both reach `config.ParseTree`'s `KnownFields(true)`
as a hard error rather than a finding.

One rule is out of scope here because it sits outside the lock model entirely: the proof
marker scan at `internal/topic/markers.go:230` hard-errors on an unnamed marker with no
generation pin at all, so a branch forked before the marker naming rule landed is refused
however current its schema generation, and no lock-transition edge can ever admit it. That
needs its own decision.

Finally, the governed-format-change rule at `internal/currentstate/transition.go:87-90` is
asserted by no current-state claim at all. Its relaxation is the one of the three that
falsified nothing, because nothing declared it.

## Decision

1. A branch forked before a sealing generation integrating across that seal is one
   sanctioned transition shape, not a collection of independent exceptions. Its two sides
   are a before authority that predates the seal and an after authority that arrives from
   the other merge parent already sealed. This item is the framing for the items below
   rather than a standing governance rule: it names what the following facets have in
   common, and it binds nothing that no claim below records.

2. The inherited-cutoff edge is written over the generation-sealed cutoffs rather than
   pinned to one generation and one cutoff. The edge admits a transition in which the schema
   generation advances across the seal of some cutoff the before authority did not carry,
   that cutoff takes the value published by the arriving authority, and every other
   permanent value stays byte-identical. It covers `adrFormatV2From`, sealed at generation
   15, and `adrFormatV3From`, sealed at generation 29, and every cutoff a later generation
   seals. It does not cover `adrFormatV1From`, which no generation seals: that cutoff is
   written at first adoption or by consuming a bridge attestation, its existing edge carries
   no generation clause, and it keeps that edge unchanged. The published value cannot be
   re-derived and must not be: the before tree yields the branch's own pre-merge next
   identity, and the after tree yields one the merge has already moved past, so re-deriving
   would lower the cutoff under records already sealed above it.

   The edge admits the case where the after generation equals the sealing generation as well
   as the case where it exceeds it, and closing the equal case requires more than widening
   this edge. The sealing edge for that cutoff is evaluated first and refuses on a value it
   cannot re-derive, so the two are made disjoint by their signal: a transition landing
   exactly on the sealing generation whose published value does not equal the before
   corpus's computed next identity falls through to this edge instead of refusing, because
   that mismatch is precisely the evidence the migration did not run here. The cost is that
   a locally sealed cutoff at the sealing generation is no longer required to equal the
   computed next identity exactly; what holds in its place is item 3's upper bound and the
   staged reparse below it, which together admit only a value the arriving corpus is
   consistent with.

3. The inherited value is bounded above by the staged merged tree's computed next identity,
   the value `nextADRIdentityFromTree(afterTree)` returns, which is derivable from what the
   validator already holds. A cutoff sealed at a value no record has reached is admissible
   only up to the number that corpus would next assign; beyond that the arriving authority
   is asserting a seal for records that do not exist. This bound is the one guard the
   downstream corpus reparse does not supply, because a corpus carrying no record at or
   above the cutoff routes nothing to the newer parser and so contradicts nothing. Below
   that bound the reparse remains the enforcement, and that dependence is carried by the
   updated `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable` text rather than by
   this record alone, so a later author reordering the staged load meets it in current state
   instead of in history.

4. The governed-format retrofit is sanctioned across any adjacent cutoff crossing, not the
   v2-to-v3 pair alone. A governed record renumbered from below a cutoff to at or above it
   takes the encoding its new number requires, and that format change is the renumber's
   cost rather than an independent edit. The clauses that keep it shut are unchanged in
   kind: an in-place format change keeps its number, a downgrade runs the other direction,
   and a jump across more than one cutoff is not this transition. The claim this item adds
   is `Backing: test`, proven by the retrofit regression test
   `TestCheckPairRenumberAcrossTheV3Cutoff` in `internal/currentstate/transition_test.go`,
   which item 7 gives a proof marker naming it.

5. The digest-pairing after-side widening is sanctioned as it currently stands. The before
   side considers only a record carrying no slug, so number immutability stays exactly as
   strong for a slug-carrying record as it was. The after side additionally considers a
   numbered record whose slug is new in the transition, which is what a record renumbered
   across a cutoff becomes when it takes the newer encoding. Requiring a number is what
   keeps the opening shut, because a pending record carries none and therefore can neither
   launder a deletion into a rename nor make a legitimate rename's digest ambiguous. No
   predicate changes; this item supplies the authority the rule has been running without.

6. The config port-forward strips every retired key, not the subset a maintainer
   remembered. The top-level `invariants` block and `audit.baseBranch` join
   `retiredKeyRemovals` and the `retiredConfigKeys` backstop. The maintenance obligation is
   restated as covering every key the current schema no longer declares, whether or not its
   removing migration carries a config-bytes action, in both places that record it: the
   comment beside the backstop in `internal/migrate/forwardport_test.go`, and the prose in
   `docs/pitfalls.md`, whose `.awf/` source is edited and re-rendered rather than the
   generated file. The rule becomes a governed claim rather than an obligation recorded only
   in a comment: `Backing: test`, proven by `TestConfigForCurrentSchemaParsesEveryRetiredKey`.
   Leaving it ungoverned would ship a fourth instance of exactly the defect item 4 exists to
   close, in the record that diagnoses it.

7. The corrections this work owes travel with it. Three doc comments are rewritten, each
   carrying a statement its own code has outgrown: `validatePermanentLockTransition`'s,
   which still says the sole non-identical edge while the body admits several;
   `renumberAliases`', which repeats both wrong clauses of the pairing claim; and
   `isRenumberRetrofit`'s, which calls the retrofit the one sanctioned governed-format
   change into the v3 range, which item 4 generalises. The four regression tests covering
   these rules carry proof markers naming the units that prove them, since they carry none
   today. The five citations in `internal/topic/markers.go` that name ADR-0199 for the
   proof-marker naming rule are corrected to ADR-0205: they are un-substituted residue of
   this very renumber, which substitutes into `Origin:` and `Revised-by:` only, so no check
   sees a citation in a code comment. The roadmap entry this record discharges is replaced
   in the same commit by the two items it defers, editing
   `.awf/docs/parts/roadmap/deferred.md` and re-rendering rather than hand-editing the
   generated `docs/roadmap.md`; the discharged entry states both premises this record
   falsifies and must not outlive it.

## State changes

- update `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`
- add `config/migrations-and-locks:port-forward-strips-every-retired-key`
- update `adr-system/adr-lifecycle:renumber-digest-paired`
- add `adr-system/adr-lifecycle:governed-format-change-bounded`

## Consequences

Integration by a stale branch stops being a bespoke event. The generalisation over the
cutoff set means the next seal does not reproduce this record: a branch forked before it
integrates on the edge already written, and the untested equal-generation case is closed
rather than left to the accident that main had moved one generation further.

The record supersedes a current-state claim ADR-0204 item 10 established. That item says a
renumber target must stay below the V3 cutoff, and that a target at or above it fails for
want of a slug or trips the changed-format rule. Item 4 here sanctions exactly that target.
ADR-0204 is Implemented and is history rather than active authority, so nothing is edited
there; the claim it established changes by the operations above.

`config/migrations-and-locks:adr-v2-cutoff-atomic-immutable` opens by saying each format
cutoff is sealed by its own schema generation. Item 2 establishes that `adrFormatV1From` is
not: it is written at adoption or by consuming a bridge attestation. The updated text has
to say which cutoffs a generation seals rather than asserting it of all of them, alongside
the third edge and the reparse dependence item 3 assigns to it.

`adr-system/adr-lifecycle:renumber-digest-paired` is false in three clauses, not the two
recorded on the roadmap. Beyond the two already named, it ends by saying such a pair admits
the number, filename, and heading change and nothing else, while the sanctioned retrofit
also changes `format:` and adds a `slug:` key. The corrected text has to state all three,
and its backing has to move: its eight existing proof markers sit on tests that predate the
widening and none of them exercises the widened admission or the retrofit.

Both added claims close the same gap, and it is the gap that made this whole class harder to
see. An enforcement rule asserted by no claim can be relaxed without falsifying anything, so
nothing flagged the third relaxation, and the roadmap's own accounting named two false
sentences rather than three. The port-forward rule was the same shape one layer down: its
obligation lived in a test comment and a pitfalls paragraph, neither of which any check
reads as authority. Two ungoverned rules surfacing in one record is the argument for asking,
separately, whether an enforcement rule should be mechanically prevented from shipping
without a governing claim.

The cost of the generalisation is that two refusal predicates now accept strictly more, at
every generation-sealed cutoff rather than one. That is the same shape of risk as the rule
this record indicts for reading correctly and enforcing almost nothing, and it deserves
naming rather than burying. Four things hold it shut. Every permanent value other than the
inheriting cutoff stays byte-identical, so nothing else moves under cover of the edge. Item
3's upper bound refuses a cutoff sealed past what the arriving corpus could reach. The
retrofit admits only an adjacent cutoff crossing, so a record cannot skip a format. And the
after-side digest index still requires a number, which is what keeps a pending record from
laundering a deletion into a rename. Item 2's disjointness clause is the one place the net
loosens without a compensating clause of its own, and it is bounded to transitions landing
exactly on a sealing generation.

What this record does not fix is the reason it was needed. Both versions of the pairing
relaxation passed the gate, and only terminal review distinguished them. The evil-merge
shape is the aggravating factor: code that appears in neither parent of a merge is never
presented as a change, so the review that catches it is reviewing a diff nobody wrote. The
proof markers item 7 adds are a partial answer, since a marker whose test is deleted or
renamed fails the check, but they do not make a too-broad predicate fail. That remains
open.

Two items move to the roadmap rather than here. The ungoverned proof-marker scan sits
outside the lock model and needs its own record. Whether a rule of this kind should be
mechanically prevented from shipping without a governing claim is a broader question than
this transition.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave the three relaxations as shipped and only correct the false claims | Cheapest, and it was the initial scope. It leaves two current-state sentences correct but the code refusing a branch forked at generation 28 merging one at exactly 29, and reproduces the whole class at the next seal. |
| Three independent edges, each justified on its own terms | Matches the existing claim topology, which spans two domains. It gives a future reader no single place to look for what happens when a branch is stale, and it was the framing that produced three separately-scoped relaxations in the first place. |
| Declare a lock-versus-corpus coherence check as the guard | Investigated and largely rejected. The derived admissible range is mathematically identical to the reparse that already runs, and the contiguity rule refusing a recorded gap at or above a cutoff means the range always collapses to a single value above `adrFormatV1From`. Only the upper bound survives, as item 3. |
| Verify the inherited cutoff against the other merge parent's lock | Would confirm the published value exactly. It requires reading git provenance during lock validation, which the numbering rules deliberately avoid, and `validatePermanentLockTransition` has no merge signal at that point. |
| Retire the cutoff-forced retrofit by reserving numbers below the cutoff | Removes the forcing condition rather than sanctioning its consequence. It requires holding numbers free against a future stale branch, which trades a rare integration cost for a permanent numbering distortion. |

## Status history

- 2026-08-01: Proposed
