---
format: current-state-v3
slug: lock-cutoffs-and-schema-generations-reconcile-at-integration
status: Implemented
date: 2026-08-01
---
# ADR-lock-cutoffs-and-schema-generations-reconcile-at-integration: Lock cutoffs and schema generations reconcile at integration

## Context

ADR-0202 made ADR numbers provisional until integration: a record authored in a worktree
carries a slug, and `awf adr number` assigns its number against the corpus it is about to
join. That decision treated the ADR number as the one identifier parallel efforts collide
on. Integrating it proved otherwise.

Two more values are allocated the same optimistic way and collide the same way.

A schema migration takes the next free generation. Two branches in flight both take it,
and the second to integrate must renumber. That happened here: `drop-max-claims-per-topic`
and `adr-format-v3-cutoff` both claimed generation 28.

A lock cutoff is worse, because renumbering is not available. `adrFormatV3From` is computed
at seal time as the corpus's next identity and is then immutable: `awf check` admits only
the sealing edge of a cutoff the prior authority did not carry
(`config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`). The immutability is right
for a published cutoff. It is wrong for one sealed inside a branch, because the value is
computed against a corpus the integration is about to change. ADR-0202 sealed 195 in its
worktree; by integration the branch it was merging into held records 0195 through 0201, all
`current-state-v2`. At cutoff 195 those seven records route to the V3 parser and every
gated command fails with `frontmatter format must be "current-state-v3"`. The branch could
not integrate at all: keeping the cutoff broke the corpus, and changing it was refused.

A second, smaller problem surfaced with it. Two claim sentences name schema generations as
literals (`schema 28 the V3 cutoff`, `the schema-29 migration`). A generation renumber at
integration falsifies both, and the record that wrote them cannot fix them, because a claim
carries at most one operation per ADR and both operations were already applied.

## Decision

1. A lock cutoff sealed while a branch is unintegrated is provisional. Integration
   re-derives it against the merged corpus, exactly as `awf adr number` re-derives an ADR
   number, and for the same reason: the value answers a question about a corpus that does
   not exist yet.
2. `awf check` admits a second edge on permanent lock authority: a cutoff already present
   may take a new value when the staged transition also advances the schema generation.
   Every other permanent value stays byte-identical, and a transition that changes a cutoff
   without advancing the generation is refused as before. A published cutoff therefore
   still never drifts, because an ordinary commit does not move the generation.
3. A current-state claim does not name a schema generation as a literal. The generation is
   an allocation detail that integration can renumber; the claim states which migration
   owns the effect, not what number it drew. The two sentences carrying literals lose them.

## State changes

- update `config/migrations-and-locks:adr-v2-cutoff-atomic-immutable`
- update `config/configuration:integration-branch-explicit`

## Consequences

Integration gains a step: after merging the integration branch in, re-derive the cutoff
before merging back, alongside numbering any pending record. The alternative was an
unintegrable branch.

The immutability guarantee narrows in a way that is visible rather than silent. A cutoff can
now change, but only in a commit that also advances the schema generation, which is a
migration commit and nothing else. The claim states the narrowed rule, so the check and the
authority agree.

Removing generation literals from claim prose costs a little concreteness and removes a
whole class of integration-time falsification. The generation stays discoverable from the
migration registry, which is where it is authoritative.

This record is itself the first pending slug ADR: it was authored with no number and takes
one at integration through `awf adr number`, which is the workflow ADR-0202 shipped.

## Alternatives Considered

Retrofit the colliding records to `current-state-v3` and keep the cutoff at 195. The
append-only invariant permits a meaning-preserving schema retrofit, and frontmatter sits
outside the content digest, so adding the derivable `slug:` key to seven landed records
would have been mechanically safe. Rejected because it retroactively changes what those
records are to preserve a value that was wrong, and it breaks any other branch in flight
holding a `current-state-v2` record in that range.

Rewrite the effort branch's history so the wrong seal never existed. Rejected: the branch
carries applied operation batches, and rewriting them would falsify the transition checks
that already validated them.

Leave the two generation literals wrong and correct them in whatever record next touches
those claims. Rejected: they are the current-state authority on how the cutoff and the
required key are sealed, and both would have stated a number that no migration draws.

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Implemented; content-sha256: eeabfd9979b223b46f350ac6627ea4fba3ca964fcb401f712b5f734b5b4c5014
