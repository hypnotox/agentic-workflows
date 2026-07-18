---
status: Proposed
date: 2026-07-18
supersedes: []
superseded_by: ""
tags: [adr-lifecycle, adr-parsing, frontmatter-validation, invariant-retirement, schema-migration]
related: [8, 10, 14, 35, 42, 105, 116, 120]
domains: [adr-system, invariants, config]
---
# ADR-0128: Coverage-Derived ADR Supersession

## Context

ADR-0120 gave partial supersession a machine-checked encoding: an inline `supersedes:` or
`supersedes-invariant:` token, placed in the successor's Decision section at the item that
overrides the predecessor's anchor. It left full supersession where it found it, as a
frontmatter claim: the successor's `supersedes: [N]`, the predecessor's `superseded_by:
"NNNN"`, and the predecessor's `status: Superseded by ADR-NNNN`, with a three-way symmetry
check (item 3) binding them.

Two years of corpus say the frontmatter path is both redundant and rationale-free.

**`superseded_by:` duplicates the status field.** `status: Superseded by ADR-0120` and
`superseded_by: "0120"` encode the same fact in the same file. The reverse half of the
symmetry check (`internal/project/supersession.go:157`) exists to validate one against the
other, which is to say it validates a field against itself. Its only other consumer is the
domain-doc index (`internal/adr/domain.go:38-41`), which could read the status suffix.

**Full supersession carries no rationale, and that is the asymmetry ADR-0120 half-fixed.**
A partial token is placed inside the Decision item that overrides the anchor, so the claim
sits with the reasoning by construction. A full supersession is `supersedes: [31]`: a bare
integer in frontmatter, with no requirement that the successor's body mention the predecessor
at all. ADR-0120 explains in prose why it replaced ADR-0031; ADR-0032 and ADR-0115 offer no
per-predecessor justification whatsoever. The mechanism that exists to make supersession
explicit and reasoned applies to the fine-grained case and skips the coarse one.

**The corpus is overwhelmingly partial.** A sweep on 2026-07-18 across `docs/decisions`,
excluding this ADR's own tokens, counts 55 anchors (37 `supersedes:` item tokens, 18
`supersedes-invariant:` slug tokens) over 32 distinct target ADRs from 32 carrier files,
against exactly 3 full pairs: ADR-0003 to
ADR-0032, ADR-0031 to ADR-0120, ADR-0113 to ADR-0115. The frontmatter mechanism serves three
relationships and duplicates machinery the other 55 already have.

**A status flip silently retires invariants.** `DeclaringADRs` skips any ADR whose status is
not `Implemented` (`internal/invariants/invariants.go:134-136`). The moment a predecessor
flips to `Superseded`, every invariant slug it declares stops being owed: no
`supersedes-invariant:` token, no rationale, no record. ADR-0120's own supersession of
ADR-0031 retired ADR-0031's slugs exactly this way, through a side effect of the status
field, in the same ADR that made retirement explicit and token-carried everywhere else.

The unifying observation is that full supersession is not a different kind of relationship
from partial supersession. It is the same relationship, applied to every anchor. Encoding it
separately buys a second mechanism, a second set of checks, and a rationale hole.

## Decision

1. **The `supersedes:` and `superseded_by:` frontmatter keys are removed from the ADR
   schema.** Neither key is parsed, rendered by the ADR template, nor accepted: `awf check`
   fails, with upgrade guidance, on any ADR whose raw frontmatter carries either key, empty
   or not, mirroring ADR-0120 item 7's treatment of `retires_invariants:`. This
   `supersedes: ADR-0120#3` for its frontmatter encoding of full supersession; that item's
   three-way symmetry has nothing left to bind, so its
   `supersedes-invariant: ADR-0120#supersession-full-symmetry` retires with it.

   Deleting the keys also removes the only input from which full-supersession *chains* were
   computed, so `supersedes-invariant: ADR-0120#active-md-supersedence-rendering` retires
   here rather than lapsing quietly: its chain clause becomes uncomputable the moment this
   item lands. Its anchor-annotation clause survives, re-declared narrowed below, and the
   replacement chain rendering over the coverage model is ADR-B's to specify.

2. **Full supersession is derived from anchor coverage.** An ADR is fully superseded when
   every one of its anchors is claimed by a token: every column-0 Decision item number, and
   every slug its Invariants section declares. Coverage counts only tokens carried by an
   `Implemented` ADR, matching the retirement gate of ADR-0120 item 6, so a `Proposed`
   successor never kills its predecessor. Coverage may be split across several successors;
   no single ADR need claim the whole.

   Requiring slug coverage alongside item coverage is what makes the status flip
   semantically inert. Under item 1 alone, completing item coverage would still silently
   retire the predecessor's invariants through `DeclaringADRs`. Requiring every slug to carry
   its own retirement token first means the flip drops nothing that was not already retired
   explicitly, at a site that explains it.

3. **The predecessor's status is hand-authored as bare `Superseded`, and `awf check`
   enforces it against derived coverage in both directions.** The suffixed
   `Superseded by ADR-NNNN` form is retired: coverage may split across successors, so a
   scalar successor name is not always a true statement. `awf check` fails when a
   fully-covered ADR is not `Superseded`, naming the ADR and the covering carriers, and when
   a `Superseded` ADR has an uncovered anchor, naming the anchor. The flip lands in the same
   commit that brings the final covering carrier to `Implemented`; that commit is one
   concern, the completion of the supersession, even though it touches two ADR files.

   awf does not write the flip. ADR-0035 item 1 makes lock membership the test of what awf
   owns: a path recorded in the lock is awf's own output, overwritten freely. ADRs are
   hand-authored source and are never in the lock, so having `awf sync` write one would make
   awf the author of a source document on every routine run. Enforcement without authorship
   is the whole point: the
   human types the field and has no discretion about its value, because the check names the
   required edit exactly.

4. **The back-pointer requirement widens to targets of any status.** `awf check` fails when a
   token targets an ADR whose `related:` lacks the carrier's number, regardless of whether
   that target is live or `Superseded`. This `supersedes: ADR-0120#4`, which scoped the check
   to live targets only. The widening is load-bearing for item 3: with bare `Superseded`
   naming no successor, `related:` is the only surface on the predecessor that names its
   claimants, and under the narrow rule a claimant landing after the flip would owe no
   back-pointer and be unrecoverable from the predecessor. Editing a frozen ADR's `related:`
   is permitted in place by ADR-0116 item 2. This
   `supersedes-invariant: ADR-0120#supersession-backpointer`, whose live-targets-only scope
   this item replaces.

5. **Flavour exclusivity is retired and the superseded-target advisory is dropped; the
   contested-anchor advisory is retained.** With one flavour there is nothing to be exclusive
   about, and a token targeting a `Superseded` ADR becomes the normal shape of every
   completed supersession rather than a degradation worth noting. An anchor claimed by two or
   more live ADRs remains an `awf check` note. This `supersedes: ADR-0120#5` and
   `supersedes-invariant: ADR-0120#supersession-flavour-exclusive`. It also
   `supersedes-invariant: ADR-0120#supersession-conflict-advisory`, which bundles both
   advisories in one slug: slugs are atomic, so dropping one of the pair means retiring the
   slug and re-declaring the surviving half narrowed.

6. **No aggregate token.** There is no shorthand that claims every anchor of a target at
   once. Full supersession costs one token per anchor, each placed at the Decision item that
   supersedes it. The friction is the mechanism: an ADR cannot be retired wholesale without
   someone stating, anchor by anchor, what replaces it.

7. **The coverage graph is acyclic and irreflexive.** `awf check` fails on a token whose
   target is its own carrier, and on any cycle in the anchor-coverage graph. ADR-0120 item 3's
   single-claimant check incidentally prevented full-supersession cycles; removing it leaves
   the derived model to traverse a graph nothing else constrains.

8. **`awf upgrade` gains a corpus migration at schema generation 10 to 11.** The migration
   strips both keys from every ADR under the configured docs dir, and for each ADR that
   carried a non-empty `supersedes:` appends one bookkeeping Decision item carrying a token
   per anchor of each named predecessor, inserting the carrier's number into each target's
   `related:` when absent and rewriting each predecessor's suffixed status to bare
   `Superseded`. Appending is triggered only by a non-empty `supersedes:` key, which the same
   migration strips, so a re-run finds no trigger and is a no-op; no separate marker is
   needed. Anchor enumeration runs against the post-generation-10 body, so the bookkeeping
   items ADR-0120 item 8 already appended are themselves anchors this migration must claim.
   The appended item is permitted by ADR-0120 item 9's carve-out shape 2, a numbered
   bookkeeping item encoding an obligation the ADR already carried: `supersedes: [3]` already
   asserted replacement of all of ADR-0003.

9. **The check enforces coverage and placement, never rationale quality.** `awf check` can
   prove that every anchor is claimed and that each claim sits inside a column-0 Decision
   item of its carrier. It cannot prove that the item explains anything. This ADR delivers
   reasoned supersession by guaranteeing a rationale *site* per anchor and leaving the
   judgment of what is written there to the `awf-reviewing-adr` lens. The gain over
   `supersedes: [31]` is that a site exists at all.

## Invariants

- `invariant: supersession-keys-refused` - `awf check` fails, with upgrade guidance, on any
  ADR whose raw frontmatter carries the `supersedes` or `superseded_by` key, empty or not.
- `invariant: supersession-coverage-derives-status` - `awf check` fails when an ADR every one
  of whose Decision items and declared invariant slugs is claimed by a token on an
  `Implemented` carrier does not have status `Superseded`, and when an ADR with status
  `Superseded` has an anchor no such token claims.
- `invariant: supersession-coverage-implemented-only` - an anchor counts as covered exactly
  when its claiming token's carrier is `Implemented`; carriers in any other status, including
  `Proposed` and `Superseded`, leave the anchor uncovered.
- `invariant: supersession-backpointer-any-status` - `awf check` fails when a token targets an
  ADR whose `related:` lacks the token-carrier's number, for a target of any status.
- `invariant: supersession-graph-acyclic` - `awf check` fails on a token whose target ADR is
  its own carrier, and on any cycle in the anchor-coverage graph.
- `invariant: supersession-contested-anchor-advisory` - an anchor claimed by two or more live
  ADRs surfaces as an `awf check` note, never an error; a token whose target is `Superseded`
  surfaces nothing.
- `invariant: upgrade-migrates-supersession-keys` - the generation-11 migration strips
  `supersedes` and `superseded_by` from every ADR under the configured docs dir, appends to
  each former full-supersession carrier one bookkeeping Decision item whose tokens claim every
  anchor of each named predecessor, inserts the carrier's number into each target's `related:`
  when absent, rewrites each predecessor's status to bare `Superseded`, and is a no-op on a
  corpus it has already migrated.
- `invariant: active-md-annotates-superseded-anchors` - ACTIVE.md renders an annotation on
  each live ADR that has a superseded anchor. This is the surviving half of ADR-0120's
  `active-md-supersedence-rendering`, retired at item 1; how ACTIVE.md renders claimants for a
  fully-superseded ADR, now that no scalar successor name exists, is ADR-B's to declare.

The five slugs this ADR retires are claimed by `supersedes-invariant:` tokens at the Decision
items that override them: `supersession-full-symmetry` and `active-md-supersedence-rendering`
at item 1, `supersession-backpointer` at item 4, `supersession-flavour-exclusive` and
`supersession-conflict-advisory` at item 5. Tokens are recognised only inside `## Decision`
(ADR-0120 item 1), so this paragraph is a reader's summary and carries no claim of its own.

## Consequences

- One encoding replaces two. The `adr-supersession` drift kind and the flavour-exclusivity
  check disappear; `adr-token-ref`, `decision-items-enumerable`, and the retirement gate are
  untouched and become load-bearing for coverage, since anchors are only as stable as the
  Decision-item numbering ADR-0120 item 12 froze.
- Full supersession gets materially more expensive: one token per anchor, each at a Decision
  item that supersedes it, where previously it was one integer. That cost is the point, but
  it is real, and it scales with the predecessor's anchor count rather than with how much of
  it is genuinely being replaced.
- The three legacy pairs structurally cannot have per-anchor rationale. Their bookkeeping
  items record what was claimed, not why, because no such reasoning was ever written and
  inventing it now would be a content edit the append-only rule forbids. The rule is
  prospective; the corpus carries three grandfathered records.
- Migration-appended bookkeeping items are themselves permanent anchors. A future successor of
  ADR-0032, ADR-0115, or ADR-0120 must write a rationale-bearing token against a bookkeeping
  item that carries no rationale to supersede, so the retirement cost of those three ADRs is
  inflated by pure ceremony. The same already holds for the items ADR-0120 item 8 appended.
- `awf upgrade` appends numbered Decision items to adopters' authored, frozen ADR bodies.
  This is a stronger intrusion than the sync-writing rejected in item 3, and is accepted on a
  narrower ground: `upgrade` is one-shot, invoked deliberately, and lands as a reviewable
  diff, where `sync` is routine and continuous. ADR-0120 item 8's migration writing under
  `docsDir` is the precedent.
- Rendered output loses the successor name in two places. The domain-doc index
  (`internal/adr/domain.go:38-41`) can no longer print `-> superseded by ADR-NNNN` from a
  scalar field, and ACTIVE.md's `Superseded` bucket becomes an undifferentiated list. Both
  recover the claimants from the coverage model instead; how they render is ADR-B's concern.
- Nothing here gives an ADR withdrawn without a successor a terminal state. `Superseded` now
  requires coverage where before it required a named successor, so the gap is unchanged, not
  widened. It stays with the lifecycle convention.
- Adopters upgrading with an asymmetric or hand-broken full-supersession record migrate into
  a coverage shortfall rather than a symmetry failure, and repair it by writing the missing
  tokens. The diagnostic names the uncovered anchors.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Drop only `superseded_by:`, keep `supersedes: [N]` | Removes the self-validating field but leaves two encodings and the whole rationale hole for full supersession. |
| Keep both keys, add a required rationale field | A schema slot for prose is checkable only for presence, so it buys ceremony without the property that makes tokens work: placement at the overriding decision. |
| Let `awf sync` write the status flip | Sync would take permanent ownership of an authored ADR (ADR-0035 item 2). Enforcement by `awf check` gets the same zero-discretion outcome without awf authoring source documents. |
| Drop `Superseded` as a status value, derive liveness only in indexes | Costs the ACTIVE.md status bucket and leaves a fully-dead ADR reading `Implemented` when opened directly, which is the signal the status field exists to give. |
| Keep the suffixed `Superseded by ADR-NNNN` form | Coverage may split across successors, so the scalar is not reliably true; `related:`, widened by item 4, carries the full claimant set. |
| Coverage over Decision items only | Leaves the status flip silently retiring the predecessor's invariants, reproducing for slugs the exact rationale hole this ADR closes for items. |
