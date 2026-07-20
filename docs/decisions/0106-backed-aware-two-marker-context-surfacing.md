---
status: Implemented
date: 2026-07-13
tags: [context-tiering, invariant-backing]
related: [8, 31, 92, 104, 105, 134]
domains: [tooling, invariants]
---
# ADR-0106: Backed-Aware Two-Marker Context Surfacing

## Context

ADR-0104 resolves `awf context <paths>` relevance in three tiers. Tier 1 ("governs this code")
is the Implemented, non-retired ADRs declaring an invariant slug **present as a marker under a
queried path**: `res.Invariants` from `invariants.MarkersUnder`, intersected with the
`slug → declaring ADR` join (`inv: context-tier1-governs`). `MarkersUnder` today scans only
`invariants.sources` and recognises the single `invariant: <slug>` marker.

ADR-0105 changes the marker model underneath that:

- Backing markers split into a **proof** marker (`invariant:`, scoped to `testGlobs` test files) and
  a **touches** marker (`touches-invariant: <slug> - <note>`, on production code, the site-level
  context payload). A backed invariant's proof lives in a test file, so a query on the *production*
  code it governs would find no marker under that path: the invariant would silently drop out of
  Tier-1 governance. `MarkersUnder` must therefore scan the **union** of `sources` and `testGlobs`
  and recognise **both** marker kinds as "present under a path," or splitting the globs narrows the
  context signal (a gap ADR-0105 flags in its Consequences).
- Invariants are explicitly classified **backed** (test-proven; a break auto-triggers a test) or
  **unbacked** (a reasoned contract carrying a `Verify:` note; nothing fails automatically). An agent
  reading context before editing needs that distinction: it tells it which governing invariants are
  guarded and which it must reason about itself. Context becomes a **risk map** only if it surfaces
  the class.
- The site-level notes (a touches marker's `- <note>` and an unbacked invariant's `Verify:`
  guidance) are exactly the "how does this site relate / how do I check this" information a query
  should return.

This ADR amends ADR-0104's Tier-1 surfacing for the two-marker model and adds the backed/unbacked
labelling and note surfacing. The Tier-2 (`context-tier2-topical`) and Tier-3
(`context-tier3-collapsed`) rules are unchanged: they derive from the Tier-1 set, whose membership
shifts but whose downstream rules do not. ADR-0104 stays `Implemented`; this ADR carries
`related: [104]` and retires the one invariant whose meaning changes.

## Decision

1. **Union scan, both markers feed Tier-1.** `invariants.MarkersUnder` scans the union of the
   `invariants.sources` globs and the `testGlobs` globs, and treats a file as marking a slug under a
   path when it carries **either** a proof `invariant: <slug>` **or** a `touches-invariant: <slug>`
   marker. Tier-1 governance is otherwise unchanged (the intersection with the shared
   `slug → declaring Implemented ADR` join). This **retires `context-tier1-governs`** (ADR-0031,
   `retires_invariants: [context-tier1-governs]`) and redeclares the union-aware successor
   `inv: context-tier1-marker-union`, because "present as a marker under a path" now spans two marker
   kinds and two glob sets.

2. **Label each governing invariant backed vs. unbacked.** Every invariant surfaced as governing a
   queried path is labelled from its declaring ADR's ADR-0105 classification: **backed** (declared
   ``invariant:``) or **unbacked** (declared ``unbacked-invariant:``). The flat path-backed
   `## Invariants` block (ADR-0104 item 6) annotates each slug with its class, so the agent sees at a
   glance which governing invariants auto-trigger on breakage and which need manual reasoning.

3. **Surface the site-level notes.** For an unbacked governing invariant, `awf context` surfaces its
   `Verify:` guidance; for a `touches-invariant:` marker under a queried path, it surfaces the
   marker's `- <note>`. Both ride the single assembled `ContextResult`, preserving `awf context`'s
   read-only, output-parity, and static-fallback contracts (ADR-0092, ADR-0104): the human and
   `--json` renderings derive from the one value, and the command mutates nothing.

4. **Retirement bookkeeping (migrated from retires_invariants by awf upgrade,
   ADR-0120).** This ADR retires `supersedes-invariant: ADR-0104#context-tier1-governs`.

## Invariants

Each slug is backed under the ADR-0105 model in the implementing commit. `awf check` enforces them
once this ADR is `Implemented`; the retired slug's marker is removed in the same commit.

- `invariant: context-tier1-marker-union`: the Tier-1 governing set reported by `awf context <paths>` is
  exactly the Implemented, non-retired ADRs declaring an invariant slug present under a queried path,
  where presence derives from the union scan of `invariants.sources` and `testGlobs` recognising both
  the proof `invariant:` and the `touches-invariant:` marker (intersected with the shared
  `slug → declaring ADR` join). Replaces `context-tier1-governs`.
- `invariant: context-invariant-backed-labeled`: every invariant `awf context` reports as governing a
  queried path is labelled backed or unbacked according to its declaring ADR's ADR-0105
  classification; no governing invariant is reported without a class.
- `invariant: context-surfaces-marker-notes`: `awf context` carries, on the single `ContextResult`, each
  surfaced unbacked governing invariant's `Verify:` note and each under-path `touches-invariant:`
  marker's site note, preserving read-only / output-parity / static-fallback.

## Consequences

- **Context is a risk map.** An editing agent sees which governing invariants are test-guarded and
  which it must reason about, with the `Verify:` and touches notes explaining how: the payoff of
  ADR-0105's explicit classification.
- **No production-code blind spot.** Because the union scan and both markers feed Tier-1, querying
  production code surfaces the invariants that govern it even when the proof marker lives in a test
  file: the gap ADR-0105 identified.
- **Depends on ADR-0105.** This ADR is inert until the two-marker model, `testGlobs`, and the
  classification exist; it lands in the same effort, after ADR-0105's mechanism. The plan(s) sequence
  the two, and a plan↔ADR resync reconciles this ADR against the finalised ADR-0105 before
  implementation.
- **`ContextResult` widens.** It carries a per-invariant class and the site notes; both renderings
  (human, `--json`) derive from the one value, preserving `context-output-parity`. `MarkersUnder`
  (and any `res.Invariants` consumer) changes signature/return to carry the marker kind and note.
- **Tier-2/Tier-3 unchanged.** `context-tier2-topical` and `context-tier3-collapsed` are unaffected:
  their rules derive from the Tier-1 set, whose membership may grow (more markers → more path-present
  slugs) but whose downstream computation is identical.
- **Doc-currency obligations (same landing commit).** Regenerate `docs/decisions/ACTIVE.md` on the
  `Implemented` flip; update the `docs/domains/tooling.md`/`docs/domains/invariants.md` narratives for
  the union scan and the backed/unbacked context labelling; the AGENTS.md `context` invariant bullets
  (rendered from `.awf/`) gain the `context-tier1-marker-union` wording and drop
  `context-tier1-governs`; add a changelog `[Unreleased]` entry for the user-facing `awf context`
  changes (the risk-map backed/unbacked labels, the surfaced `Verify:`/touches notes, and the
  `--json` shape carrying per-invariant class and notes), folded into ADR-0105's joint `[Unreleased]`
  entry, since the two land together.
- **New branches to cover.** union-scan hit via proof-only, via touches-only, and via both; a backed
  vs. an unbacked governing invariant; an unbacked invariant's `Verify:` surfaced; a touches note
  surfaced; and the static-fallback path: each needs an explicit test under the 100% gate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Only proof markers feed Tier-1 governance | Querying production code would miss invariants whose proof lives in a test file: the exact blind spot this effort closes; touches markers carry the production-site signal. |
| Amend `context-tier1-governs` in place (keep the slug) | Its meaning changes (two marker kinds, two glob sets); redeclaring a union-aware successor via the retirement mechanism mirrors ADR-0104's own handling of its predecessors and keeps one slug = one meaning. |
| A separate "touches" context tier below Tier-1 | The backed/unbacked label already carries the strength distinction within Tier-1; a fourth tier duplicates that and fragments the governing set the agent needs in one place. |
| Fold this into ADR-0105 | Couples a context-relevance decision to the backing-model decision; kept separate, one decision per ADR, per the brainstorm's two-ADR split. |

## Migration history

- 2026-07-13: retired invariant `ADR-0104#context-tier1-governs`; basis: encoded
