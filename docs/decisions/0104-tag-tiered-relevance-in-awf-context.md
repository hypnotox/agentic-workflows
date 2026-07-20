---
status: Implemented
date: 2026-07-13
tags: [context-tiering]
related: [8, 31, 92, 98, 99, 102, 103, 106, 109, 134]
domains: [tooling, invariants]
---
# ADR-0104: Tag-Tiered Relevance in awf context

## Context

`awf context <paths>` (ADR-0092) reflects committed `.awf/` state back to the workflow, but it
resolves ADR and pitfall relatedness by **coarse domain membership**: any file under a domain's
territory pulls *every* ADR and pitfall tagged with that domain. A three-file query returned ~90
ADRs and ~40 pitfalls (essentially the whole corpus) burying the few records that actually govern
the edited code. This is slice three (the payoff) of the `awf context` relevance rework: slice one
shipped `awf context --uncovered` (ADR-0102, the domain-coverage report), slice two built and
governed the relevance currency (ADR-0103: revived ADR `tags:`/`related:`, added pitfall `tags:`, a
governed `tags:` vocabulary, and the `awf check` governance). This slice **spends** that currency to
narrow the output.

The precise signal already exists and is unused for relevance. `ContextFor` computes
`res.Invariants = invariants.MarkersUnder(queried paths)` (the invariant slugs literally backed in
the edited files) but then surfaces ADRs by domain membership, discarding that precision. Meanwhile
`internal/invariants` already builds, inside `Check`, a `required` map of `slug → declaring
Implemented ADR` that filters to Implemented status and applies ADR-0031 retirements: exactly the
`path → present invariant slug → declaring ADR` join a precise relevance tier needs. It is private
to `Check` today; this decision exposes it.

Grounding fixed the constraints:

- **The slug→declaring-ADR join is one-to-one and must respect status/retirement.** `Check` refuses
  two Implemented ADRs declaring the same slug (`invariants.go` "duplicate inv slug"), so a green
  gate guarantees each slug has exactly one Implemented, non-retired declarer; re-declaration only
  ever crosses a supersession boundary (the predecessor is no longer Implemented). The join is thus
  `slug → single declaring Implemented ADR`, and a query's Tier-1 *set* still holds several ADRs
  because several distinct present slugs map to distinct declarers.
- **ADRs and pitfalls now carry `Tags` (ADR-0103); ADRs carry `Related`.** So tag-overlap and the
  `related:` graph are available in-process: no new parsing.
- **Two surfacing invariants must be reconciled, not silently broken.** `context-surfaces-pitfalls`
  (ADR-0099) mandates surfacing *every* domain-owned pitfall, and `context-surfaces-linked-plans`
  (ADR-0098) surfaces plans linked to *any reported ADR*. Tiering changes both contracts (pitfalls
  move to tag-based; the reported-ADR set narrows to the enumerated tiers). Both slugs are retired
  here (ADR-0031 `retires_invariants`) and replaced by tiered successors, rather than leaving a live
  invariant whose text the new behaviour violates.
- **`context-output-parity` (ADR-0092) must be preserved.** Both the human and `--json` renderings
  derive from one `ContextResult`; the tiered result is still one value, so the invariant stands
  unchanged (not retired).
- **Tier 1 does not depend on domain ownership; Tier 3 does.** Tier 1 rides invariant markers, which
  live in source files regardless of whether a domain claims them, so a query in the domain-unowned
  `internal/project` (which holds `context.go` itself) still resolves Tier 1 from its `context-*`
  markers. Tier 3 (domain background) is simply empty for an unowned package, which is *honest*, not
  broken: the query has no domain, so there is no background to collapse. Expanding domain territory
  to the unowned production packages would *populate* Tier 3 there, but it is an improvement, not a
  correctness precondition; this ADR records it as a recommended follow-up (Consequences), not a
  bundled decision, so no domain-model choice is owed here.

## Decision

1. **Resolve `awf context <paths>` relevance in three tiers**, replacing the single coarse
   domain-membership ADR/pitfall surface. Domain resolution and `res.Invariants` (markers under the
   query) are computed as today and feed the tiers.

2. **Tier 1: "governs this code".** The Tier-1 ADR set is exactly the Implemented, non-retired ADRs
   that declare (in their Invariants section) an invariant slug present as a marker under a queried
   path: the intersection of `res.Invariants` with the `slug → declaring Implemented ADR` join
   exposed from `internal/invariants` (the `required` map refactored into an exported function that
   `Check` and `ContextFor` share). These ADRs are enumerated individually. The **precise tag set**
   is the union of their `tags:` **minus any tag that names a configured domain**: a domain-mirror
   tag (`tooling`, `rendering`, `config`, `adr-system`, `invariants` here) carries only domain-level
   relatedness, which Tier 3 already represents, so excluding it keeps Tier 2 to the finer,
   cross-cutting tags the domain axis cannot express. The precise tag set is empty when every Tier-1
   tag is a domain-mirror.

3. **Tier 2: "topically related".** The Tier-2 set is every non-Tier-1, non-Superseded ADR that
   either shares at least one tag with the precise tag set or is named in a Tier-1 ADR's `related:`,
   plus every pitfall sharing at least one tag with the precise tag set. Each artifact appears in at
   most one tier: Tier 1 wins over Tier 2. Tier 2 is enumerated but presented compactly (see item
   6). Because the precise set excludes domain-mirror tags (item 2), Tier-2 tag matches are on the
   finer cross-cutting tags only: a query governed by a `tooling`-tagged ADR does not pull every
   `tooling` ADR into Tier 2. When the precise tag set is empty (no Tier-1 ADR, or all Tier-1 tags
   are domain-mirrors), the only Tier-2 members are the `related:`-linked ADRs of any Tier-1 ADR.

4. **Tier 3: "domain background".** The ADRs surfaced by domain membership (today's coarse join)
   that fall in neither Tier 1 nor Tier 2 are reported only as a **collapsed background** (a count
   plus the owning domains' current-state doc pointers (already in `res.Domains`)), never enumerated
   individually. This removes the whole-corpus dump that motivated the rework while keeping a pointer
   to the full background.

5. **Retire the two broken surfacing invariants and redeclare tiered successors** (ADR-0031
   `retires_invariants: [context-surfaces-linked-plans, context-surfaces-pitfalls]`):
   - **Plans** surface iff their `adrs:` links a Tier-1-or-Tier-2 ADR (the enumerated set), not a
     Tier-3-collapsed one; `inv: context-surfaces-tiered-plans` replaces `context-surfaces-linked-plans`.
   - **Pitfalls** surface iff they share a tag with the precise tag set (Tier 2), not by domain
     membership; `inv: context-surfaces-tiered-pitfalls` replaces `context-surfaces-pitfalls`.

6. **Compact the presentation.** The flat, path-backed `## Invariants` block (the precise signal)
   stays; per-ADR `invariants:` echoes are dropped where they duplicate it. Tier 1 and Tier 2 are
   distinct labelled sections; Tier 3 is a one-line collapsed count. `ContextResult` is restructured
   to carry the tiers as distinct fields (preserving `context-output-parity`: both renderings derive
   from the one restructured value), and `awf context`'s read-only and static-fallback contracts
   (ADR-0092) are unchanged: the assembly reads only committed state and the command mutates
   nothing.

7. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,
   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0098#context-surfaces-linked-plans`, `supersedes-invariant: ADR-0099#context-surfaces-pitfalls`.

## Invariants

Each slug below is backed by a `// invariant: <slug>` marker (comment or test) in the implementing
commit, per the backed-invariants rule (ADR-0008); `awf check` enforces them once this ADR is
`Implemented`. The two retired slugs' markers are removed in the same commit that flips this ADR.

- `invariant: context-tier1-governs`: the Tier-1 ADR set reported by `awf context <paths>` is exactly the
  Implemented, non-retired ADRs declaring an invariant slug that is present as a marker under a
  queried path (the intersection of the path-present slugs with the shared `slug → declaring
  Implemented ADR` join); no Tier-1 ADR lacks such a slug and no such ADR is omitted.
- `invariant: context-tier2-topical`: the precise tag set is the union of the Tier-1 ADRs' tags with every
  tag that names a configured domain removed; a non-Tier-1, non-Superseded ADR is reported in Tier 2
  iff it shares a tag with that precise set or is named in a Tier-1 ADR's `related:`, and a pitfall
  is reported in Tier 2 iff it shares a tag with that set; every artifact appears in at most one tier
  (Tier 1 over Tier 2). An empty precise set yields a Tier 2 of only the Tier-1 ADRs' `related:`
  links.
- `invariant: context-tier3-collapsed`: a domain-membership ADR in neither Tier 1 nor Tier 2 is reported
  only as part of a collapsed background count with the domain current-state pointers, never as an
  individually enumerated ADR entry.
- `invariant: context-surfaces-tiered-plans`: `awf context` surfaces a plan iff its `adrs:` links a
  Tier-1 or Tier-2 ADR, on the single `ContextResult`, preserving read-only / output-parity /
  static-fallback (replaces `context-surfaces-linked-plans`).
- `invariant: context-surfaces-tiered-pitfalls`: `awf context` surfaces a pitfall iff it shares a tag
  with the Tier-1 precise tag set, on the single `ContextResult`, preserving read-only /
  output-parity / static-fallback (replaces `context-surfaces-pitfalls`).

## Consequences

Easier:
- The output answers "what governs the code I am editing" precisely: Tier 1 is the handful of ADRs
  whose invariants are literally backed in the queried files, not the domain's entire ADR set. The
  motivating whole-corpus dump collapses to a one-line Tier-3 pointer.
- The precise `path → invariant marker → declaring ADR → tags` join rides existing markers and the
  ADR-0103 tag currency: no new per-artifact `paths:` territory to maintain, no new parsing.
- Exposing the `slug → declaring Implemented ADR` join from `internal/invariants` removes a
  duplicated concept: `Check` and `ContextFor` share one retirement-aware join.

Harder / accepted trade-offs:
- **Recall drops deliberately.** A pitfall or ADR relevant to a domain but not sharing a precise tag
  no longer surfaces; Tier-3 collapse hides the coarse ADR set behind a count. This is the point
  (precision over recall), and the domain pointer + `--uncovered` keep the full background reachable.
- **Tier 2 breadth is bounded by excluding domain-mirror tags, not by display.** Without that
  exclusion the precise set would include high-frequency tags (`tooling` on ~58 ADRs), and Tier 2
  would re-materialize the whole-corpus dump merely relabelled: the common case, not an edge, for
  the tooling-heavy packages. Item 2's domain-mirror exclusion narrows the *set* (not just the
  display) to the finer cross-cutting tags, so Tier 2 stays proportionate. A residual risk remains if
  a genuinely cross-cutting tag itself grows high-frequency; further narrowing (weight by shared-tag
  count, require ≥2 shared tags) is a deliberate future refinement, not pre-optimised here.
- **Two Implemented invariants are retired.** `context-surfaces-pitfalls` (ADR-0099) and
  `context-surfaces-linked-plans` (ADR-0098) are retired via `retires_invariants`; their markers are
  removed in the flip commit, coupling the behaviour change to the status flip (the standard
  retirement coupling). Their sibling invariants in 0098/0099 (`pitfall-*`, `plan-*`) are untouched.
- **Recommended follow-up: domain coverage for unowned packages.** Tier 1 works without domain
  ownership (it rides markers), so this ADR does not require it; but expanding domain territory to the
  currently-unowned production packages (`internal/project` and any others `awf context --uncovered`
  reports) would populate their Tier-3 background. That is a separate, load-bearing choice (a new
  domain vs folding into an existing one, with its own current-state part, config-reference row, and
  ADR-0077 staleness territory) and is deliberately left to a follow-up effort rather than
  under-decided here.

Ruled out / deferred:
- **Weighted or ranked Tier 2** (shared-tag count, recency): deferred; the tier labels carry the
  ranking for now.
- **Dropping clickable ADR paths for pure number references**: kept; the paths are clickable in the
  harness and the tiering, not path removal, is the real compaction.
- **A per-tag or per-invariant `paths:` field**: rejected in ADR-0103 and still rejected; path
  precision rides invariant markers.

Downstream work: an implementation plan covering the exported `internal/invariants` join; the
`ContextResult` tier restructure + `ContextFor` tier assembly; the `printContext`/JSON rendering of
the tiers with Tier-3 collapse; the two retired markers removed and the tiered successors backed;
and doc currency (the AGENTS.md invariants list (two retired bullets removed, five added), the
`tooling`/`invariants` domain current-state parts and a changelog `[Unreleased]` entry). The
domain-coverage expansion for unowned packages is a separate follow-up (Consequences), not part of
this plan.
When this ADR flips to `Implemented`, the same commit regenerates `docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep domain-membership surfacing, only compact the presentation (dedupe paths, group) | Treats the symptom, not the cause: the coarse *set* is the noise; compacting a 90-ADR dump still surfaces 90 ADRs. Tiering narrows the set by precision. |
| Extend `context-surfaces-pitfalls`/`context-surfaces-linked-plans` in place rather than retire | Their text mandates the old (domain / any-reported-ADR) contract, which the new behaviour violates; leaving a live invariant whose statement is false is exactly what ADR-0031 retirement exists to prevent. Retire + redeclare is the honest move. |
| Make Tier 1 the only surface (drop Tier 2/3 entirely) | Loses genuinely useful topical and background context; the complaint was *noise*, not *any* secondary context. Tiering keeps it, ranked and collapsed. |
| Add a `paths:` field to tags or invariants for path precision | Rejected in ADR-0103 as duplicated, stale-prone territory; the invariant-marker → declaring-ADR bridge is free and self-maintaining. |
| Include Tier-1 tags verbatim (no domain-mirror exclusion) in the precise set | A domain-mirror tag like `tooling` (~58 ADRs) would flood Tier 2 with the whole coarse set: the dump this rework removes, relabelled. Domain-level relatedness is Tier 3's job; Tier 2 uses the finer tags. |
| Bundle the domain-coverage expansion into this ADR as a decision | It is a distinct load-bearing choice (new domain vs fold-in, with its own territory/staleness/currency) on a different rationale (coverage, not noise); Tier 1 works without it and empty Tier 3 for an unowned package is honest, so it is a follow-up, not a bundled sub-decision. |

## Migration history

- 2026-07-13: retired invariant `ADR-0098#context-surfaces-linked-plans`; basis: encoded
- 2026-07-13: retired invariant `ADR-0099#context-surfaces-pitfalls`; basis: encoded
