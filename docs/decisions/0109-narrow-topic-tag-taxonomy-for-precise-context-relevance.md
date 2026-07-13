---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: [context-tier2-topical]
superseded_by: ""
tags: [context, governance]
related: [92, 99, 102, 103, 104, 106]
domains: [tooling, config, invariants]
---
# ADR-0109: Narrow-Topic Tag Taxonomy for Precise Context Relevance

## Context

`awf context <paths>` tiers relevance (ADR-0104): Tier 1 is the *governing* ADRs whose invariant
slugs are present under the queried path (precise, invariant-backed); Tier 2 is the *topically
related* ADRs and pitfalls sharing a tag with the Tier-1 "precise tag set" or named in a Tier-1
ADR's `related:`; Tier 3 is a collapsed background count. ADR-0103 built `tags:` as the currency
Tier 2 spends — explicitly "a cross-cutting keyword list, **finer than `domains:`**" — and governs it
with a closed vocabulary so it can be trusted as a relevance signal.

A run against the live corpus shows the currency has drifted back to domain scale, so Tier 2 no
longer discriminates:

- A domain-scoped query (`awf context internal/adr`) enumerates **~32 of 108** ADRs under "Related
  ADRs (shared tag)", plus ~19 pitfalls — near-total recall that buries the signal Tier 1 sharpens.
- The curated 35-member vocabulary is top-heavy and domain-parallel: `tooling` is carried by 59/108
  ADRs, `rendering` 48, `config` 32; **98 of 108 ADRs carry at least one tag equal to a configured
  domain name**. Several vocabulary members (`tooling`, `rendering`, `config`, `invariants`,
  `adr-system`, `context`, `docs`, `audit`, …) are literally domain names.
- ADR-0104's mitigation — dropping domain-named tags from the precise set (`context.go` "union of
  Tier-1 tags minus any tag naming a configured domain") — is silently carrying almost the whole
  corpus: even after it removes `tooling`/`rendering`/`config`, the surviving precise tags are still
  broad (`docs` on 16 ADRs, `workflow` on 14), so a two-ADR Tier-1 set pulls ~30 ADRs into Tier 2.

The breadth is not a few historically "mixed" early ADRs — early ADRs (001–030) average 2.47 tags
each, recent ones (079–108) average 3.57. The driver is the *vocabulary being domain-scale*, applied
liberally across the whole corpus. The tags never became "specific topics in a domain"; they became a
pool of catch-all terms.

Grounding fixed the boundaries this decision must respect:

- **Pitfalls have no other relevance bridge.** A pitfall carries no invariant marker, so it can never
  reach Tier 1; its *only* path to relevance is a shared tag (`context-surfaces-tiered-pitfalls`).
  Any fix that merely demotes shared-tag ADRs to a collapsed Tier 3 would delete pitfalls from useful
  output. The fix must restore tag **precision**, not abandon tags.
- **Tier 2 has a second, tag-independent selector.** Selection is `sharesTag(a.Tags, precise) ||
  relatedNum[n]` (`internal/project/context.go`), where `relatedNum` is built from the Tier-1 ADRs'
  `related:` links. Narrowing tags does not touch the `related:`-link tail — but that tail is small
  (an ADR names a handful of `related:` ADRs) and hand-curated, i.e. it is the *intended* precise
  adjacency, not the breadth problem.
- **Re-tagging is a data edit, not a migration.** `adr.ADR` already parses `Tags []string` /
  `Related []int` and `pitfallEntry` already parses `Tags []string`; consuming them needs no code
  change, and re-tagging awf's own corpus is a one-time in-repo edit governed by the existing
  vocabulary check — never a schema migration (`internal/migrate` is adopter-facing; ADR-0103
  precedent). A new optional, absent-safe surface needs no schema-generation bump (ADR-0049).
- **Changing an Implemented invariant's meaning needs a slug rename.** `DeclaringADRs` rejects a
  duplicate slug in its first pass, before retirements are applied in the second — so a successor ADR
  cannot retire-and-redeclare `context-tier2-topical` under the same name while ADR-0104 stays
  Implemented. The precise-set clause changes meaning (no more domain-name filtering), so this ADR
  retires that slug and declares a renamed successor (mechanism: ADR-0031).
- **The advisory note is a new producer.** `awf check`'s existing non-failing `note:` channel is
  computed from a render pass (unset-var / stub / part-marker); tag-frequency and tag-coverage fit
  neither it nor the vocabulary check's hard `Drift`, so the tag-health note is a distinct producer
  that happens to print through the same `note:` prefix.

## Decision

1. **Redefine the tag standard as a sub-domain topic.** A tag names the specific mechanism or concern
   a decision turns on — `placeholder-degradation`, `section-assembly`, `lock-atomicity`,
   `closure-planning`, `marker-backing` — not a domain-scale bucket, and **never a configured domain
   name**. Target granularity: each tag lands on a genuine cluster (roughly 2–10 artifacts). A
   single-artifact tag is permitted — it is simply inert for Tier 2 — and is preferable to an
   over-broad one. A "mixed" ADR carries several *narrow* tags, one per concern; ADRs stay
   append-only, so accurate multi-tagging, not splitting, captures its concerns.

2. **Re-curate the vocabulary and re-tag awf's corpus in the implementing commit(s).** `.awf/config.yaml`
   `tags:` is re-curated to a narrow-topic set (expected ~60–90 members, each with its one-line
   meaning), and awf's ~108 ADR frontmatters and ~46 pitfall entries are re-tagged to it — a one-time
   in-repo data edit governed by the existing `awf check` vocabulary rule, **not** a schema migration.
   The curation is quality-first (tight, meaningful topics), not a mechanical union of prior labels.
   The five current vocabulary members that name domains (`adr-system`, `config`, `invariants`,
   `rendering`, `tooling`) are removed and every artifact using them re-tagged in the *same* commit,
   so item 3's gate lands green rather than flagging the pre-existing corpus; ADR-0109's own
   frontmatter is re-tagged with it.

3. **Gate the standard's hard floor.** When the `tags:` vocabulary is non-empty, `awf check` fails if
   any vocabulary member name equals a configured domain name (added to the existing
   `checkTagVocabulary` membership check). This encodes "finer than domains" as an exact,
   non-fuzzy rule and mechanically prevents the coarse-tag regression. It is inert for a project with
   no domains or an empty vocabulary.

4. **Add a non-failing tag-health note to `awf check`** (a new note producer, distinct from the
   render-pass advisory channel and from the vocabulary `Drift`):
   - **Frequency:** each vocabulary tag carried by more than a fixed default share — **25% of the
     artifacts carrying at least one vocabulary tag** — yields a note that the tag is coarsening. Its
     job is *prospective*: after the re-tag no tag should approach 25%, so the note stays quiet on a
     healthy corpus and fires only when a topic tag later drifts toward domain scale — the
     broad-but-not-domain-named coarsening item 3's exact gate cannot express. (On today's
     un-re-tagged corpus of 155 tag-bearing artifacts only `tooling` at 48% and `rendering` at 39%
     cross the line, and both are domain names item 3 already rejects — so the note earns its keep
     going forward, not on the present state.) The 25% default is a documented constant, no new
     config key.
   - **Coverage:** each ADR or pitfall carrying **zero** tags yields a note, the backstop against
     silent under-tagging across the hand re-tag. The whole tag-health producer is inert under an
     empty/absent vocabulary, so an un-curated adopter — and the example — stays note-free; a
     domain-named tag is never surfaced by the coverage note, because under a governed vocabulary it
     is already a hard failure (item 3's gate or `tag-vocabulary-governed`) and under an empty
     vocabulary the producer does not run.

   Both notes are advisory and never change the exit code.

5. **Simplify Tier 2 now that tags cannot name domains.** The precise tag set becomes exactly the
   union of the Tier-1 ADRs' tags, with **no** domain-name filtering (the filtering block in
   `context.go` becomes provably inert under item 3 and is removed). `context-tier2-topical` is
   retired and a renamed successor invariant carries the simplified rule; the retained
   `related:`-link tail (the deliberate curated adjacency) and the "at most one tier" property are
   unchanged. The two proof markers (`context.go` and `context_test.go`) move to the new slug in the implementing commit — and the
   existing `TestContextForAssembles` domain-mirror-*exclusion* assertion, inverted once the filter is
   gone, is re-purposed into a `tag-not-domain-name` gate test rather than carried stale onto the new
   slug. The two sibling ADR-0104 invariants that reference the precise set need **no** rename: once
   item 3 bans domain-name tags, "union of Tier-1 tags minus any tag naming a domain" is identically
   "union of Tier-1 tags", so `context-surfaces-tiered-pitfalls`'s observable contract is unchanged;
   `context-surfaces-tiered-plans` references tier *membership*, not the precise set, and is likewise
   stable.

6. **Resync the rendered surfaces citing the Tier-2 wording.** The agent guide's invariant list
   (sourced from `.awf/agents-doc.yaml`) is updated — the Tier-2 bullet renamed to
   `context-tier2-precise-tag`, and three new bullets added for `tag-not-domain-name`,
   `tag-frequency-note`, and `tag-coverage-note` (the list is hand-maintained with no gate) — and
   re-synced in the same change (docs travel with the change), alongside the `./x sync` regeneration
   of `docs/decisions/ACTIVE.md` at the eventual Proposed→Implemented status flip.

## Invariants

Each slug below is backed by a `// invariant: <slug>` proof marker on a test in the implementing
commit, per the backed-invariants rule (ADR-0008); `awf check` enforces them once this ADR is
`Implemented`. The retired slug's two proof markers (`internal/project/context.go` and
`internal/project/context_test.go`) are removed and re-homed to the renamed successor in the same
commit.

- `` `invariant: tag-not-domain-name` `` — with a non-empty `tags:` vocabulary and a non-empty
  `domains:` set, `awf check` fails iff some vocabulary member name equals a configured domain name;
  an empty vocabulary or a project with no configured domains is inert (no finding).
- `` `invariant: tag-frequency-note` `` — `awf check` emits a non-failing `note:` for each vocabulary
  tag carried by strictly more than 25% of the artifacts carrying at least one vocabulary tag (the
  denominator; the numerator is the artifacts carrying that tag, counting ADRs and pitfalls alike),
  and for no tag at or below that share; the finding never changes the exit code.
- `` `invariant: tag-coverage-note` `` — under a non-empty vocabulary, `awf check` emits a non-failing
  `note:` for each ADR and each pitfall carrying zero tags, and for no tagged artifact; the finding
  never changes the exit code, and an empty/absent vocabulary is inert.
- `` `invariant: context-tier2-precise-tag` `` (replaces `context-tier2-topical`) — the precise tag
  set reported by `awf context <paths>` is exactly the union of the Tier-1 ADRs' tags, with no
  domain-name filtering; a non-Tier-1, non-Superseded ADR is reported in Tier 2 iff it shares a tag
  with the precise set or is named in a Tier-1 ADR's `related:`, and a pitfall is reported in Tier 2
  iff it shares a tag with the precise set; every artifact appears in at most one tier (Tier 1 over
  Tier 2). An empty precise set yields a Tier 2 of only the Tier-1 ADRs' `related:` links.

## Consequences

- **Tier 2 and pitfall surfacing become precise for both artifact kinds.** A domain-scoped query
  returns a tight cluster of genuinely-adjacent ADRs and pitfalls instead of a third of the corpus,
  restoring ADR-0103's "finer than domains" intent that ADR-0104's spend could not realise over a
  coarse vocabulary.
- **The `related:`-link tail is retained by design.** Tier 2 still lists a Tier-1 ADR's hand-curated
  `related:` ADRs (typically a handful); this is intentional precise adjacency, not breadth, and is
  called out so it is not mistaken for a regression.
- **The regression is fenced on two levels.** The exact `tag ≠ domain name` gate blocks the worst
  coarsening deterministically; the advisory frequency note catches the subtler
  broad-but-not-domain-named drift the gate cannot express as a hard rule.
- **Re-tagging ~154 artifacts by hand is the main cost.** The coverage note backstops *omission*
  (a zero-tagged artifact), but no check can judge whether a chosen narrow tag is the
  *right* topic — that remains authoring and review discipline.
- **The frequency note is corpus-size-sensitive.** A fixed percentage default keeps awf's own output
  quiet after re-tagging and loud before it, but a very small adopter corpus may see false frequency
  notes; because the note is advisory this is tolerable, and making the threshold configurable is
  deferred to the future housekeeping surface rather than adding a config key now.
- **This is a partial-item refinement of ADR-0104, not a supersession.** ADR-0104's tiering, Tier-1
  governance, Tier-3 collapse, and plan/pitfall surfacing otherwise stand; only the precise-set clause
  changes, via one retired-and-renamed invariant. ADR-0103's vocabulary machinery and governance are
  unchanged and extended by item 3.
- **Adopters inherit the standard and the guards, publication-safe.** The gate and both notes degrade
  to inert when the vocabulary or the domain set is empty; adopters curate their own narrow vocabulary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Move shared-tag adjacency to a collapsed Tier 3, keep coarse tags | Pitfalls have no other relevance bridge, so it would delete them from useful output; and it leaves the mis-curated domain-scale vocabulary in place, treating the symptom not the cause. |
| Require ≥2 shared tags / a Jaccard threshold in Tier 2 | A fuzzy relevance mechanic layered over a mis-curated currency; once tags are narrow, a single shared tag is already a strong signal, with no magic number to tune. |
| Make tag frequency a hard gate | The threshold is a judgment call and adopter corpora differ wildly; only the exact `tag = domain name` floor is crisp enough to gate — the rest belongs in the advisory tier. |
| Split the historically "mixed" early ADRs into single-concern records | ADRs are append-only; accurate narrow multi-tagging captures a mixed ADR's concerns without rewriting a settled decision. |
