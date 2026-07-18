---
status: Proposed
date: 2026-07-18
tags: [adr-lifecycle, adr-parsing, invariant-retirement, invariant-backing, active-md]
related: [8, 9, 15, 16, 105, 116, 119, 120, 121, 128, 129, 130]
domains: [adr-system, invariants, rendering]
---
# ADR-0131: Complete and Self-Enforcing Supersession Records

## Context

ADR-0120 made partial supersession machine-checked, ADR-0128 derived full supersession from
anchor coverage, and ADR-0129 unified the model. What none of them established is that the
encoded record is *complete*. A corpus audit on 2026-07-18, covering all 133 ADRs, measured the
gap.

**The hand-correction pass ADR-0128 promised never ran.** ADR-0128's generation-12 migration
rewrote every pre-existing inline item token to `refines:`, deliberately: "Classification cannot
be mechanised, so the migration maps the ambiguous old token to the reading that asserts less"
(ADR-0128 Decision 4). It predicted 13 of the 37 were genuine retirements and closed with
"Correcting a downgraded retirement afterwards is an ordinary authoring edit, not a migration
concern." That correction ran for exactly one token, ADR-0128's own `supersedes: ADR-0120#3`.
Today all 13 `supersedes:` item tokens in the corpus are migration bookkeeping (ADR-0003 x3,
ADR-0031 x5, ADR-0113 x4, plus that one), and **zero** of the 37 pre-existing tokens assert a
retirement. The audit re-classified them by reading both sides of each token and found 8
confident retirements among them, consistent with ADR-0128's own estimate.

**Roughly 40 supersession claims are stated in prose and never encoded.** They share one
signature: the carrier ADR states the override in its Decision section, often naming the exact
item number, and writes no token. ADR-0047 Decision 1 reads "supersedes ADR-0040 Decision item 1"
in prose alone. ADR-0075 Decision 4 and ADR-0087 Decision 1 each claim the override is "recorded
via `related`" when the `related:` back-pointer is also absent. ADR-0120 Decision 11 committed to
hand-tokenizing "the freeform partial-supersession citations across the corpus (0105, 0119, 0081,
0108, 0020, and the rest of a grep sweep)"; the sweep caught the ADRs it named and every slug
retirement, and missed the item-level claims in the 0001-0109 range. The 0113-0130 chains are
clean, so this is historical residue, not an ongoing practice failure.

**Two invariants are green while their proofs assert their negation.** ADR-0009 declares
`invariant: config-root` ("Config loads from `.claude/awf/config.yaml`") and
`invariant: parts-convention` (a four-tier precedence including `explicit replaceWith`). ADR-0016
relocated the config root to `.awf/`; ADR-0015 deleted the `replaceWith` field. Both slugs are
still owed backing, and what backs `config-root` is `internal/config/config_test.go:119`, a test
carrying both `// invariant: config-root` and `// invariant: awf-config-root` whose body writes a
`.claude/awf.yaml` decoy and asserts `Load` ignores it. The proof demonstrates the declaration is
false.

This is not an oversight. ADR-0016 Decision 7 chose it: "Both predecessors keep their
`Implemented` status ... and their backing tests (`config-root`, ...) update in the same commit."
ADR-0015's Consequences say the same for `parts-convention`. **The mechanism they chose was never
available.** An invariant's declaration is prose in a frozen ADR body; ADR-0116 Decision 2 (as
narrowed by ADR-0120 Decision 9) freezes that body's meaning once the ADR leaves `Proposed`. So
"amend the invariant and update its test" can only ever move the test, leaving a frozen
declaration contradicted by its own proof. Retire-and-redeclare is the only coherent option under
append-only, and it is what ADR-0121 and ADR-0125 do correctly. ADR-0015 and ADR-0016 predate the
token mechanism and reached for an amendment that the lifecycle does not offer.

**The relation grammar cannot express every claim authors make.** `internal/adr/adr.go:71-85`
defines three patterns: `supersedes:` (retires an item), `refines:` (adapts an item, items only),
and `supersedes-invariant:` (retires a slug). There is no refinement form for a slug. ADR-0016
Decision 7 says it "narrows ADR-0015 `inv: provenance-banner` only insofar as the banner text now
names `.awf/`"; the only legal token there asserts full retirement, which is false. ADR-0128
Decision 2 reasoned that "a slug is atomic, so narrowing one means retiring it and re-declaring a
successor", and that holds for a slug whose *meaning* narrows. It does not cover a slug whose
declaration is merely re-scoped by a successor while remaining in force, which is what ADR-0016
describes.

**Citation spelling is not uniform.** ADR-0105 unified the *declaration* token to `invariant:`,
but prose citations were untouched: 87 `` `inv: <slug>` `` citations survive across 34 ADRs
alongside the `` `invariant: <slug>` `` spelling. A detector keyed to one spelling sees roughly
half the corpus. Rarer shapes exist too: ADR-0015 Decision 6 cites "ADR-0001 Invariants bullet 1",
an ordinal reference to an invariant that carries no slug at all, and ADR-0001 declares no slugs.

**A detector is the expected next step, and the evidence it waited for now exists.** ADR-0116
Decision 5 declined a mechanical check "for now", and its Alternatives table records the
verb-anchored advisory rule as "the strongest rejected option, and not refuted ... Explicitly the
expected next step", set aside because "the procedure has never been tried, so building the check
now would permanently confound what the prose alone can achieve." The procedure has since been
tried for a full release cycle. This audit is the measurement: the prose procedure holds for new
work (0113-0130 clean) and did not retroactively repair the residue it inherited.

## Decision

1. **Retire `config-root` and `parts-convention` retroactively, and redeclare nothing.**
   `supersedes-invariant: ADR-0009#config-root` and
   `supersedes-invariant: ADR-0009#parts-convention`. Both already have live successors declared
   by the ADRs that displaced them: ADR-0016's `awf-config-root` and ADR-0015's `no-replacewith`.
   The retirement drops the stale slugs from owed backing, and the duplicated proof marker on
   `internal/config/config_test.go:119` is deleted so that test backs `awf-config-root` alone.

   This reverses the keep-alive clause of ADR-0016 Decision 7 (`refines: ADR-0016#7`) and of
   ADR-0015 Decision 6 (`refines: ADR-0015#6`). Both items survive: their substantive override
   claims, path relocations, and precedence collapse all stand. Only their chosen bookkeeping
   mechanism, which the append-only rule made unavailable, is replaced.

2. **A slug gains a refinement relation: `refines-invariant: ADR-NNNN#<slug>`.** It records that
   a successor re-scopes a declared invariant that remains in force, and counts toward nothing,
   exactly as `refines:` does for an item. `internal/adr/adr.go` gains the pattern, `Relation`
   gains no new constant (the existing `Refines` value carries it, keyed by token name), and
   `coverage.go` excludes it from coverage on the same branch that excludes `refines:`.

   This narrows ADR-0128 Decision 2's "`supersedes-invariant:` gains no sibling"
   (`refines: ADR-0128#2`). That item's reasoning holds for a slug whose meaning narrows, which
   still owes retire-and-redeclare; it did not consider a slug re-scoped by a successor while
   remaining in force. The atomicity argument is preserved: a `refines-invariant:` claim never
   changes what the target slug asserts, which is why it can count toward nothing and leave the
   declaration binding.

3. **`awf check` reports a supersession claim stated in prose and not encoded.** The check is
   verb-anchored and scoped to `## Decision`: it fires when an override verb (`supersedes`,
   `overrides`, `replaces`, `reverses`, `amends`, `revises`, `narrows`, `generalizes`) occurs in
   the same Decision item as a citation of another ADR's anchor, and that Decision item carries no
   relation token for that anchor. It names the carrier, the carrier's item, the cited anchor, and
   the token shapes that would satisfy it.

   Citations are recognized in both anchor kinds and both spellings: `ADR-NNNN Decision item N`
   for items, and `` `inv: <slug>` `` or `` `invariant: <slug>` `` adjacent to an `ADR-NNNN`
   reference for slugs. Section scoping disambiguates the slug spellings for free: the same string
   inside `## Invariants` is a declaration, already parsed by `declRe`, and is never a citation.

4. **The check is unconditional and data-driven; it ships behind no config key.**
   `internal/project/check.go` runs every check for every adopter, and AGENTS.md's "`awf check` is
   the drift oracle" invariant means the same thing in every tree. A gating key would make "check
   is clean" adopter-relative for the first time. Instead the check reports nothing when no ADR
   carries an unencoded claim, so an adopter with a clean corpus never sees it and an adopter with
   residue gets findings without opting in.

   This deliberately declines the `proseGate` shape (ADR-0119 Decision 7). That gate is opt-in
   because it scans every tracked file, including prose awf does not own; this check reads only
   `docsDir/decisions`, whose grammar awf defines.

5. **`cites: ADR-NNNN#<anchor>` declares a citation informational.** It joins the inline token
   family as an inert relation: recognized only inside `## Decision`, counting toward nothing,
   surfaced nowhere. It exists because a Decision item can legitimately mention another ADR's
   anchor without claiming it. ADR-0116 Decision 3 names the case (ADR-0079 "cites ADR-0065
   Decision 4 informationally while amending Decision 3"), and third-party narration is another:
   ADR-0058 recounts that "ADR-0057 narrowed ADR-0034 item 1" without itself claiming that anchor.

   A comment-shaped marker was rejected for this. ADR-0121's `<!-- awf:comment -->` is a
   whole-line directive stripped at ingestion, so it cannot mark a mid-sentence citation, and
   ADR-0105's `touches-invariant:` is scoped to source and test markers. Reusing the ADR-0120
   token family keeps one grammar and lets the corpus view carry the suppression as parsed data
   rather than an out-of-band regex.

6. **Four citation classes are exempt by construction, not by marker.** The check never demands a
   token when the cited target is `Proposed` (ADR-0120 Decision 2 forbids the token outright, so
   demanding one would red a second check), when the citation is a self-citation (already a
   `GraphFaults` report), when the cited invariant bullet carries no slug (ADR-0001's bullets are
   unslugged and therefore unanchorable in any grammar this project has), or when the citation
   falls outside `## Decision`.

   The last class is a permanent hole and is recorded as such. ADR-0120 Decision 1 recognizes
   tokens only inside `## Decision`, so a Context-section citation such as ADR-0034's "This ADR
   refines, and does not replace, ADR-0015 Decision item 4" cannot be encoded where it sits, and
   moving it is a content edit the append-only rule forbids. Five ADRs carry this shape. They stay
   untokenized; the alternative, appending a bookkeeping item under ADR-0120 Decision 9's shape 2,
   is available to an author who judges a specific case worth it, and is not required here.

7. **This ADR does not extend ADR-0120 Decision 9's carve-out.** Correcting a `refines:` token to
   `supersedes:` needs no new permission: ADR-0128 Decision 4 already classifies it as "an
   ordinary authoring edit, not a migration concern", because the migration that downgraded it was
   the meaning-altering event and the correction restores the meaning its carrier's prose always
   stated. Inserting a token beside an existing prose citation is carve-out shape 3, already
   permitted. The carve-out is cited, not widened.

8. **Citation extraction is a method on the corpus view.** ADR-0130's `corpus-owns-field-reads`
   forbids any file outside `internal/adr` from reading `ADR.Refs` or `Sections`, and that
   invariant is currently clean. Extraction therefore lands in `internal/adr` as a `Corpus`
   method, and `internal/project` consumes parsed citations only. The check itself lands in a new
   `internal/project/citations.go` rather than growing the 254-line `supersession.go`.

9. **This repo completes its own retrofit before the check ships.** The 8 relation corrections,
   the roughly 40 backfilled tokens, the two retirements of item 1, and the three missing
   `related:` back-pointers (ADR-0069 lacks 75, ADR-0045 lacks 87, ADR-0082 lacks 85) all land
   before item 3's check is enabled, so it ships green. The corrections land separately from the
   bulk backfill: flipping `refines:` to `supersedes:` can complete an anchor's coverage and force
   a status flip, which is a different concern from inserting a token beside prose that already
   states the claim.

## Invariants

- `` `invariant: citation-check-decision-scoped` ``: the citation check considers only text
  inside a `## Decision` section; an override verb and an anchor citation together in Context,
  Consequences, Alternatives Considered, or Invariants never produce a finding.
- `` `invariant: citation-check-both-slug-spellings` ``: a slug citation is recognized in both the
  `` `inv: <slug>` `` and `` `invariant: <slug>` `` spellings, and a slug declaration inside
  `## Invariants` is never treated as a citation.
- `` `invariant: citation-check-exempts-unanchorable` ``: the check produces no finding for a
  citation whose target is `Proposed`, for a self-citation, or for a cited invariant bullet that
  declares no slug.
- `` `invariant: cites-token-inert` ``: a `cites:` token suppresses the citation check for the
  anchor it names, contributes nothing to anchor coverage, and appears in no ACTIVE.md or
  `awf context` supersedence rendering.
- `` `invariant: refines-invariant-counts-nothing` ``: a `refines-invariant:` claim never counts
  toward its target's anchor coverage, so an invariant that has only ever been refined cannot
  complete its declaring ADR's coverage.
- `` `invariant: citation-extraction-owned-by-corpus` ``: prose-citation extraction is reachable
  only as a method on `internal/adr`'s corpus view; no file outside `internal/adr` reads section
  bodies to find citations.
- `` `unbacked-invariant: unslugged-invariant-bullets-unanchorable` ``: an invariant bullet
  declared without a slug cannot be named by any relation token, so a prose citation of one is
  permanently unencodable rather than merely unencoded. **Verify:** read ADR-0001's `## Invariants`
  section, which declares four bullets and no slugs, then confirm `internal/adr`'s anchor grammar
  (`Anchor` in `coverage.go`) addresses a slug anchor only by slug string, with no ordinal form.

## Consequences

The supersession record becomes complete for the first time: after item 9's retrofit, every
override an ADR states in its Decision section is encoded, and item 3 keeps it that way. ACTIVE.md
and `awf context` annotations become trustworthy as a *complete* account of what no longer binds,
rather than a lower bound on it.

Two invariants stop being enforced, and that is a reduction in real coverage, not a cleanup of
dead weight. `config-root` and `parts-convention` describe properties nobody wants violated. What
item 1 removes is the false claim that the corpus was checking them; their live successors
(`awf-config-root`, `no-replacewith`) are what actually hold today, and the audit confirmed both
are backed.

The check's recall is bounded by its trigger, and the bound is known. The audit found roughly 40
owed tokens by reading; a verb-anchored, Decision-scoped trigger covers roughly 20 of them. The
remainder are claims whose prose carries no listed verb, such as ADR-0060 Decision 5's "every
invariant listed in ADR-0043/0027 keeps its current wording". Item 9's backfill fixes those by
hand; item 3 will not prevent their recurrence. This is an accepted precision-over-recall trade:
a check that fires on every ADR cross-reference would be disabled within a release, and ADR-0116
already measured the cite-a-Decision-item trigger as materially less precise. The gap is recorded
here so a future observer does not read a green check as proof of completeness.

`cites:` is new vocabulary an adopter must learn, and its absence is silent: an author who omits
it on a genuinely informational citation gets a finding and may encode a claim that was never
meant, recording a supersession that did not happen. The check's message names `cites:` as one of
the satisfying shapes to make the choice explicit at the point of failure.

Correcting the 8 relations changes derived state. Each correction moves an anchor from
uncontested to retired, and coverage completion forces a predecessor's status to `Superseded`
(ADR-0128 Decision 4). The audit confirmed no ADR is within reach of full coverage from these 8
alone (the nearest live ADR, ADR-0001, remains 2 anchors short after its correction), so no
derived death lands with this ADR. That headroom is measured, not structural, and a future
correction may complete a coverage set.

Adopters upgrading past this release may see findings on their own corpora with no way to defer
them, which is the cost of item 4's refusal of a config key. The findings name exact edits, and
the token insertions they ask for are permitted by ADR-0120 Decision 9 shape 3 without reopening
any ADR.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Opt-in config key, mirroring `proseGate` | The shape fits the migration problem but breaks the drift oracle: no check in `internal/project/check.go` is config-gated today, and a key makes "`awf check` is clean" mean different things in different trees. Item 4's data-driven silence gets the same adopter experience without that cost. |
| Advisory `repoaudit` rule, as ADR-0116 sketched | Range-scoped and needing no backfill, but repo-local: adopters get no mechanism at all, and the residue this ADR measures is exactly what an ignorable channel failed to prevent. |
| Cite-anchored trigger (any anchor citation owes a token or a `cites:`) | Closes the recall gap item 3 accepts, at the price of marking every informational citation corpus-wide. ADR-0116 already judged this trigger materially less precise, and a gate that fires on ordinary cross-references gets disabled. |
| Amend ADR-0009's invariant prose in place | Forbidden by the append-only rule, and it is precisely the unavailable mechanism ADR-0015 and ADR-0016 reached for. Retire-and-redeclare is the lifecycle's answer. |
| Leave `config-root` and `parts-convention` alone | Preserves two prior decisions verbatim at the cost of leaving two invariants green whose proofs assert their negation. A check asserting something untrue is worse than one fewer check. |
| Extend ADR-0120 Decision 9's carve-out for relation corrections | Unnecessary: ADR-0128 Decision 4 already classifies the correction as ordinary authoring. Widening an exhaustive carve-out without need weakens the append-only rule. |
| One ADR per workstream (retirements, grammar, detector) | They share one commitment and one failure mode; the detector needs `refines-invariant:` to offer a truthful remedy for slug citations, and the retirements are the motivating instance. Splitting would spread one decision across three review cycles. |
