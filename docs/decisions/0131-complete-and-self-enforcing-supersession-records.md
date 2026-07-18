---
status: Proposed
date: 2026-07-18
tags: [adr-lifecycle, adr-parsing, invariant-retirement, invariant-backing, active-md]
related: [8, 9, 15, 16, 34, 57, 58, 65, 79, 82, 85, 105, 116, 119, 120, 121, 128, 129, 130]
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
retirement. Re-reading both sides of each token found 4 confident retirements against ADR-0128's
estimate of 13. The 9-token difference is residue the re-reading judged ambiguous: each has a
target item with at least one surviving clause, so each stays `refines:` under the rule that the
relation asserting less is correct when the reading is contested.

That rule did most of the work, and it reduced the count twice. ADR-0093 Decision 2 is headed
"Supersede ADR-0024 Decision items 1 and 6", which reads as a retirement of both, and its token
into item 6 is indeed corrected to `supersedes:` here; item 1 is not, because what ADR-0093
replaces is the command names, while item 1's requirement that a kind be given and its
`skill`-to-`skills` mapping survive the rename untouched. Two further candidates fell to review of
this ADR. ADR-0120's token into ADR-0116 Decision 5 stays `refines:` because this ADR's own Context
relies on that item as live: it quotes item 5's "expected next step" clause as the warrant for
building the check now, which a full retirement would have removed. ADR-0123's token into ADR-0122
Decision 4 stays `refines:` because item 4's closing permission carries a proviso, that a future
extension consume the rendered reviewer skills "without changing their paths or names", and a
proviso still binding on the successor is a surviving clause.

**Seventeen supersession claims are stated in prose and never encoded**, sixteen citing a Decision
item and one an invariant slug. They share one signature: the carrier ADR states the override in its Decision
section, often naming the exact item number, and writes no token. A first pass put this figure near
forty; re-enumerating against the corpus found the surplus was informational citations, which item
4 addresses with `cites:` rather than a relation token, plus sites whose claim the carrier had
already tokenized elsewhere in the same Decision section.

Exactly one untokenized slug citation joins them, and its shape explains why the count is otherwise
all item anchors. Item 1 tokenizes the `parts-convention` citation in ADR-0015 Decision item 6, the
`config-root` citation in ADR-0016, and retires and redeclares `residue-exemptions-pinned` in
ADR-0085. But ADR-0015 Decision item **4** cites `parts-convention` again, with its own override
verb, and item 2 scopes this check per Decision item because ADR-0129 Decision 2 requires each claim
to sit at its own rationale site. A token in item 6 therefore does not satisfy item 4, which owes
one of its own. The same reasoning gives item 4 duplicate item-anchor tokens for `ADR-0001#2` and
`ADR-0009#4`, both already counted above.

One further slug citation was examined and rejected: ADR-0016 states that it narrows ADR-0015's
`provenance-banner`, but that slug declares only that "every rendered file carries the awf
generated-by banner as its first line" and names no path, so relocating the config root leaves it
true and owing nothing. Its rejection is why ADR-0015 needs no `related:` edge to ADR-0016. ADR-0047 Decision 1 records a supersession of ADR-0040's first
Decision item in prose alone. ADR-0075 Decision 4 and ADR-0087 Decision 1 each claim the override
is "recorded via `related`" when the `related:` back-pointer is also absent. ADR-0120 Decision 11
committed to hand-tokenizing "the freeform partial-supersession citations across the corpus (0105,
0119, 0081, 0108, 0020, and the rest of a grep sweep)"; the sweep caught the ADRs it named and
every slug retirement, and missed the item-level claims in the 0001-0109 range. The 0113-0130
chains are clean, so this is historical residue, not an ongoing practice failure.

**Citation shapes are not uniform, and the majority shape is not the obvious one.** Across
`docs/decisions`, item citations fall into four disjoint shapes totalling 193 occurrences:
`ADR-NNNN item N` (74), `ADR-NNNN Decision N` (69), `ADR-NNNN Decision item N` (49), and
`ADR-NNNN DN` (1). Any detector keyed to the longest and most explicit shape alone sees about a
quarter of the citation space. Slug citations diverge the same way: ADR-0105 unified the
*declaration* token to `invariant:`, but prose citations were untouched, so 87 `` `inv: <slug>` ``
citations survive across 34 ADRs alongside the `` `invariant: <slug>` `` spelling. Rarer shapes
exist too: ADR-0015 Decision 6 refers to ADR-0001's first Invariants bullet by ordinal, and
ADR-0001 declares no slugs at all.

**Three invariants are green while their proofs assert their negation.** ADR-0009 declares
`invariant: config-root` ("Config loads from `.claude/awf/config.yaml`") and
`invariant: parts-convention` (a four-tier precedence including `explicit replaceWith`). ADR-0016
relocated the config root to `.awf/`; ADR-0015 deleted the `replaceWith` field. Both slugs are
still owed backing, and what backs `config-root` is `internal/config/config_test.go:119`, a test
carrying both `// invariant: config-root` and `// invariant: awf-config-root` whose body writes a
`.claude/awf.yaml` decoy and asserts `Load` ignores it. The proof demonstrates the declaration is
false.

The third is ADR-0082's `invariant: residue-exemptions-pinned`, which declares that "the
identity-exemption list contains exactly two entries, the bootstrap template and the agents-doc
template". ADR-0085 Decision 5 added a third entry, and the proof at
`internal/project/residue_scan_test.go:28` now asserts the list has exactly three, naming the
upgrade-script template alongside the other two. It differs from the first two in a way item 1
must handle: `config-root` and `parts-convention` were each displaced by an ADR that declared a
live successor slug (`awf-config-root`, `no-replacewith`), while ADR-0085 declared none, so
retiring this slug with no replacement would drop a check that is wanted and passing.

**The mechanism the predecessors chose was available, ran as designed, and is incoherent.**
ADR-0016 Decision 7 commits that the predecessors' "backing tests (`config-root`, ...) update in
the same commit"; ADR-0015 Decision 6 says "its `parts-convention` backing test is updated in
lockstep when this ADR reaches `Implemented`." Both are test-only commitments, and both were
executed. Neither ADR proposed amending an invariant's declaration, and neither could have: an
invariant's declaration is prose in a frozen ADR body, and ADR-0116 Decision 2, as narrowed by
ADR-0120 Decision 9, permits in-place edits only to `status` and cross-reference metadata.

ADR-0085 Decision 5 is the case that proves the point, because it did reach for the forbidden
edit: it states that `inv: residue-exemptions-pinned` "is reworded accordingly". The reword could
not happen and did not; only the test moved. ADR-0082's declaration still reads "exactly two"
today. An instruction the append-only rule cannot execute, followed by a proof that moves anyway,
is the same end state the other two reached by a shorter route.

That is precisely the defect. Moving a proof to track new reality, while the declaration it
proves stays frozen at the old reality, does not preserve the invariant: it produces a green
check whose declared claim is false and whose test asserts something else. The slug survives as a
name with two contradictory definitions, one binding on readers and one binding on CI.
Retire-and-redeclare is the lifecycle's answer, and ADR-0121 and ADR-0125 use it correctly.
ADR-0015 and ADR-0016 predate the token mechanism and reached for the only tool then visible.

**A detector is the expected next step, and the evidence it waited for now exists.** ADR-0116
Decision 5 declined a mechanical check "for now", and its Alternatives table records the
verb-anchored advisory rule as "the strongest rejected option, and not refuted ... Explicitly the
expected next step", set aside because "the procedure has never been tried, so building the check
now would permanently confound what the prose alone can achieve." The procedure has since been
tried for a full release cycle. This audit is the measurement: the prose procedure holds for new
work (0113-0130 clean) and did not retroactively repair the residue it inherited.

## Decision

1. **Retire all three stale slugs retroactively, with each retirement token carried by the ADR that
   displaced the slug.** ADR-0015 gains a `supersedes-invariant:` token naming ADR-0009's
   `parts-convention`, ADR-0016 one naming ADR-0009's `config-root`, and ADR-0085 one naming
   ADR-0082's `residue-exemptions-pinned`. Each is inserted beside prose that already states the
   claim, which is ADR-0120 Decision 9's third carve-out shape and needs no new permission
   (`cites: ADR-0120#9`).

   This item deliberately names each token by key and anchor rather than writing it out. A complete
   token string inside a `## Decision` section **is** a token: the parser recognises it wherever it
   appears there (ADR-0120 Decision 1), and an earlier draft of this item wrote all three in full,
   which made this ADR their carrier and contradicted the sentence that follows. This ADR carries no
   retirement token of its own, and the way to keep that true is to not write one. The citation
   check's code-span exemption (item 5) does not help here, because it governs citations and verbs,
   not token recognition; extending it to tokens would unparse the entire corpus.

   Carrier choice is not bookkeeping preference; it decides when the retirement takes effect. A
   `supersedes-invariant:` token retires a slug only while its carrier is `Implemented`, and a
   declaration is owed only from an `Implemented` ADR. All three displacing ADRs are already
   `Implemented`, so each retirement is live in the commit that inserts its token, and the stale
   slug stops being owed in the same commit that moves its proof. Had this ADR carried the tokens
   instead, nothing would retire until its own status flip, opening a window in which a slug is
   still declared, still owed, and no longer backed. The record is also more faithful this way: the
   ADR that displaced a slug is the ADR that says so.

   The proof edits land with their tokens. The duplicated marker on
   `internal/config/config_test.go:119` is deleted so that test backs `awf-config-root` alone, and
   the marker at `internal/project/residue_scan_test.go` is retargeted from
   `residue-exemptions-pinned` to the successor declared below.

   Only the third slug needs a successor declaration, and only this ADR can supply it. ADR-0016
   declared `awf-config-root` and ADR-0015 declared `no-replacewith`, both live and backed, so
   those two retirements strand nothing. ADR-0085 declared none: it instructed a reword of ADR-0082's
   frozen declaration, which the append-only rule cannot execute, and it is itself frozen now. This
   ADR therefore declares `residue-exemptions-pinned-three` in its Invariants section, pinning the
   list at exactly the three entries the guard test already asserts. Retiring that slug bare would
   delete a live, passing check rather than a false one, which is the opposite of this item's
   purpose. The declaration lives outside its natural domain as a consequence; ADR-0082's and
   ADR-0085's `related:` name this ADR so a reader arrives from either end.

   This reverses the test-update clause of ADR-0016 Decision 7 (`refines: ADR-0016#7`), of
   ADR-0015 Decision 6 (`refines: ADR-0015#6`), and the reword clause of ADR-0085 Decision 5
   (`refines: ADR-0085#5`). All three items survive: their substantive override claims, path
   relocations, precedence collapse, and the third exemption entry all stand. Only the bookkeeping
   instruction to move a proof under a frozen declaration, or to reword one, is replaced, by the
   retirement that instruction was reaching for.

2. **`awf check` reports a supersession claim stated in prose and not encoded.** The check is
   verb-anchored and scoped to `## Decision`: it fires when an override verb occurs in the same
   Decision item as a citation of another ADR's anchor, and that Decision item carries no relation
   token for that anchor. It names the carrier, the carrier's item, the cited anchor, and the
   token shapes that would satisfy it.

   The override verbs are `supersede`, `override`, `replace`, `reverse`, `amend`, `revise`,
   `narrow`, and `generalize`. Each contributes an **enumerated set of surface forms**, not a stem
   plus a suffix rule. A generative rule was specified first and measured against the corpus:
   appending the empty string, `s`, `d`, `ed`, or `ing` to each stem fails on every e-final verb,
   because the participle elides the e (`replacing`, not `replaceing`), and fails on irregulars and
   nominalizations entirely. The corpus contains 21 occurrences of `replacing`, 23 of `overridden`,
   11 of `overriding`, and 55 of `supersedence`, all of which that rule misses, and `supersedence`
   is the form ADR-0094 uses at one of the sites item 9 backfills. A rule that misses the forms the
   corpus predominantly uses is worse than a list, because it reads as though it covers them.

   The enumerated forms are: `supersede`, `supersedes`, `superseded`, `superseding`,
   `supersedence`; `override`, `overrides`, `overrode`, `overridden`, `overriding`; `replace`,
   `replaces`, `replaced`, `replacing`, `replacement`; `reverse`, `reverses`, `reversed`,
   `reversing`, `reversal`; `amend`, `amends`, `amended`, `amending`, `amendment`; `revise`,
   `revises`, `revised`, `revising`, `revision`; `narrow`, `narrows`, `narrowed`, `narrowing`;
   `generalize`, `generalizes`, `generalized`, `generalizing`, `generalization`. Matching is on
   whole words. Enumeration also buys precision a stem match would lose: `narrower`, `narrowest`,
   and `overridable` occur in the corpus as ordinary description and none is an override claim.

   Item citations are recognized in all four shapes the corpus uses: `ADR-NNNN Decision item N`,
   `ADR-NNNN Decision N`, `ADR-NNNN item N`, and `ADR-NNNN DN`. Recognizing only the first would
   cover 49 of 193 citations. Slug citations are recognized in both the `` `inv: <slug>` `` and
   `` `invariant: <slug>` `` spellings, adjacent to an `ADR-NNNN` reference. Section scoping
   disambiguates the slug spellings for free: the same string inside `## Invariants` is a
   declaration, already parsed by `declRe`, and is never a citation.

3. **The check is unconditional and data-driven; it ships behind no config key.**
   `internal/project/check.go` runs every check for every adopter, and AGENTS.md's "`awf check` is
   the drift oracle" invariant means the same thing in every tree. A gating key would make "check
   is clean" adopter-relative for the first time. Instead the check reports nothing when no ADR
   carries an unencoded claim, so an adopter with a clean corpus never sees it and an adopter with
   residue gets findings without opting in.

   This deliberately declines the `proseGate` shape (ADR-0119 Decision 7). That gate is opt-in
   because it scans every tracked file, including prose awf does not own; this check reads only
   `docsDir/decisions`, whose grammar awf defines.

4. **`cites: ADR-NNNN#<anchor>` declares a citation informational.** It joins the inline token
   family as an inert relation: recognized only inside `## Decision`, counting toward nothing,
   surfaced in no ACTIVE.md or `awf context` rendering. It exists because a Decision item can
   legitimately mention another ADR's anchor without claiming it. ADR-0116 Decision 3 names the
   case, an ADR that cites one Decision item informationally while amending a different one
   (`cites: ADR-0065#4`, `cites: ADR-0065#3`), and third-party narration is another: ADR-0058
   recounts a refinement one earlier ADR made to another's first Decision item without itself
   claiming that anchor (`cites: ADR-0034#1`).

   A comment-shaped marker was rejected for this. ADR-0121's `<!-- awf:comment -->` is a
   whole-line directive stripped at ingestion, so it cannot mark a mid-sentence citation, and
   ADR-0105's `touches-invariant:` is scoped to source and test markers. Reusing the ADR-0120
   token family keeps one grammar and lets the corpus view carry the suppression as parsed data
   rather than an out-of-band regex.

5. **Five citation classes are exempt by construction, not by marker.** The check never demands a
   token when the cited target is `Proposed` (ADR-0120 Decision 2 forbids the token outright, so
   demanding one would red a second check), when the citation is a self-citation (already a
   `GraphFaults` report), when the cited invariant bullet carries no slug (ADR-0001's bullets are
   unslugged and therefore unanchorable in any grammar this project has), when the citation falls
   outside `## Decision`, or when it falls inside an inline code span.

   The code-span exemption is what lets an ADR discuss the grammar without triggering it. This
   ADR's own item 2 enumerates every override verb, and items 1, 7, and 9 name the `refines:` and
   `supersedes:` tokens as data; all sit inside backticks, and none is an assertion that any
   anchor is superseded. Code-span boundaries are a structural fact the parser already holds, in
   the same class as the other four exemptions. A broader "inside a quotation" exemption was
   rejected for the opposite reason: detecting quoted spans across block quotes and inline double
   quotes is not mechanically decidable, and an undecidable exemption would repeat the mistake
   this ADR corrected during review when it withdrew a relation for lacking a decidable criterion.
   Genuine quoted citations take `cites:` instead, which is why item 4's two motivating cases are
   both quotations.

   The outside-`## Decision` class is a permanent hole and is recorded as such. ADR-0120 Decision
   1 recognizes tokens only inside `## Decision`, so a Context-section citation such as ADR-0034's
   statement that it refines rather than replaces one of ADR-0015's Decision items
   (`cites: ADR-0015#4`) cannot be encoded where it sits, and moving it is a content edit the
   append-only rule forbids. Five ADRs carry this shape. They stay untokenized; the alternative,
   appending a bookkeeping item under ADR-0120 Decision 9's shape 2, is available to an author who
   judges a specific case worth it, and is not required here.

6. **The `adr-reviewer` doc-currency lens is the named owner of the verbless residue.** ADR-0116
   Decision 4 charged that lens with the partial-amendment back-pointer rule as "the backstop for
   the case the procedure cannot reach", and item 2's verb anchoring leaves part of the claim
   space to it: a claim whose prose carries no listed verb, such as ADR-0060 Decision 5's "every
   invariant listed in ADR-0043/0027 keeps its current wording". That lens keeps its existing
   charge and gains one item, widening its citation coverage from claimed anchors to cited ones.
   The accepted recall gap therefore has an owner rather than only an acknowledgement.

7. **This ADR does not extend ADR-0120 Decision 9's carve-out** (`cites: ADR-0120#9`). Correcting
   a `refines:` token to `supersedes:` needs no new permission: ADR-0128 Decision 4 already
   classifies it as "an ordinary authoring edit, not a migration concern"
   (`cites: ADR-0128#4`), because the migration that downgraded it was the meaning-altering event
   and the correction restores the meaning its carrier's prose always stated. Inserting a token
   beside an existing prose citation is carve-out shape 3, already permitted. The carve-out is
   cited, not widened.

8. **Citation extraction is a method on the corpus view.** ADR-0130's `corpus-owns-field-reads`
   forbids any file outside `internal/adr` from reading `ADR.Refs` or `Sections`, and that
   invariant is currently clean and already backed at `internal/adr/corpus_test.go:123`.
   Extraction therefore lands in `internal/adr` as a `Corpus` method, and `internal/project`
   consumes parsed citations only. No new invariant is declared for this: ADR-0130's existing one
   covers it. The check itself lands in a new `internal/project/citations.go` rather than growing
   the 254-line `supersession.go`.

9. **This repo completes its own retrofit, this ADR included, before the check ships.** The 4
   relation corrections, the 17 backfilled tokens, item 1's three retirements and one
   redeclaration, the missing `related:` back-pointers, and this ADR's own `cites:` tokens all land
   before item 2's check is enabled, so it ships green. Including this ADR is not a formality: its
   Decision section carries 10 item citations and 3 slug citations, and a check whose own defining
   document could not pass it would be evidence the token shape is wrong.

   Item 1's retirements are not part of the pre-flip batch in the ordering sense that phrase
   suggests: each takes effect the moment its token lands on an already-`Implemented` carrier, so
   each must be committed together with the proof edit it authorises, and neither half may land
   alone. The corrections are separated from the bulk backfill for a different reason: flipping a
   refinement to a retirement can complete an anchor's coverage and force a status flip, which is a
   different concern from inserting a token beside prose that already states the claim.

   The audit verified the back-pointer edge for every token site and found seven missing edges, all
   on the target side: ADR-0004 lacks 28, ADR-0022 lacks 43, ADR-0024 lacks 26, ADR-0040 lacks 47,
   ADR-0045 lacks 87, ADR-0069 lacks 75, and ADR-0082 lacks 85. This
   matters because ADR-0128 Decision 5 requires a back-pointer on a target of any status
   (`cites: ADR-0128#5`), so a missed edge fails the retrofit commit. The edges this ADR's own
   tokens require are already in place, added in the commit that amended it.

## Invariants

- `` `invariant: citation-check-decision-scoped` ``: the citation check considers only text
  inside a `## Decision` section; an override verb and an anchor citation together in Context,
  Consequences, Alternatives Considered, or Invariants never produce a finding.
- `` `invariant: citation-check-item-shapes` ``: an item citation is recognized in all four corpus
  shapes (`ADR-NNNN Decision item N`, `ADR-NNNN Decision N`, `ADR-NNNN item N`, `ADR-NNNN DN`).
- `` `invariant: citation-check-slug-spellings` ``: a slug citation is recognized in both the
  `` `inv: <slug>` `` and `` `invariant: <slug>` `` spellings.
- `` `invariant: citation-check-verb-forms` ``: an override verb matches exactly the surface forms
  item 2 enumerates for it, on a whole-word basis, and no others; in particular `narrower`,
  `narrowest`, and `overridable` do not match, and `overridden`, `overriding`, `replacing`, and
  `supersedence` do.
- `` `invariant: citation-check-exempts-proposed-target` ``: a citation whose target ADR is
  `Proposed` produces no finding.
- `` `invariant: citation-check-exempts-self-citation` ``: a Decision item citing an anchor of its
  own ADR produces no finding.
- `` `invariant: citation-check-exempts-unslugged-bullet` ``: a citation of an invariant bullet
  that declares no slug produces no finding, because `Anchor` addresses a slug only by slug string
  and offers no ordinal form.
- `` `invariant: citation-check-exempts-code-spans` ``: an anchor citation or override verb inside
  an inline code span produces no finding.
- `` `invariant: cites-token-suppresses-citation-check` ``: a `cites:` token suppresses the
  citation check for the anchor it names, and for that anchor only.
- `` `invariant: cites-token-uncounted` ``: a `cites:` token contributes nothing to anchor
  coverage.
- `` `invariant: cites-token-unrendered` ``: a `cites:` token appears in no ACTIVE.md or
  `awf context` supersedence rendering.
- `` `invariant: residue-exemptions-pinned-three` ``: the identity-exemption list contains exactly
  three entries, the bootstrap template, the upgrade-script template, and the agents-doc template;
  extending the list requires a successor ADR. This redeclares ADR-0082's
  `residue-exemptions-pinned` at the reality ADR-0085 established, which that ADR could not do
  itself (item 1). The per-entry staleness rule is deliberately not restated: it belongs to
  ADR-0082's `template-source-residue`, which is live, backed, and unaffected by this ADR.

## Consequences

The supersession record becomes complete for the first time: after item 9's retrofit, every
override an ADR states in its Decision section is encoded, and item 2 keeps it that way. ACTIVE.md
and `awf context` annotations become trustworthy as a *complete* account of what no longer binds,
rather than a lower bound on it.

Two invariants stop being enforced, and that is a reduction in real coverage, not a cleanup of
dead weight. `config-root` and `parts-convention` describe properties nobody wants violated. What
item 1 removes is the false claim that the corpus was checking them; their live successors
(`awf-config-root`, `no-replacewith`) are what actually hold today, and the audit confirmed both
are backed. The third retirement costs nothing, because item 1 redeclares it: the exemption list
keeps a slug, keeps its proof, and gains a declaration that matches what the proof asserts.

The check's recall is bounded by its trigger, and the bound is partly unmeasurable. Every one of
the 15 owed tokens item 9 backfills carries one of item 2's enumerated verbs in its Decision item,
so the trigger reaches all of them once the enumeration is correct. That figure must not be read
as a recall estimate. The enumeration that produced it swept for override verbs, which is the
signal the trigger keys on, so it could not have found a verbless claim: the measurement and the
mechanism share a blind spot. ADR-0060 Decision 5 proves the class is non-empty (item 6 quotes it);
nothing here establishes its size. The honest statement is that the check catches every claim the
audit could see, and that the audit could not see the class item 6 assigns.

Item 6 assigns that remainder to the `adr-reviewer` lens rather than leaving it unowned, but a
probabilistic reviewer is a weaker guarantee than a check, and that asymmetry is accepted here.
The alternative, a trigger firing on every ADR cross-reference, was measured by ADR-0116 as
materially less precise and would be disabled within a release. The gap is recorded so a future
observer does not read a green check as proof of completeness.

`cites:` is new vocabulary an adopter must learn, and its absence is silent: an author who omits
it on a genuinely informational citation gets a finding and may encode a claim that was never
meant, recording a supersession that did not happen. The check's message names `cites:` as one of
the satisfying shapes to make the choice explicit at the point of failure.

Correcting the 4 relations changes derived state. Each correction moves an anchor from
uncontested to retired, and coverage completion forces a predecessor's status to `Superseded`
(ADR-0128 Decision 4). The audit confirmed no ADR is within reach of full coverage from these 4
alone: the nearest live ADR, ADR-0001, remains 2 anchors short after its correction. Exactly one
backfilled token asserts a retirement rather than a refinement, ADR-0047's into ADR-0040 Decision
item 1, and it takes ADR-0040 to 2 covered anchors of 9 (seven Decision items, two declared slugs,
one of which `bootstrap-pin` already carries a retirement). Nothing here completes a coverage set. Item 1's
retirements move ADR-0009 from zero to two covered anchors of fifteen (eight Decision items,
seven declared slugs), and the two Decision items this ADR touches on it carry only refinements,
which count toward nothing, so ADR-0009 also stays far from coverage-derived supersession.
ADR-0082 goes to one covered anchor of six (four Decision items, two declared slugs). That
headroom is measured, not structural, and a future correction may complete a coverage set.

Adopters upgrading past this release may see findings on their own corpora with no way to defer
them, which is the cost of item 3's refusal of a config key. The findings name exact edits, and
the token insertions they ask for are permitted by ADR-0120 Decision 9 shape 3 without reopening
any ADR.

**Doc-currency obligations.** `cites:` adds a token shape to the grammar, and several surfaces
enumerate the legal shapes exhaustively or teach the rule this ADR changes. The commit that flips
this ADR to `Implemented` updates, via the `.awf/` source and `./x sync` where the artifact is
rendered: `.awf/skills/parts/adr-lifecycle/` (the rendered
`templates/skills/adr-lifecycle/SKILL.md.tmpl` teaches the token family),
`docs/decisions/README.md` (the supersedence section), `.awf/docs/glossary.yaml` (the supersession
term entries), `.awf/domains/parts/adr-system/current-state.md` and
`.awf/domains/parts/invariants/current-state.md`, `.awf/agents/adr-reviewer.yaml`, and
`.awf/agents-doc.yaml` with its rendered `AGENTS.md` invariant line. `internal/adr/adr.go`'s
`Relation` doc comment is updated in the same change. The `adr-reviewer.yaml` edit carries the
wholesale-override hazard ADR-0116 Decision 4 recorded: its `docCurrencyItems` key replaces the
catalog defaults entirely and already restates all seven verbatim, so item 6's new entry must be
appended there; adding it to the catalog default alone would never reach this repo.
`docs/decisions/ACTIVE.md` is regenerated by `./x sync` in the status-flip commit; no
`docs/decisions/README.md` index row is owed (ADR-0005).

Item 1's retarget carries doc-currency obligations of its own, in the commit that performs it.
`internal/project/residue_scan_test.go` attributes the exemption list in both a comment ("ADR-0082
Decision 2, extended to three entries by ADR-0085 Decision 5") and the guard's failure message
("ADR-0082, last extended by ADR-0085"); both must name this ADR once it owns the successor slug,
or the test explains itself by citing only the decisions it no longer implements. The
`.awf/skills/parts/adr-lifecycle/` path named above does not exist today: the token family is
taught by the shipped default at `templates/skills/adr-lifecycle/SKILL.md.tmpl`, and because
`cites:` changes the grammar for every adopter rather than this repo alone, that default is what
changes, not a repo-local part override.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Add a `refines-invariant:` relation for re-scoped slugs | Withdrawn during review for want of an instance. The motivating case, ADR-0016 re-scoping ADR-0015's `provenance-banner`, is not one: that slug declares "every rendered file carries the awf generated-by banner as its first line" and names no path, so relocating the config root leaves it literally true and owes no token. Without a decidable test separating "re-scoped but in force" from "meaning narrowed", the relation would also license the frozen-declaration-versus-moved-reality state item 1 exists to end. ADR-0128 Decision 2's retire-and-redeclare stands. The third stale slug found after review, ADR-0082's `residue-exemptions-pinned`, confirms the withdrawal rather than reopening it: "exactly two" against a proof asserting exactly three is a contradiction, not a narrowing, so it takes a retirement. Unlike the other two, whose successors were already declared elsewhere, it also takes a redeclaration here. |
| Exempt any citation inside a quotation | Rejected as undecidable: detecting quoted spans across block quotes and inline double quotes is not a structural fact the parser holds, unlike the five exemptions item 5 does adopt. It would also delete one of the two motivating cases for `cites:`, since third-party narration is quotation. Code spans alone are decidable and cover the discuss-the-grammar case. |
| Tokenize every self-trip site and add no exemption | The reviewer's preference, and close: it dogfoods hardest. Rejected because item 2's own enumeration of the eight override verbs would then need a token, which asserts a claim about anchors that the enumeration does not make. A definitional list is a mention, not a use. |
| Opt-in config key, mirroring `proseGate` | The shape fits the migration problem but breaks the drift oracle: no check in `internal/project/check.go` is config-gated today, and a key makes "`awf check` is clean" mean different things in different trees. Item 3's data-driven silence gets the same adopter experience without that cost. |
| Advisory `repoaudit` rule, as ADR-0116 sketched | Range-scoped and needing no backfill, but repo-local: adopters get no mechanism at all, and the residue this ADR measures is exactly what an ignorable channel failed to prevent. |
| Cite-anchored trigger (any anchor citation owes a token or a `cites:`) | Closes the recall gap item 2 accepts, at the price of marking every informational citation corpus-wide. ADR-0116 already judged this trigger materially less precise, and a gate that fires on ordinary cross-references gets disabled. |
| Recognize only the `ADR-NNNN Decision item N` citation shape | Measured at 49 of 193 corpus citations. The check would pass its declared invariants while missing three quarters of the claim space, and would not even recognize the majority of this ADR's own citations. |
| Keep moving proofs under frozen declarations, as ADR-0015 and ADR-0016 instructed | The instruction was followed and produced the defect: a green check whose declared claim is false and whose test asserts something else. Amending the declaration instead is forbidden by the append-only rule, so retirement is the only remaining coherent option. |
| Leave `config-root` and `parts-convention` alone | Preserves two prior decisions verbatim at the cost of leaving two invariants green whose proofs assert their negation. A check asserting something untrue is worse than one fewer check. |
| Extend ADR-0120 Decision 9's carve-out for relation corrections | Unnecessary: ADR-0128 Decision 4 already classifies the correction as ordinary authoring. Widening an exhaustive carve-out without need weakens the append-only rule. |
| One ADR per workstream (retirements, detector) | They share one commitment and one failure mode, and the retirements are the motivating instance for the detector. Splitting would spread one decision across two review cycles. |
