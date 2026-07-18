---
status: Proposed
date: 2026-07-18
tags: [adr-parsing, adr-lifecycle, active-md, domain-index, context-tiering]
related: [14, 92, 104, 116, 120, 128]
domains: [adr-system, rendering]
supersedes: []
superseded_by: ""
---
# ADR-0129: Single Anchor-Coverage Model for Every Supersession Consumer

## Context

Supersession is read four independent ways today, and no two of them agree on what they are
reading.

- `bucketKey` (`internal/adr/adr.go:27-32`) folds any status with the `Superseded` prefix into
  one ACTIVE.md section. It is a string-prefix test over a prose field.
- The per-domain ADR index (`internal/adr/domain.go:38-41`) prints a successor arrow from the
  `SupersededBy` scalar and nothing else.
- `awf context`'s Tier-2 exclusion (`internal/project/context.go:189`) is a second, separate
  `strings.HasPrefix(a.Status, "Superseded")`.
- `SupersessionIndex` (`internal/adr/adr.go:117-156`) is the only place the two supersession
  flavours meet at all, and it does not unify them: it returns full chains built from the
  `Supersedes` frontmatter list and a map of per-target overrides built from inline tokens,
  two values with no shared type, computed in one pass because they happen to need the same
  loop.

Two consequences follow from that fragmentation, one already shipped as a defect.

**The domain indexes are blind to partial supersession.** `domain.go` reads only
`SupersededBy`, so an ADR whose Decision items have been overridden renders as pristine in
every domain doc it belongs to. With 55 partial anchors across 32 target ADRs and only 3 full
pairs in the corpus, the indexes are silent about the overwhelming majority of supersession
that exists. A reader who reaches an ADR through `docs/domains/<domain>.md` rather than
ACTIVE.md gets no signal at all.

**ADR-0128 removes the inputs two of these readings depend on.** Deleting `supersedes:` and
`superseded_by:` leaves `domain.go`'s scalar read with nothing to read and the chain half of
`SupersessionIndex` with nothing to build from, and it makes full supersession a *computed*
property of anchor coverage that no structure in the codebase represents. ADR-0128 also had to
drop its own acyclicity item during review for exactly this reason: it declared a check over an
"anchor-coverage graph" that nothing defined, because the model was deferred here.

The shape the domain needs is not a tree. An ADR may be claimed by several successors, and one
successor may claim anchors in several predecessors, so parents multiply in both directions.
Modelling it as a tree would reintroduce the single-scalar-claimant assumption ADR-0128 item 4
deleted on purpose.

## Decision

1. **One computed model, built once per corpus, is the sole source of supersession facts.** A
   package-level constructor takes the parsed `[]ADR` and returns a value that answers every
   supersession question the tooling asks. No consumer re-derives supersession by reading a
   status string, a frontmatter field, or an ADR's refs directly, save the single exception
   item 4 enumerates.

2. **Nodes are anchors, edges are claims.** A node is a specific anchor of a specific ADR: an
   item number or a declared invariant slug. An edge is one token's claim on that node,
   carrying the relation (retirement or refinement, per ADR-0128 item 2), the claiming ADR's
   number, and the number of the claiming ADR's Decision item that carries the token. Carrying
   the carrier's item number is what makes the rationale site addressable rather than merely
   known to exist, which is the property ADR-0128 item 9 rests its whole argument on. The
   result is a directed acyclic graph with multiple parents in both directions, not a tree.

3. **Per-ADR state is derived, never stored.** The model classifies each ADR as `Live` (no
   anchor carries any claim), `Covered` (the ADR has at least one anchor and every one is
   claimed by a retirement token on an `Implemented` carrier, per ADR-0128 item 3), or
   `Partial` (anything else), and exposes the distinct claimant set behind each. `Partial` is
   the residual case by construction, so an ADR whose anchors are all claimed but only by
   refinements, or only by retirements on carriers not yet `Implemented`, classifies there
   rather than falling through. An ADR with no anchors at all is `Live`, never vacuously
   `Covered`. `Covered` is what ADR-0128 item 4's check compares the authored `Superseded`
   status against.

4. **Every consumer is re-pointed at the model, with one enumerated exception.** `bucketKey`
   and `statusOrder` bucket from derived state instead of a status prefix; the domain index
   renders from the model; and `SupersessionIndex`, the `Override` type, and its `Label()`
   method are deleted, their uses becoming queries on the model. That includes `awf context`'s
   annotation path, which calls `SupersessionIndex` at `context.go:133` and threads `Override`
   through `adrRefOf` and `overrideAnnotation` (`context.go:269, 282`); it is re-pointed like
   any other consumer.

   The single exception is `awf context`'s Tier-2 exclusion (`context.go:189`), which keeps
   its status-prefix test. The distinction is not effort: it is that bucketing must show the
   divergence between derived state and authored status, because a tree failing ADR-0128 item
   4's check has an ADR whose status and coverage disagree and ACTIVE.md is where a reader
   should see it. Tier-2 exclusion is a relevance filter, not a report of supersession, so it
   reads the authored field deliberately.

5. **The domain index surfaces partial supersession, bounded.** A domain-doc entry for an ADR
   with claimed anchors names the claiming ADR numbers only; per-anchor detail stays in
   ACTIVE.md. The unbounded shape is not viable here: ACTIVE.md renders ADR-0120's annotation
   as a single line of nine clauses, and a domain index is a compact bulleted link list that
   such a line would swamp. This closes the blindness described in Context at the resolution a
   link list can carry, and every domain doc's generated `## Decisions` index re-renders on
   the sync that lands it.

   The hand-authored `.awf/domains/parts/adr-system/current-state.md` is *not* covered by that
   re-render, and it currently describes the scalar back-pointer, the three-way symmetry, and
   chains-as-pairs, all of which this pair invalidates. It is updated by hand in the same
   commit, as the docs-travel-with-the-change rule requires.

6. **Supersession chains become one-to-many.** ACTIVE.md's chain rendering, which paired one
   predecessor with one successor from the frontmatter list, renders a `Covered` ADR against
   the full set of ADRs that retired its anchors. This is the replacement rendering ADR-0128
   item 1 deferred when it retired `active-md-supersedence-rendering`; that slug's surviving
   annotation clause, re-declared by ADR-0128 as `active-md-annotates-superseded-anchors`, is
   unaffected and stays where it is. This `supersedes: ADR-0120#10` for its description of
   chains as predecessor-to-successor pairs; under ADR-0128 item 2 the claim classifies as a
   refinement, since item 10's annotation half and its requirement that the section exist both
   stand, and the generation-11 migration will rewrite it accordingly. That rewrite must land
   before this ADR reaches `Implemented`: a `supersedes:` token on an `Implemented` carrier
   counts toward coverage, so left unmigrated it would become a genuine retirement claim
   against ADR-0120 item 10.

7. **The model refuses a claim graph it cannot traverse.** `awf check` fails on a token whose
   target ADR is its own carrier, and on any cycle in the retirement relation restricted to
   ADRs the model classifies as `Covered`. That restriction is the whole rule: A completing
   B's coverage while B completes A's derives two dead ADRs, orphaning every decision they
   held with nothing live holding the replacements. A cycle among ADRs that stay `Live` or
   `Partial` is refused by nothing, because two live ADRs each retiring one anchor of the
   other is ordinary mutual evolution and nothing forbids it today. Refinement edges never
   participate in either test.

   The check therefore runs after state derivation rather than over the raw edge set, which
   costs a second pass. That is the price of not over-refusing a legal shape, and at corpus
   scale it is not a cost worth trading accuracy for. This is the item ADR-0128 dropped during
   review, landing where its subject is defined; ADR-0120 item 3's single-claimant check
   incidentally prevented full-supersession cycles, and ADR-0128 item 1 removes it.

## Invariants

- `invariant: supersession-model-single-source` - every supersession fact the tooling reports
  (ACTIVE.md buckets, ACTIVE.md chains and annotations, `awf context`'s anchor annotations,
  domain-index entries, and the coverage-versus-status check) is answered by the
  anchor-coverage model. The only consumer permitted to test a status prefix for itself is
  `awf context`'s Tier-2 exclusion, enumerated in item 4.
- `invariant: supersession-model-anchor-nodes` - the model keys claims by anchor, and each
  claim carries its relation, the claiming ADR, and the claiming ADR's Decision item number.
- `invariant: supersession-model-derives-state` - the three states are exhaustive and disjoint
  over every ADR: `Covered` exactly when the ADR has at least one anchor and every one of its
  Decision items and declared invariant slugs is claimed by a retirement token on an
  `Implemented` carrier; `Live` when no anchor carries any claim; `Partial` otherwise, which
  includes an ADR every anchor of which is claimed only by refinements or only by retirements
  on non-`Implemented` carriers.
- `invariant: domain-index-surfaces-partial` - a per-domain ADR index entry for an ADR with
  claimed anchors names those anchors and their claimants.
- `invariant: active-md-chains-one-to-many` - ACTIVE.md renders a `Covered` ADR against every
  ADR that retired one of its anchors, not a single successor.
- `invariant: supersession-graph-acyclic` - `awf check` fails on a token whose target ADR is
  its own carrier, and on a cycle in the retirement relation among ADRs the model classifies
  as `Covered`; a retirement cycle whose members are `Live` or `Partial`, and any cycle formed
  by refinement edges, pass.

## Consequences

- Adding a supersession consumer stops being a decision about which field to read. The cost of
  the current fragmentation is visible in `domain.go`, which picked the scalar and is
  therefore blind to 55 of the corpus's 58 supersession relationships. It is correct about the
  3 it reads; it simply cannot see the rest.
- Generated output changes in two places on the sync that lands this: domain docs gain partial
  annotations, and ACTIVE.md chains become one-to-many. Adopters see their `docs/domains/*.md`
  and `ACTIVE.md` re-render with content they did not author, which is normal for generated
  files but is a larger diff than a routine sync.
- The model is built per invocation from already-parsed ADRs, so it adds a pass over the corpus
  to commands that already walk it. At corpus scale (129 ADRs, 58 relationships) this is not a
  cost worth engineering around, and saying so now forestalls a premature cache.
- Deleting `SupersessionIndex` removes exported surface: the function, the `Override` type,
  and its `Label()` method. Pre-1.0 with no external API stability commitment this needs no
  deprecation, but it reaches into `internal/project` rather than staying inside
  `internal/adr`.
- Acyclicity lands here rather than in ADR-0128, so between the two ADRs' implementation there
  is a window with no constraint on the claim graph. The window is closed within one plan and
  no corpus case exists, but the ordering is a real dependency: ADR-0128's checks and this
  model are not independently shippable.
- Nothing here changes what an author writes. Both ADRs together move supersession from two
  encodings and four readings to one encoding and one reading, with no new authoring surface
  beyond ADR-0128's refinement relation.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `SupersessionIndex` and extend it for coverage | It is already two unrelated computations sharing a loop; a third return value entrenches the fragmentation rather than resolving it. |
| Model nodes as ADRs, with anchors as edge labels | Coverage is a property of the anchor set, so anchor-as-node makes the central query a lookup rather than a filter, and it is what makes the derived state cheap. |
| A tree of decisions | Parents multiply in both directions: several successors may claim one ADR, and one successor may claim several. A tree reintroduces the scalar-claimant assumption ADR-0128 item 4 deleted. |
| Fold this into ADR-0128 | The two are ordered and co-shipped, so the claim is not shipping independence; it is revision independence. The boundary between what an author writes and how the tooling computes over it is durable, so a future encoding change re-points one ADR rather than reopening both. |
| Re-point `awf context`'s Tier-2 exclusion too | Its prefix test is already correct under bare `Superseded`, and partially-superseded ADRs are already included. Changing it would be churn with no behaviour difference. |
| Cache the model across invocations | Every command that needs it already parses the corpus in the same process; a cache would add invalidation risk to save a pass over 129 records. |
