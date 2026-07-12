---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: [context-surfaces-linked-plans, context-surfaces-pitfalls]
superseded_by: ""
tags: [context, invariants, tooling, adr-system, governance]
related: [8, 31, 92, 98, 99, 102, 103]
domains: [tooling, invariants]
---
# ADR-0104: Tag-Tiered Relevance in awf context

## Context

`awf context <paths>` (ADR-0092) reflects committed `.awf/` state back to the workflow, but it
resolves ADR and pitfall relatedness by **coarse domain membership**: any file under a domain's
territory pulls *every* ADR and pitfall tagged with that domain. A three-file query returned ~90
ADRs and ~40 pitfalls — essentially the whole corpus — burying the few records that actually govern
the edited code. This is slice three (the payoff) of the `awf context` relevance rework: slice one
shipped `awf context --uncovered` (ADR-0102, the domain-coverage report), slice two built and
governed the relevance currency (ADR-0103: revived ADR `tags:`/`related:`, added pitfall `tags:`, a
governed `tags:` vocabulary, and the `awf check` governance). This slice **spends** that currency to
narrow the output.

The precise signal already exists and is unused for relevance. `ContextFor` computes
`res.Invariants = invariants.MarkersUnder(queried paths)` — the invariant slugs literally backed in
the edited files — but then surfaces ADRs by domain membership, discarding that precision. Meanwhile
`internal/invariants` already builds, inside `Check`, a `required` map of `slug → declaring
Implemented ADR` that filters to Implemented status and applies ADR-0031 retirements — exactly the
`path → present invariant slug → declaring ADR` join a precise relevance tier needs. It is private
to `Check` today; this decision exposes it.

Grounding fixed the constraints:

- **The slug→declaring-ADR join is many-to-one and must respect status/retirement.** Multiple
  Implemented ADRs re-declare a slug (e.g. `config-root`); a present slug's Tier-1 set is *all* its
  Implemented, non-retired declarers. Superseded ADRs are already excluded by the Implemented filter.
- **ADRs and pitfalls now carry `Tags` (ADR-0103); ADRs carry `Related`.** So tag-overlap and the
  `related:` graph are available in-process — no new parsing.
- **Two surfacing invariants must be reconciled, not silently broken.** `context-surfaces-pitfalls`
  (ADR-0099) mandates surfacing *every* domain-owned pitfall, and `context-surfaces-linked-plans`
  (ADR-0098) surfaces plans linked to *any reported ADR*. Tiering changes both contracts (pitfalls
  move to tag-based; the reported-ADR set narrows to the enumerated tiers). Both slugs are retired
  here (ADR-0031 `retires_invariants`) and replaced by tiered successors, rather than leaving a live
  invariant whose text the new behaviour violates.
- **`context-output-parity` (ADR-0092) must be preserved.** Both the human and `--json` renderings
  derive from one `ContextResult`; the tiered result is still one value, so the invariant stands
  unchanged (not retired).
- **Tier 3 is only meaningful once packages are owned.** ~42% of production `.go` files carry no
  invariant marker, and `internal/project` (which holds `context.go` itself) is domain-unowned — so
  a query there yields no Tier-1 signal and an empty Tier-3. This decision therefore pairs the
  consumer with a domain-coverage pass (add domains for the unowned production packages, guided by
  `awf context --uncovered`), executed in the implementation, not decided here.

## Decision

1. **Resolve `awf context <paths>` relevance in three tiers**, replacing the single coarse
   domain-membership ADR/pitfall surface. Domain resolution and `res.Invariants` (markers under the
   query) are computed as today and feed the tiers.

2. **Tier 1 — "governs this code".** The Tier-1 ADR set is exactly the Implemented, non-retired ADRs
   that declare (in their Invariants section) an invariant slug present as a marker under a queried
   path — the intersection of `res.Invariants` with the `slug → declaring Implemented ADR` join
   exposed from `internal/invariants` (the `required` map refactored into an exported function that
   `Check` and `ContextFor` share). These ADRs are enumerated individually. The **precise tag set**
   is the union of their `tags:`.

3. **Tier 2 — "topically related".** The Tier-2 set is every non-Tier-1, non-Superseded ADR that
   either shares at least one tag with the precise tag set or is named in a Tier-1 ADR's `related:`,
   plus every pitfall sharing at least one tag with the precise tag set. Each artifact appears in at
   most one tier — Tier 1 wins over Tier 2. Tier 2 is enumerated but presented compactly (see item
   6). When the precise tag set is empty (no Tier-1 ADR), Tier 2 is empty.

4. **Tier 3 — "domain background".** The ADRs surfaced by domain membership (today's coarse join)
   that fall in neither Tier 1 nor Tier 2 are reported only as a **collapsed background** — a count
   plus the owning domains' current-state doc pointers (already in `res.Domains`) — never enumerated
   individually. This removes the whole-corpus dump that motivated the rework while keeping a pointer
   to the full background.

5. **Retire the two broken surfacing invariants and redeclare tiered successors** (ADR-0031
   `retires_invariants: [context-surfaces-linked-plans, context-surfaces-pitfalls]`):
   - **Plans** surface iff their `adrs:` links a Tier-1-or-Tier-2 ADR (the enumerated set), not a
     Tier-3-collapsed one — `inv: context-surfaces-tiered-plans` replaces `context-surfaces-linked-plans`.
   - **Pitfalls** surface iff they share a tag with the precise tag set (Tier 2), not by domain
     membership — `inv: context-surfaces-tiered-pitfalls` replaces `context-surfaces-pitfalls`.

6. **Compact the presentation.** The flat, path-backed `## Invariants` block (the precise signal)
   stays; per-ADR `invariants:` echoes are dropped where they duplicate it. Tier 1 and Tier 2 are
   distinct labelled sections; Tier 3 is a one-line collapsed count. `ContextResult` is restructured
   to carry the tiers as distinct fields (preserving `context-output-parity`: both renderings derive
   from the one restructured value), and `awf context`'s read-only and static-fallback contracts
   (ADR-0092) are unchanged — the assembly reads only committed state and the command mutates
   nothing.

7. **Pair the consumer with domain coverage (execution, not a separate decision).** The
   implementation adds domain `paths:` territory for the currently-unowned production packages
   (`internal/project`, and any others `awf context --uncovered` reports), so Tier-3 background is
   populated and Tier-1 queries in those packages resolve; the exact domain assignments are an
   implementation task, guided by the ADR-0102 report.

## Invariants

Each slug below is backed by a `// invariant: <slug>` marker (comment or test) in the implementing
commit, per the backed-invariants rule (ADR-0008); `awf check` enforces them once this ADR is
`Implemented`. The two retired slugs' markers are removed in the same commit that flips this ADR.

- `inv: context-tier1-governs` — the Tier-1 ADR set reported by `awf context <paths>` is exactly the
  Implemented, non-retired ADRs declaring an invariant slug that is present as a marker under a
  queried path (the intersection of the path-present slugs with the shared `slug → declaring
  Implemented ADR` join); no Tier-1 ADR lacks such a slug and no such ADR is omitted.
- `inv: context-tier2-topical` — a non-Tier-1, non-Superseded ADR is reported in Tier 2 iff it
  shares a tag with the Tier-1 precise tag set or is named in a Tier-1 ADR's `related:`, and a
  pitfall is reported in Tier 2 iff it shares a tag with that set; every artifact appears in at most
  one tier (Tier 1 over Tier 2), and an empty precise tag set yields an empty Tier 2.
- `inv: context-tier3-collapsed` — a domain-membership ADR in neither Tier 1 nor Tier 2 is reported
  only as part of a collapsed background count with the domain current-state pointers, never as an
  individually enumerated ADR entry.
- `inv: context-surfaces-tiered-plans` — `awf context` surfaces a plan iff its `adrs:` links a
  Tier-1 or Tier-2 ADR, on the single `ContextResult`, preserving read-only / output-parity /
  static-fallback (replaces `context-surfaces-linked-plans`).
- `inv: context-surfaces-tiered-pitfalls` — `awf context` surfaces a pitfall iff it shares a tag
  with the Tier-1 precise tag set, on the single `ContextResult`, preserving read-only /
  output-parity / static-fallback (replaces `context-surfaces-pitfalls`).

## Consequences

Easier:
- The output answers "what governs the code I am editing" precisely: Tier 1 is the handful of ADRs
  whose invariants are literally backed in the queried files, not the domain's entire ADR set. The
  motivating whole-corpus dump collapses to a one-line Tier-3 pointer.
- The precise `path → invariant marker → declaring ADR → tags` join rides existing markers and the
  ADR-0103 tag currency — no new per-artifact `paths:` territory to maintain, no new parsing.
- Exposing the `slug → declaring Implemented ADR` join from `internal/invariants` removes a
  duplicated concept: `Check` and `ContextFor` share one retirement-aware join.

Harder / accepted trade-offs:
- **Recall drops deliberately.** A pitfall or ADR relevant to a domain but not sharing a precise tag
  no longer surfaces; Tier-3 collapse hides the coarse ADR set behind a count. This is the point
  (precision over recall), and the domain pointer + `--uncovered` keep the full background reachable.
- **Tier 2 can still be broad** when a Tier-1 ADR carries a high-frequency tag (e.g. `config`): tag
  overlap is finer than domain membership but not surgical. It is deliberately the *secondary* tier,
  presented compactly and clearly subordinate to Tier 1; tightening it (e.g. weighting by shared-tag
  count) is left to a future refinement rather than pre-optimised here.
- **Two Implemented invariants are retired.** `context-surfaces-pitfalls` (ADR-0099) and
  `context-surfaces-linked-plans` (ADR-0098) are retired via `retires_invariants`; their markers are
  removed in the flip commit, coupling the behaviour change to the status flip (the standard
  retirement coupling). Their sibling invariants in 0098/0099 (`pitfall-*`, `plan-*`) are untouched.
- **Domain-coverage work rides along.** Making Tier 3 meaningful requires assigning domain territory
  to unowned packages — a judgement task done in the implementation, not a mechanical consequence.
- **`internal/project` gaining a domain** means its files start surfacing that domain's background;
  the assignment must be deliberate (a `project`/`context` domain, or folding into an existing one).

Ruled out / deferred:
- **Weighted or ranked Tier 2** (shared-tag count, recency) — deferred; the tier labels carry the
  ranking for now.
- **Dropping clickable ADR paths for pure number references** — kept; the paths are clickable in the
  harness and the tiering, not path removal, is the real compaction.
- **A per-tag or per-invariant `paths:` field** — rejected in ADR-0103 and still rejected; path
  precision rides invariant markers.

Downstream work: an implementation plan covering the exported `internal/invariants` join; the
`ContextResult` tier restructure + `ContextFor` tier assembly; the `printContext`/JSON rendering of
the tiers with Tier-3 collapse; the two retired markers removed and the tiered successors backed;
the domain-coverage `paths:` additions for unowned packages; and doc currency (the AGENTS.md
invariants list — two retired bullets removed, five added — the `tooling`/`invariants` domain
current-state parts, `config-reference.md` if domains change, and a changelog `[Unreleased]` entry).
When this ADR flips to `Implemented`, the same commit regenerates `docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep domain-membership surfacing, only compact the presentation (dedupe paths, group) | Treats the symptom, not the cause — the coarse *set* is the noise; compacting a 90-ADR dump still surfaces 90 ADRs. Tiering narrows the set by precision. |
| Extend `context-surfaces-pitfalls`/`context-surfaces-linked-plans` in place rather than retire | Their text mandates the old (domain / any-reported-ADR) contract, which the new behaviour violates; leaving a live invariant whose statement is false is exactly what ADR-0031 retirement exists to prevent. Retire + redeclare is the honest move. |
| Make Tier 1 the only surface (drop Tier 2/3 entirely) | Loses genuinely useful topical and background context; the complaint was *noise*, not *any* secondary context. Tiering keeps it, ranked and collapsed. |
| Add a `paths:` field to tags or invariants for path precision | Rejected in ADR-0103 as duplicated, stale-prone territory; the invariant-marker → declaring-ADR bridge is free and self-maintaining. |
| Ship the consumer without the domain-coverage pass | Tier 3 (and Tier 1 for unowned packages like `internal/project`) would be empty for a large share of the tree, making the feature look broken exactly where it is exercised; pairing them is what makes the tiers meaningful. |
