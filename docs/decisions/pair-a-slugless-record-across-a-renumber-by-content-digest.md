---
format: current-state-v3
slug: pair-a-slugless-record-across-a-renumber-by-content-digest
status: Implemented
date: 2026-08-01
---
# ADR-pair-a-slugless-record-across-a-renumber-by-content-digest: Pair a slugless record across a renumber by content digest

## Context

ADR-0202 makes an ADR number provisional until integration. A record authored in a worktree
carries a slug, `awf adr number` assigns its number against the corpus it is about to join, and
staged transition validation pairs the pending record with its numbered successor on the retained
slug. That covers every record authored from now on. It covers none of the records that already
exist.

Integration merges the integration branch into the effort branch, numbers, and then fast-forwards
the integration branch onto the result. A fast-forward creates no commit and runs no hook, so the
merge commit on the effort branch is the only transition anything checks, and its before side is
always the effort branch.

A record predating the slug format is paired on its number. When the integration branch has taken
that number meanwhile, the merge must rename ours, and both universes then hold a record keyed on
the old number: the before side ours, the after side the unrelated record that took it. They pair.
`awf check --staged` refuses the merge with 37 findings, and they are mispair-shaped rather than
the delete-plus-add shape a rename suggests: 2 on the mispaired record itself, 6 claims added that
already existed, 13 Revised-by preservation violations, 13 updates with no canonical field changed,
and 3 belonging to the other record whose batches the mispair consumed.

The number is occupied on the after side in every instance of this, because a number being taken
is what makes the rename necessary. A pairing rule that only inspects records left unpaired
therefore never sees these two, which is why the fallback shape this record first proposed could
not fire.

Seven slugless records across five in-flight efforts, this one included, already collide with
numbers the integration branch has taken. Each hits this wall at integration.

A slugless record nevertheless carries an identifier that survives a renumber. The content-sha256
covers the five canonical body sections and excludes the frontmatter and the `# ADR-NNNN:` heading,
which is precisely why ADR-0202's own numbering rewrite is digest-safe. For a slugless record that
digest is what the slug is for a current-state-v3 one: a name that does not mention the number.
It is a key for pairing two universes, not a corpus identity key. Parse-time identity and
duplicate detection keep reading the number, so `corpus-single-identity-key` is untouched.

Verified on the real merged tree, with the effort branch as first parent and `MERGE_HEAD` present
so the check applies its merge contract: resolving the digest before the number key takes the same
transition from 37 findings to clean, with the rest of the suite unaffected.

## Decision

1. Staged transition validation resolves a governed record's pairing key in three steps: the
   retained slug when it has one, then the canonical content digest for a slugless record, then
   the assigned number. The digest step is resolved before the number, not as a fallback after it.
   Ours then pairs with itself at its new number, and the record that took the old number is left
   over as an ordinary addition.

2. The digest is the content-sha256 computed over the record's five canonical sections on each
   side. A record's latest Status history stamp equals that value by construction, because
   `internal/adr/format.go` refuses a record whose latest stamp does not, so the computed form is
   used and no history position is consulted. That is not a convenience: the current stamp can
   sit on an `Amended` event in the middle of a history whose trailing `Applied` events carry no
   digest at all, so an implementation reading the last event's stamp would key such a record on
   an empty value. The computed form has no absent case to guard.

3. The digest is exposed as a method on the record. `ADR.Sections` is owned by `internal/adr`
   (ADR-0130 item 2, enforced by a test that also counts the permitted mutation fixtures), so the
   check layer asks the record for its digest rather than reading its sections.

4. A digest pair forms only on a digest carried by exactly one governed slugless record on each
   side, and only where it re-keys the record: a record whose digest resolves to the key it
   already has is left on its number. The digest step therefore changes a record's key only when
   that record's number changes across the pair: an amended record's two digests differ so neither
   side finds a partner, and a status flip or an appended batch moves neither the digest nor the
   number. Since a slugless record whose number changes is refused today in every case, this
   cannot silently accept anything the current rule refused, apart from the renumber shape item 6
   constrains. A digest repeated on either side leaves every record holding it on its number, which
   is what makes the rule fail closed: a genuine deletion alongside an unrelated addition carries
   different digests, forms no pair, and stays refused with the wording it has today.

5. The rule applies only to records carrying no `slug:` key on both sides. A current-state-v3
   record has a stable identity through numbering already, so it never reaches the digest step,
   which leaves `adr-system/adr-lifecycle:adr-number-immutable` exactly as strong for slug-carrying
   records as ADR-0202 wrote it. The mechanism is bounded to a corpus that stops growing.

6. A digest pair admits exactly the renumber and nothing else: the assigned number, the filename,
   and the heading. Its status and Status history must be byte-identical, its canonical content
   unchanged, and no application batch appended or dropped. It is the only pair for which a changed
   assigned number is not a finding.

7. The provenance substitution map that carries a numbered record's slug to its assigned number
   also carries a digest-paired record's old number to its new one. A claim citing a renumbered
   record takes the substitution contract ADR-0202 already defines: the `Origin:` and each matching
   `Revised-by:` entry substituted exactly, each touched list canonicalized to duplicate-free
   ascending order, no other change, and no operation declared, because a renumber appends no
   application batch.

8. The rename rewrites the filename and the heading line, and nothing else in the file. A
   file-wide substitution of the old number would move the digest, dissolve the pair, and surface
   as the same 37-finding avalanche naming no cause. No command performs this rename today; it is
   done by hand under that discipline, and that gap is named rather than hidden.

9. A renumber and a content amendment are separate commits, for the same reason: an amendment
   moves the digest. Renumber first, amend second.

10. The renumber target must stay below the V3 cutoff the integration re-derives. A target at or
    above it parses as V3, fails for want of a mandatory `slug:` key, or trips the changed-format
    rule.

11. The pairing keys are resolved once per transition, from both universes together, and every
    site consumes that one resolution: both directions of the record-level transition checks, the
    appended-batch derivation, and the provenance substitution map. A key function taking one
    record cannot express this rule, because both the uniqueness guard and the re-key test depend
    on the other universe, so the record indexes are built from the resolved keys rather than
    recomputed per record. Resolving it in one site and not the others leaves the remaining sites
    mispairing.

## State changes

- add `adr-system/adr-lifecycle:renumber-digest-paired`
- update `adr-system/adr-lifecycle:adr-number-immutable`
- update `adr-system/adr-lifecycle:numbering-transition-mode`

## Consequences

The renumber integration has always performed by hand becomes a checked transition. It was never
checked before, because merge commits ran no hook until ADR-0202 rendered a `pre-merge-commit`
payload, and the first thing that backstop caught was this rename. The backstop is right; the
technique it invalidated is the one integration has always used.

The relaxation is bounded three ways: to records carrying no slug, a set that stops growing the
moment ADR-0202 ships; to a digest unique on both sides; and to a record whose key actually
changes, so no ordinary transition pairs differently than it does today.

One residual risk is honest to name: two slugless records whose canonical body sections are
byte-identical would pair. They would also have to match on status, on Status history byte for
byte, and on applied batch count, so a mispair still has to look exactly like a renumber in every
respect other than the number. Two distinct decisions with identical Context, Decision, State
changes, Consequences and Alternatives are not a shape the corpus produces.

A digest repeated on one side, as opposed to the paragraph above's digest matching once on each,
does not mispair; it makes the renumber unavailable. The uniqueness guard
withholds the pair, so the rename is refused with the wording it has today, and the operator's
remedy is to distinguish the two bodies before renaming. That is the safe direction to fail, but
the refusal does not say so, which is worth knowing before reading it.

`adr-number-immutable` is amended rather than left to be read narrowly. Its opening sentence is
unconditional, and a slugless record's number now does change under a checked transition, so the
sentence is corrected to carve out the digest-paired rename while keeping the reset-remake remedy
for a stale numbering. `numbering-transition-mode` is amended on both ends: it opens by stating
that validation pairs governed records on their retained slug, which now describes only the first
of three steps, and it closes by declaring that a provenance substitution with no paired numbering
behind it stays an unmatched mutation, which a digest-paired substitution now also satisfies. The
three-step resolution is owned by `renumber-digest-paired`, and `numbering-transition-mode` states
the slug as that resolution's first step rather than restating the order itself, so the two claims
cannot drift apart.

The proof obligation is specific, because this effort has repeatedly shipped proofs that could not
fail. The claim is backed by test, and its fixtures must be plural and heterogeneous: a record
whose current stamp sits on a mid-history `Amended` event with digest-less trailing `Applied`
events, a universe where the old number is occupied on the after side rather than vacant, a
duplicated digest that must refuse to pair, an unchanged record that must still pair on its
number, a record amended at an unchanged number whose moved digest must still fall through to the
number, and a renumbered record that both carries provenance substitutions and derives an appended
batch, so all four pairing sites are proven to resolve the same key.

Each of those pins something a weaker fixture would miss. A uniform history passes against an
implementation that reads the last event's digest. A vacant old number passes against the fallback
shape that cannot fire at all. Without the amended-at-the-same-number case the ordinary commit
path is unpinned, and without the last case every fixture is satisfiable by an implementation that
fixes only the record-level check and leaves the batch derivation and the substitution map keying
on the number, which is the exact shape of this effort's Phase 2 defect.

The missing tool is a real cost. `awf adr number` renumbers a pending record; nothing renumbers a
slugless one, so the operation this record sanctions is performed by hand, and its discipline
lives in prose. `.awf/parts/working-with-awf/commands.md`, rendered into `docs/working-with-awf.md`,
gains the rename discipline, the separate-commits rule, and the cutoff constraint, and its existing
"a number, once assigned, never changes" paragraph is reconciled with the sanctioned rename.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Resolve the digest as a fallback after the number key | Cannot fire. The old number is occupied on the after side in every instance, because a number being taken is what makes the rename necessary, so no record is ever left unpaired for a fallback to consider. Verified against the real merged tree. |
| Reverse the merge direction so the integration branch is the before side | Verified to work with no pairing change at all: the renamed record becomes an ordinary addition and the same transition is clean. Rejected because integration fast-forwards the integration branch onto the effort tip, so no commit exists on that side for a check to run against, and reversing means the integration branch takes a merge commit and becomes the place merges are debugged. A sanctioned follow-on recorded in `docs/roadmap.md` moves the other way, toward making `awf effort integrate` fast-forward-only. |
| Commit the renumber with `--no-verify` | Drops build, drift, staged-authority, gate, prose and memory checks together for one narrow rewrite, leaves no trace, and routes around the `pre-merge-commit` payload for the exact operation it was rendered to catch. |
| A declarative flag or environment variable authorizing the renumber | Every in-flight effort would have to discover it at integration time, the worst moment to learn a new escape hatch, and an escape hatch checked by nothing is `--no-verify` with extra steps. |
| Retrofit the colliding record to current-state-v3 so slug pairing covers it | Adds an identity key to a record whose application batches are already applied and whose transitions were validated without it, and would have to be repeated for every slugless record a future integration renames. |
| Unmake the numbering by reset-remake, the remedy `adr-number-immutable` names | That remedy addresses a numbering that went stale before anything depended on it. Here the record carries five applied batches across six implemented phases, and unmaking it discards the checked transitions that validated them. |

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Implemented; content-sha256: 20e272fdf43ab4e1cba617ccf9d514be8afe8e5652b9857a047b8599a60a42c4
