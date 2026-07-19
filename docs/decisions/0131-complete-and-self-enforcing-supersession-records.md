---
status: Implemented
date: 2026-07-18
tags: [adr-lifecycle, adr-parsing, invariant-retirement, invariant-backing, active-md]
related: [5, 6, 8, 9, 15, 16, 20, 23, 32, 34, 57, 58, 65, 79, 82, 85, 105, 116, 119, 120, 121, 128, 129, 130]
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
section, often naming the exact item number, and writes no token. ADR-0047 Decision 1 records an
override of ADR-0040's first Decision item in prose alone. ADR-0075 Decision 4 and ADR-0087
Decision 1 each claim the override is "recorded via `related`" when the `related:` back-pointer is
also absent. ADR-0120 Decision 11 committed to hand-tokenizing "the freeform partial-supersession
citations across the corpus (0105, 0119, 0081, 0108, 0020, and the rest of a grep sweep)"; the
sweep caught the ADRs it named and every slug retirement, and missed the item-level claims in the
0001-0109 range. The 0113-0130 chains are clean, so this is historical residue, not an ongoing
practice failure.

A first pass put this figure near forty; re-enumerating against the corpus found the surplus was
informational citations, which item 4 addresses with `cites:` rather than a relation token, plus
sites whose claim the carrier had already tokenized elsewhere in the same Decision section. A later
pass found the enumeration had also *missed* one, ADR-0047's, which this Context had named all
along. Both directions of error came from sweeping for a signal rather than reading the sites.

Exactly one untokenized slug citation joins them, and its shape explains why the count is otherwise
all item anchors. Item 1 tokenizes the `parts-convention` citation in ADR-0015 Decision item 6, the
`config-root` citation in ADR-0016, and retires and redeclares `residue-exemptions-pinned` in
ADR-0085. But ADR-0015 Decision item **4** cites `parts-convention` again, with its own override
verb, and item 2 scopes this check per Decision item because ADR-0129 Decision 2 requires each claim
to sit at its own rationale site. A token in item 6 therefore does not satisfy item 4, which owes
one of its own. The same reasoning gives item 4 duplicate item-anchor tokens for `ADR-0001#2` and
`ADR-0009#4`, both already counted above.

One further slug citation was examined and rejected as a retirement: ADR-0016 states that it
narrows ADR-0015's `provenance-banner`, but that slug declares only that "every rendered file
carries the awf generated-by banner as its first line" and names no path, so relocating the config
root leaves it true and owing no retirement.

It does not follow that the site owes nothing at all, and an earlier draft of this paragraph drew
that conclusion and used it to argue that ADR-0015 needs no `related:` edge to ADR-0016. Decision 4
falsified it: the citation is informational, which is precisely what `cites-invariant:` is for, so
the site takes an inert token and the edge becomes owed like any other. The reasoning was sound
when written and stale by the time the citation grammar existed, which is the ordinary hazard of a
Context section that argues from a decision the same ADR later revises. What survives is the
narrower and still-correct claim: a slug whose declaration stays literally true owes no retirement.

**Citation shapes are not uniform, and the majority shape is not the obvious one.** Across
`docs/decisions`, item citations fall into four disjoint shapes totalling 186 occurrences:
`ADR-NNNN item N` (78), `ADR-NNNN Decision item N` (59), `ADR-NNNN Decision N` (48), and
`ADR-NNNN DN` (1). Any detector keyed to the longest and most explicit shape alone sees under a
third of the citation space.

These figures are measured over `docs/decisions/*.md` excluding this ADR and the generated
`ACTIVE.md`, with newlines folded to spaces first, because a citation wrapped across a line break
is invisible to a line-scoped `grep`:

```
cat <files> | tr '\n' ' ' | tr -s ' ' | grep -ohE "ADR-[0-9]{4}('s)? Decision item [0-9]+" | wc -l
```

and the same for the other three shapes. The command is recorded because an earlier draft of this
paragraph carried a `Decision N` count of 69 that no method reproduces; the shapes are disjoint by
construction, since each pattern requires its own literal between the ADR number and the digit. Slug citations diverge the same way: ADR-0105 unified the
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

1. **Retire all six stale slugs retroactively, with each retirement token carried by the ADR that
   displaced the slug.** ADR-0015 gains a `supersedes-invariant:` token naming ADR-0009's
   `parts-convention`, ADR-0016 one naming ADR-0009's `config-root`, and ADR-0085 one naming
   ADR-0082's `residue-exemptions-pinned`. ADR-0020 gains two, naming ADR-0005's
   `sync-generates-active-md` and ADR-0006's `render-active-md`, and ADR-0032 gains one naming
   ADR-0023's `uninstall-removes-lock-tracked`. Each is inserted beside prose that already states
   the claim, which is ADR-0120 Decision 9's third carve-out shape and needs no new permission
   (`cites: ADR-0120#9`).

   The last three were found by item 2's check rather than by the prose audit that found the first
   three, which is the recall difference item 9 predicted, measured. Each shows the same pathology:
   a frozen declaration whose stated claim the displacing ADR falsified, and a proof marker that
   passes because it asserts the new behaviour instead. ADR-0005's slug declares that an ADR-less
   decisions dir "writes no `ACTIVE.md` and prunes any previously locked one", and ADR-0006's that
   `RenderActiveMD` "returns `\"\"` when the directory holds no ADRs"; ADR-0020 Decision 6 made
   `ACTIVE.md` always render, and the markers at `internal/project/project_test.go:833` and
   `internal/adr/adr_test.go:193` now assert the placeholder, which is the negation of the sentence
   they are recorded as backing. ADR-0023's slug is a three-part conjunction whose middle clause
   unsets `core.hooksPath`; ADR-0032 Decision 7 says in terms that the clause "is dropped", so the
   sentence is false as written, while the marker at `internal/project/install_test.go:41` asserts
   only the lock-entry half. ADR-0032 also promises that the "lock-tracked-removal substance" of
   the slug remains "in force and backed", which is coherent prose and was true when written: it
   preserves the substance while dropping a clause. It is not a reason to leave the declaration
   standing, because the declaration is a conjunction and one conjunct is now false, so the
   sentence is false whatever substance survives. That promise is reversed here rather than
   contradicted, which is why it earns a token of its own below.

   This item deliberately names each token by key and anchor rather than writing it out. A
   complete token string inside a `## Decision` section **is** a token: the parser recognises it
   wherever it appears there (ADR-0120 Decision 1 (`cites: ADR-0120#1`)), and an earlier draft of
   this item wrote them out in full, which made this ADR their carrier and contradicted the
   sentence that follows. This ADR carries no retirement token of its own, and the way to keep
   that true is to not write one. The citation check's code-span exemption (item 5) does not help
   here, because it governs citations and verbs, not token recognition; extending it to tokens
   would unparse the entire corpus.

   Carrier choice is not bookkeeping preference; it decides when the retirement takes effect. A
   `supersedes-invariant:` token retires a slug only while its carrier is `Implemented`, and a
   declaration is owed only from an `Implemented` ADR. All three displacing ADRs are already
   `Implemented`, so each retirement is live in the commit that inserts its token, and the stale
   slug stops being owed in the same commit that moves its proof. Had this ADR carried the tokens
   instead, nothing would retire until its own status flip, opening a window in which a slug is
   still declared, still owed, and no longer backed. The record is also more faithful this way: the
   ADR that displaced a slug is the ADR that says so.

   The proof edits land with their tokens, at nine marker sites. The duplicated marker on
   `internal/config/config_test.go:119` is deleted so that test backs `awf-config-root` alone. The
   marker at `internal/project/residue_scan_test.go` is retargeted from `residue-exemptions-pinned`
   to the successor declared below. And the `parts-convention` marker at
   `internal/project/render_tree_test.go:83` is deleted outright: that slug is backed today, so
   retiring it without removing its proof would strand the marker, and its live successor
   `no-replacewith` is already backed at `internal/config/config_test.go:249`, so the deletion costs
   no real coverage.

   The three later retirements are all retargets, because each backs behaviour that is still
   wanted. The two `sync-generates-active-md` markers (`internal/project/project_test.go:785` and
   `:833`) move to `sync-always-writes-active-md`, the three `render-active-md` markers
   (`internal/adr/adr_test.go:18`, `:104`, `:193`) to `render-active-md-grouped-or-placeholder`, and
   the `uninstall-removes-lock-tracked` marker (`internal/project/install_test.go:41`) to
   `uninstall-removes-lock-entries`. None is deleted: the tests assert grouping by status, the
   superseded-variant grouping, the placeholder render, sync's write-and-lock behaviour, and
   uninstall's lock-entry removal, every one of which survives its slug.

   Four of the six need a successor declaration, and only this ADR can supply them. ADR-0016
   declared `awf-config-root` and ADR-0015 declared `no-replacewith`, both live and backed, so
   those two retirements strand nothing. The other four displacing ADRs declared no successor at
   all. ADR-0085 instructed a reword of ADR-0082's frozen declaration, which the append-only rule
   cannot execute, and it is itself frozen now; ADR-0020's Invariants section declares only
   `dead-reference-gated`, and ADR-0032's carries nothing covering the surviving lock-entry
   substance. This ADR therefore declares `residue-exemptions-pinned-three`,
   `sync-always-writes-active-md`, `render-active-md-grouped-or-placeholder`, and
   `uninstall-removes-lock-entries` in its Invariants section, each pinning what its retargeted
   markers already assert. Retiring any of the four bare would delete a live, passing check rather
   than a false one, which is the opposite of this item's purpose. The declarations live outside
   their natural domain as a consequence, so each ADR whose slug is redeclared here names this ADR
   in its `related:` field and a reader arrives from either end. ADR-0082 and ADR-0085 already do;
   ADR-0005, ADR-0006, and ADR-0023 gain the edge in the commit that lands their retirement.

   That edge is documentation, and it is not the one the check enforces. A back-pointer is owed to
   the **carrier of the token**, which for these retirements is the displacing ADR rather than this
   one: ADR-0005 and ADR-0006 owe an edge to ADR-0020, and ADR-0023 already carries its edge to
   ADR-0032. Both kinds are owed and neither substitutes for the other, so a target may name two
   ADRs for one retirement. ADR-0009 is the landed example, naming its carriers ADR-0015 and
   ADR-0016 alongside this ADR. Stating only the documentation edge is what led the implementing
   plan to schedule it alone and miss the enforced one.

   The asymmetry is worth stating, because it is the general rule this item follows rather than an
   accident of these six. A retirement strands nothing when the displacing ADR already declared a
   successor slug covering the surviving substance, and strands a live check when it did not.
   Declaring the successor is therefore not optional bookkeeping: it is the difference between
   retiring a false claim and deleting a true one, and the author who retires is the only one who
   can tell which case they are in.

   This reverses the test-update clause of ADR-0016 Decision 7 (`refines: ADR-0016#7`), of
   ADR-0015 Decision 6 (`refines: ADR-0015#6`), and the reword clause of ADR-0085 Decision 5
   (`refines: ADR-0085#5`). All three items survive: their substantive override claims, path
   relocations, precedence collapse, and the third exemption entry all stand. Only the bookkeeping
   instruction to move a proof under a frozen declaration, or to reword one, is replaced, by the
   retirement that instruction was reaching for.

   The three later retirements reverse a clause each, and for the same reason. ADR-0020 Decision 6
   states the defect more plainly than any other item in the corpus: having named the two falsified
   clauses exactly, it closes "the backing tests for both invariants are updated to assert the new
   behaviour, keeping their `// invariant:` markers", which is the proof move under a frozen
   declaration that this item exists to undo, with an explicit instruction to keep the markers
   where they are. This ADR retargets those five markers instead (`refines: ADR-0020#6`).
   ADR-0032 Decision 7 promises that the "lock-tracked-removal substance" of its slug remains "in
   force and backed"; retiring the slug reverses that promise as stated, even though the
   redeclaration preserves the substance under a new name (`refines: ADR-0032#7`). Both items
   survive in full otherwise: ADR-0020's always-render decision and ADR-0032's removal of hook
   activation are untouched, and only the bookkeeping clause about where a proof lives, or whether
   a slug endures, is replaced.

   That these were found by a check rather than by reading is the honest lesson, and it is not that
   the prose was silent. ADR-0020 Decision 6 says everything a reader needed; three separate audits
   read past it. A verb-anchored check does not read better than a person, it reads every item
   every time, and that is the difference the retrofit is buying.

   ADR-0032's sentence is left standing rather than rewritten: it is frozen content, and the
   refinement token beside it is the correction the append-only rule permits.

2. **`awf check` reports a supersession claim stated in prose and not encoded.** The check is
   verb-anchored and scoped to `## Decision`: it fires when an override verb occurs in the same
   Decision item as a citation of another ADR's anchor, and that Decision item carries no relation
   token for that anchor. It names the carrier, the carrier's item, the cited anchor, and the
   token shapes that would satisfy it.

   **Item claims are scoped per Decision item; slug claims are scoped per carrier.** For an item
   anchor the carrier item is the rationale site (ADR-0129 Decision 2 (`cites: ADR-0129#2`)), and
   two items may hold genuinely different relations to one target, so a token at item 7 does not
   answer a claim at item 5. A slug anchor is not like that: a slug is atomic and dies once, so a
   carrier that already retires it anywhere in its `## Decision` section has said everything there
   is to say. Demanding a second token would ask an author to double-encode a retirement the
   corpus already records, and there is no truthful token to write. An inert `cites-invariant:`
   would deny a retirement the same ADR asserts, and a second `supersedes-invariant:` would claim
   one anchor twice from one carrier.

   Two corpus sites forced this and neither is hypothetical. ADR-0081 item 7 (`cites: ADR-0081#7`)
   discusses retiring ADR-0013's `doc-gated-skill-suppressed` while item 10 carries the token, and
   ADR-0085 item 1 (`cites: ADR-0085#1`) discusses retiring ADR-0040's `bootstrap-pin` while item
   9 carries it; `ACTIVE.md` records both retirements as landed. A per-item rule reported both as
   unencoded.

   Only the retirement key carries across items. A `cites-invariant:` suppresses at its own item
   and nowhere else, matching the anchor-only scope of the suppression rule below. The reason is
   the same one that motivates the wider scope for the other key, read the other way: a retirement
   is a fact about the slug, so restating it is redundant, while a citation is a fact about the
   item that makes it, so one item's inert mention says nothing about what a later item asserts.
   Letting it carry would let an author suppress a genuine unencoded retirement at item 9 by
   mentioning the slug informationally at item 3. With that split the narrower scoping costs no
   recall: the carrier still owes exactly one retirement token per retired slug, and a carrier that
   retires nothing still reports every slug claim it leaves unencoded.

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
   cover 59 of 186 citations. Slug citations are recognized in both the `` `inv: <slug>` `` and
   `` `invariant: <slug>` `` spellings, adjacent to an `ADR-NNNN` reference. Section scoping
   disambiguates the slug spellings for free: the same string inside `## Invariants` is a
   declaration, already parsed by `declRe`, and is never a citation.

   **Two wrappers around the ADR reference are recognized, and one shape is deliberately not.**
   The possessive `ADR-NNNN's Decision item N` and the markdown link
   `[ADR-NNNN](path.md) Decision item N` carry the ADR number in a form a bare `ADR-NNNN` prefix
   match misses. Both are admitted, because each still names its target unambiguously and neither
   costs precision. The possessive omission was a latent spec-to-code gap: this ADR's own Context
   measured the corpus with `ADR-[0-9]{4}('s)? Decision item [0-9]+` while the implementation
   dropped the `('s)?`, so the ADR asserted a coverage its check did not have. The link shape
   matters for the same reason: ADR-0047 Decision 1 (`cites: ADR-0047#1`), the one claim this
   ADR's Context records the enumeration as having *missed*, is written that way.

   Not recognized: a bare `item N` whose ADR reference sits earlier in the same Decision item, as
   in ADR-0032 item 7 (`cites: ADR-0032#7`)'s "partially supersedes ADR-0023 ... item 2 ... item
   3". Resolving those against the nearest preceding `ADR-NNNN` was measured over the corpus and
   rejected: it yields roughly 48 candidates of which the large majority are an ADR referring to
   *its own* items ("item 5 below", "item 2's verb anchoring") or to hypothetical ones, including
   this item's own "a token at item 7 does not answer a claim at item 5". No rule short of reading
   the sentence separates them, and a check whose findings are mostly false teaches authors to
   ignore it. The genuine instances are hand-encoded instead; ADR-0032 item 7's three were the
   only ones the corpus still owed. This is an accepted recall boundary, recorded rather than
   hidden, and it is the same standard this item applies to the verb list: a rule that reads as
   though it covers a shape it does not is worse than one that says where it stops.

3. **The check is unconditional and data-driven; it ships behind no config key.**
   `internal/project/check.go` runs every check for every adopter, and AGENTS.md's "`awf check` is
   the drift oracle" invariant means the same thing in every tree. A gating key would make "check
   is clean" adopter-relative for the first time. Instead the check reports nothing when no ADR
   carries an unencoded claim, so an adopter with a clean corpus never sees it and an adopter with
   residue gets findings without opting in.

   This deliberately declines the `proseGate` shape (ADR-0119 Decision 7). That gate is opt-in
   because it scans every tracked file, including prose awf does not own; this check reads only
   `docsDir/decisions`, whose grammar awf defines.

4. **`cites: ADR-NNNN#<item>` and `cites-invariant: ADR-NNNN#<slug>` declare a citation
   informational.** They join the inline token family as an inert relation: recognized only inside
   `## Decision`, counting toward nothing, surfaced in no ACTIVE.md or `awf context` rendering. It
   exists because a Decision item can legitimately mention another ADR's anchor without claiming
   it. ADR-0116 Decision 3 (`cites: ADR-0116#3`) names the case, an ADR that cites one Decision
   item informationally while amending a different one (`cites: ADR-0065#4`, `cites: ADR-0065#3`),
   and third-party narration is another: ADR-0058 recounts a refinement one earlier ADR made to
   another's first Decision item without itself claiming that anchor (`cites: ADR-0034#1`).

   **Two keys, not one, because the key names the anchor kind (ADR-0120 item 1
   (`cites: ADR-0120#1`)).** A single `cites:` key carrying both anchor shapes is not decidable:
   the slug grammar admits an all-digit slug, so `#3` could be Decision item 3 or the slug `3`,
   and this is exactly why `internal/project/supersession.go` keys anchors by kind prefix and
   treats an item ref and a slug ref into one target as distinct anchors. Two patterns over one
   key do not resolve it either, they double-match: every item-anchored citation would also
   register a phantom slug anchor and fail the token-ref check against the target's declared
   slugs. The retirement tokens already avoid this the same way, with `supersedes:` and
   `supersedes-invariant:` as separate keys, so the citation tokens mirror that split rather than
   inventing a shape-inference rule the rest of the grammar rejects.

   A comment-shaped marker was rejected for this. ADR-0121's `<!-- awf:comment -->` is a
   whole-line directive stripped at ingestion, so it cannot mark a mid-sentence citation, and
   ADR-0105's `touches-invariant:` is scoped to source and test markers. Reusing the ADR-0120
   token family keeps one grammar and lets the corpus view carry the suppression as parsed data
   rather than an out-of-band regex.

5. **Five citation classes are exempt by construction, not by marker.** The check never demands
   a token when the cited target is `Proposed` (ADR-0120 Decision 2 (`cites: ADR-0120#2`) forbids
   the token outright, so demanding one would red a second check), when the citation is a
   self-citation (already a `GraphFaults` report), when the cited invariant bullet carries no slug
   (ADR-0001's bullets are unslugged and therefore unanchorable in any grammar this project has),
   when the citation falls outside `## Decision`, or when it falls inside an inline code span.

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
   1 (`cites: ADR-0120#1`) recognizes tokens only inside `## Decision`, so a Context-section
   citation such as ADR-0034's statement that it refines rather than replaces one of ADR-0015's
   Decision items (`cites: ADR-0015#4`) cannot be encoded where it sits, and moving it is a
   content edit the append-only rule forbids. Five ADRs carry this shape. They stay untokenized;
   the alternative, appending a bookkeeping item under ADR-0120 Decision 9 (`cites: ADR-0120#9`)'s
   shape 2, is available to an author who judges a specific case worth it, and is not required
   here.

6. **The `adr-reviewer` doc-currency lens is the named owner of the verbless residue.**
   ADR-0116 Decision 4 (`refines: ADR-0116#4`) charged that lens with the partial-amendment
   back-pointer rule as "the backstop for the case the procedure cannot reach", and item 2's verb
   anchoring leaves part of the claim space to it: a claim whose prose carries no listed verb,
   such as ADR-0060 Decision 5 (`cites: ADR-0060#5`)'s "every invariant listed in ADR-0043/0027
   keeps its current wording". That lens keeps its existing charge and gains one item, widening
   its citation coverage from claimed anchors to cited ones. The accepted recall gap therefore has
   an owner rather than only an acknowledgement.

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
   relation corrections, the 17 backfilled tokens, item 1's six retirements and four
   redeclarations, the corpus-wide tokenization of informational citations with `cites:`, the
   missing `related:` back-pointers, and this ADR's own `cites:` tokens all land before item 2's
   check is enabled, so it ships green.

   The check's own first run against the corpus is the measurement this item promised, and it
   revised the retrofit's size rather than confirming it. It reported 44 unencoded claims across
   18 carriers, where the prose audit behind the corrections and backfills had found its last
   straggler. Adjudicating all 44 against the cited target's clause set produced 3 retirements
   (item 1's later three), 5 refinements, and 34 informational citations, plus the 2 slug claims
   the per-carrier scoping in item 2 dissolves. The distribution is the finding, not the total:
   no item anchor in the corpus was under-recorded as a refinement when it was a retirement, and
   every genuine retirement the check surfaced was slug-anchored. Decision items are built upon;
   invariant declarations are falsified. The informational half is not an afterthought: item 4's
   token has to be applied everywhere it is owed, or the check fires on citations that assert
   nothing. Including this ADR is not a formality: its Decision section cites other ADRs' items
   more densely than any other ADR in the corpus, and a check whose own defining document could
   not pass it would be evidence the token shape is wrong. The exact figure is deliberately not
   stated. It was written as 10, corrected to 21, and was wrong again within the same commit,
   because item 1's rewrite added citations after the recount; a self-referential count in a
   document that keeps being amended is a claim that goes stale faster than a reader can check
   it. The check's own output is the authority. It carries no slug citation at all, which is a fact about
   the grammar rather than about this ADR: item 1 names the six retired slugs in bare backticks,
   and item 2 recognizes a slug citation only in the `` `inv: <slug>` `` and
   `` `invariant: <slug>` `` spellings adjacent to an `ADR-NNNN` reference. Naming a slug without
   its key is therefore invisible to the check, which is the same recall bound item 6 assigns to
   the reviewer lens, reached by a different route.

   Item 1's retirements are not part of the pre-flip batch in the ordering sense that phrase
   suggests: each takes effect the moment its token lands on an already-`Implemented` carrier, so
   each must be committed together with the proof edit it authorises, and neither half may land
   alone. The corrections are separated from the bulk backfill for a different reason: flipping a
   refinement to a retirement can complete an anchor's coverage and force a status flip, which is a
   different concern from inserting a token beside prose that already states the claim.

   The audit verified the back-pointer edge for every relation-token site and found seven missing
   edges there, all on the target side: ADR-0004 lacks 28, ADR-0022 lacks 43, ADR-0024 lacks 26, ADR-0040 lacks 47,
   ADR-0045 lacks 87, ADR-0069 lacks 75, and ADR-0082 lacks 85. The `cites:` insertions owe a
   further seven edges of their own, on ADR-0034 (twice), ADR-0039, ADR-0065, ADR-0086, ADR-0119,
   and ADR-0015, which lacks 16. That last one is owed only because the Context paragraph's
   withdrawn inference is withdrawn: the `provenance-banner` site takes an inert token after all,
   and its carrier is ADR-0016.
   This
   matters because ADR-0128 Decision 5 requires a back-pointer on a target of any status
   (`cites: ADR-0128#5`), so a missed edge fails the retrofit commit.

   This ADR's own `cites:` tokens owe edges on the same rule, and the rule bites earlier than it
   looks: `internal/project/supersession.go:178` requires the back-pointer for a token of any
   relation from a carrier of any status, so an inert `cites:` token is no exception and a
   `Proposed` carrier is no exception either. Its eight tokens resolve to five targets. Two,
   ADR-0015 and ADR-0128, already name this ADR. The other three, ADR-0034, ADR-0065 and ADR-0120,
   do not, and gain the edge in the same commit that first makes `cites:` parseable, because that is
   the moment the tokens go live and the edges become owed.

   The two `refines:` tokens item 1 adds for ADR-0020 Decision 6 (`cites: ADR-0020#6`) and
   ADR-0032 Decision 7 (`cites: ADR-0032#7`) are live the moment they are written, since
   `refines:` is already parseable, so ADR-0020 and ADR-0032 gain their back-pointer edge in the
   amending commit rather than later. That is the same rule biting at a different time, and it is
   worth noting that the timing follows the parser's state, not the ADR's: a token owes its edge
   as soon as the grammar recognizes it.

10. **`related:` is an ascending array, and `awf check` enforces it.** A back-pointer edge has
    exactly one correct position, so appending a low-numbered carrier to an array that already
    names a higher one is visibly wrong; three of this retrofit's edges were mid-array inserts for
    that reason. The rule was previously an authoring convention that several plan steps described
    as gate-enforced when nothing enforced it, which is the worse of the two failure modes: an
    executor who trusts a stated check appends and never learns it was wrong. Two corpus arrays had
    been out of order since before this ADR, ADR-0013 and ADR-0098, and nothing saw them.

    Sorting an existing array needs no carve-out. Append-only permits editing status and
    cross-reference metadata on a live ADR, as ADR-0118's alternatives table records when it
    rejects amending ADR-0115 in place, and `related:` is cross-reference metadata; the array
    carries an unordered set, so reordering it changes no decision and membership is unchanged.
    ADR-0118's retroactive-normalization carve-out is deliberately *not* the warrant here: that one
    is bounded to edits changing punctuation and nothing else, limited to the seven banned
    codepoints, so it does not reach a frontmatter array and invoking it would imply a wider reach
    than ADR-0118 grants. Resolution and ordering are scanned separately, so stopping at the first
    descent cannot also stop the dangling-link scan.

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
- `` `invariant: cites-token-suppresses-citation-check` ``: a citation token (`cites:` or
  `cites-invariant:`) suppresses the citation check for the anchor it names, and for that anchor
  only.
- `` `invariant: citation-check-slug-claims-per-carrier` ``: a slug claim is satisfied by a
  `supersedes-invariant:` token for that slug anywhere in the carrier's `## Decision` section, not
  only at the citing item; a `cites-invariant:` suppresses only at its own item, and an item claim
  is satisfied only at its own item.
- `` `invariant: adr-related-ascending` ``: every ADR's `related:` array ascends, and `awf check`
  reports `adr-related-order` naming the first descent when one does not. Resolution and ordering
  are reported independently, so a descending array still has every entry checked for resolution.
- `` `invariant: cites-token-uncounted` ``: a citation token of either key contributes nothing to
  anchor coverage.
- `` `invariant: cites-token-unrendered` ``: a citation token of either key appears in no ACTIVE.md
  or `awf context` supersedence rendering.
- `` `invariant: residue-exemptions-pinned-three` ``: the identity-exemption list contains exactly
  three entries, the bootstrap template, the upgrade-script template, and the agents-doc template;
  extending the list requires a successor ADR. This redeclares ADR-0082's
  `residue-exemptions-pinned` at the reality ADR-0085 established, which that ADR could not do
  itself (item 1). The per-entry staleness rule is deliberately not restated: it belongs to
  ADR-0082's `template-source-residue`, which is live, backed, and unaffected by this ADR.
- `` `invariant: sync-always-writes-active-md` ``: `awf sync` writes
  `<docsDir>/decisions/ACTIVE.md` for every decisions dir, recording it in the lock when the dir
  holds ADRs and rendering the placeholder index when it holds none. This redeclares ADR-0005's
  `sync-generates-active-md` at the reality ADR-0020 Decision 6 established (item 1). The retired
  sentence's prune clause is deliberately not restated, because the file is no longer conditionally
  absent and so is never pruned; its grouping clause is not restated either, because neither
  retargeted marker asserts grouping. That property belongs to
  `render-active-md-grouped-or-placeholder`, which is where it is proved.
- `` `invariant: render-active-md-grouped-or-placeholder` ``: `internal/adr.RenderActiveMD` groups
  its entries by status, collapsing every `Superseded by ADR-NNNN` status into one `## Superseded`
  group while each entry retains its full status string, and returns the placeholder index rather
  than the empty string for a decisions dir holding no ADRs. This redeclares
  ADR-0006's `render-active-md` at the reality ADR-0020 Decision 6 established (item 1); the
  retired sentence's byte-identical-to-the-pre-rename-generator clause is deliberately not
  restated, because it pinned a one-time refactor against a generator that no longer exists.
- `` `invariant: uninstall-removes-lock-entries` ``: `awf uninstall` removes the in-tree files
  recorded in the lock and no file outside it, reporting the count it removed. This redeclares
  ADR-0023's `uninstall-removes-lock-tracked` at what its proof actually asserts, which is
  narrower than the retired sentence in two ways beyond the `core.hooksPath` clause ADR-0032
  Decision 7 dropped: the marked test asserts neither the lock file's own removal nor the survival
  of the authored `.awf/` config, and never creates a config to survive. Those clauses are dropped
  rather than carried forward, because carrying an unproven clause under a new slug would repeat
  the defect this ADR exists to end, one ADR further along. Re-establishing either is a successor's
  job, with the assertion that earns it.

## Consequences

The supersession record becomes complete for the first time: after item 9's retrofit, every
override an ADR states in its Decision section is encoded, and item 2 keeps it that way. ACTIVE.md
and `awf context` annotations become trustworthy as a *complete* account of what no longer binds,
rather than a lower bound on it.

Two invariants stop being enforced, and that is a reduction in real coverage, not a cleanup of
dead weight. `config-root` and `parts-convention` describe properties nobody wants violated. What
item 1 removes is the false claim that the corpus was checking them; their live successors
(`awf-config-root`, `no-replacewith`) are what actually hold today, and the audit confirmed both
are backed. The other four retirements cost nothing, because item 1 redeclares each: the slug
changes, the proof stays, and the new declaration matches what the proof asserts.

Net enforcement therefore rises rather than falls. Five of the six slugs were green over a
declaration their own proof contradicted, which is worse than an unbacked slug: an unbacked slug
is reported, a falsely-backed one is not. The sixth,
`uninstall-removes-lock-tracked`, failed the other way: its proof under-asserts, covering one
conjunct of a three-part declaration. After this ADR the seven retargeted tests run under four
declarations that state what they check, and the two deleted markers
(`internal/config/config_test.go:119`, `internal/project/render_tree_test.go:83`) run under no new
declaration because their properties are already held by live successors. The cost is four slug
renames and nine marker edits, plus three clauses deliberately dropped rather than carried
forward unproven; the gain is that no surviving declaration in the corpus is known to be false,
and none claims more than its proof asserts.

The check's recall is bounded by its trigger, and the bound is partly unmeasurable. Every one of
the 17 owed tokens item 9 backfills carries one of item 2's enumerated verbs in its Decision item,
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
alone: the nearest live ADR, ADR-0001, remains 2 anchors short after its correction. Fifteen of the
17 backfilled tokens are refinements, and the two that do assert a retirement cannot move any ADR's
coverage either: both sit on ADR-0015 Decision item 4 and duplicate anchors ADR-0015 already retires
from its item 6, and a claim set is per-ADR, so re-asserting an anchor its own carrier has already
retired adds no member to any coverage set. Encoding those two as refinements instead would be worse
than redundant: it would give one anchor two contradictory relations from one carrier.

A third candidate fell during plan review: ADR-0047's claim on ADR-0040 Decision item 1 reads as a
retirement and the carrier's own verb is "supersedes", but item 1 bundles three clause groups and
ADR-0047 replaces only the rendered path and filename, leaving the script's download-and-verify
behaviour and its URL convention in force. ADR-0047's Context says as much: "no slug encodes the
path, so nothing retires: only the backing test's path assertion moves."

That miscall is worth recording, because it is the same one this ADR's own audit made twice. The
carrier's verb states an intent; the relation follows from what survives in the target. Reading the
verb and stopping is how a refinement gets encoded as a retirement, and a retirement is the
relation that can silently kill a live ADR through coverage. Item 1's
retirements move ADR-0009 from zero to two covered anchors of fifteen (eight Decision items,
seven declared slugs), and the two Decision items this ADR touches on it carry only refinements,
which count toward nothing, so ADR-0009 also stays far from coverage-derived supersession.
ADR-0082 goes to one covered anchor of six (four Decision items, two declared slugs). That
headroom is measured, not structural, and a future correction may complete a coverage set.

The check's first run re-measured that headroom across every ADR it touched, and none is close.
The worst case, in which all 44 findings had resolved to retirements, leaves ADR-0120 at 12
covered anchors of 25, ADR-0115 at 5 of 13, ADR-0116 at 4 of 8, and ADR-0128 at 3 of 17. The
actual verdicts add fewer still, because ADR-0120's twelve slug anchors are untouched by the
entire backfill and its ten claims all resolved to citations: nine of them invoke one of its
grammar or permission clauses as the premise of a new rule, and a carrier cannot both rely on a
clause and retire it. Item 1's three later retirements each move a different target one anchor,
so no ADR reaches coverage from this retrofit.

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

Two rules this ADR settled late must be taught in that same sweep, because both misled a reader
once already. The first is item 2's split between per-item and per-carrier claim scoping. The
second is item 1's distinction between the two back-pointer kinds: the documentation edge to the
ADR that redeclares a slug, and the edge the check enforces to the token's carrier, neither
substituting for the other. Each was stated first in only one of its two halves, and each time the
implementing plan followed the stated half and missed the other. An adopter reading a one-sided
account would make the same mistake, which is the definition of a doc-currency obligation rather
than a nicety.

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
| Add a `refines-invariant:` relation for re-scoped slugs | Withdrawn during review for want of an instance. The motivating case, ADR-0016 re-scoping ADR-0015's `provenance-banner`, is not one: that slug declares "every rendered file carries the awf generated-by banner as its first line" and names no path, so relocating the config root leaves it literally true and owes no retirement. It is not tokenless: item 4's `cites-invariant:` is what the site does take, which is the distinction the Context paragraph on it now draws. Without a decidable test separating "re-scoped but in force" from "meaning narrowed", the relation would also license the frozen-declaration-versus-moved-reality state item 1 exists to end. ADR-0128 Decision 2's retire-and-redeclare stands. The third stale slug found after review, ADR-0082's `residue-exemptions-pinned`, confirms the withdrawal rather than reopening it: "exactly two" against a proof asserting exactly three is a contradiction, not a narrowing, so it takes a retirement. Unlike the other two, whose successors were already declared elsewhere, it also takes a redeclaration here. |
| Exempt any citation inside a quotation | Rejected as undecidable: detecting quoted spans across block quotes and inline double quotes is not a structural fact the parser holds, unlike the five exemptions item 5 does adopt. It would also delete one of the two motivating cases for `cites:`, since third-party narration is quotation. Code spans alone are decidable and cover the discuss-the-grammar case. |
| Tokenize every self-trip site and add no exemption | The reviewer's preference, and close: it dogfoods hardest. Rejected because item 2's own enumeration of the eight override verbs would then need a token, which asserts a claim about anchors that the enumeration does not make. A definitional list is a mention, not a use. |
| Opt-in config key, mirroring `proseGate` | The shape fits the migration problem but breaks the drift oracle: no check in `internal/project/check.go` is config-gated today, and a key makes "`awf check` is clean" mean different things in different trees. Item 3's data-driven silence gets the same adopter experience without that cost. |
| Advisory `repoaudit` rule, as ADR-0116 sketched | Range-scoped and needing no backfill, but repo-local: adopters get no mechanism at all, and the residue this ADR measures is exactly what an ignorable channel failed to prevent. |
| Cite-anchored trigger (any anchor citation owes a token or a `cites:`) | Closes the recall gap item 2 accepts, at the price of marking every informational citation corpus-wide. ADR-0116 already judged this trigger materially less precise, and a gate that fires on ordinary cross-references gets disabled. |
| Recognize only the `ADR-NNNN Decision item N` citation shape | Measured at 59 of 186 corpus citations. The check would pass its declared invariants while missing more than two thirds of the claim space, and would not even recognize the majority of this ADR's own citations. |
| Keep moving proofs under frozen declarations, as ADR-0015 and ADR-0016 instructed | The instruction was followed and produced the defect: a green check whose declared claim is false and whose test asserts something else. Amending the declaration instead is forbidden by the append-only rule, so retirement is the only remaining coherent option. |
| Leave `config-root` and `parts-convention` alone | Preserves two prior decisions verbatim at the cost of leaving two invariants green whose proofs assert their negation. A check asserting something untrue is worse than one fewer check. |
| Extend ADR-0120 Decision 9's carve-out for relation corrections | Unnecessary: ADR-0128 Decision 4 already classifies the correction as ordinary authoring. Widening an exhaustive carve-out without need weakens the append-only rule. |
| One ADR per workstream (retirements, detector) | They share one commitment and one failure mode, and the retirements are the motivating instance for the detector. Splitting would spread one decision across two review cycles. |
