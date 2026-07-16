---
status: Implemented
date: 2026-07-16
supersedes: [31]
superseded_by: ""
tags: [adr-lifecycle, adr-parsing, invariant-retirement, schema-migration, active-md]
related: [8, 10, 25, 39, 42, 105, 116]
domains: [adr-system, invariants, config, tooling, rendering]
---
# ADR-0120: Structured, machine-checked ADR supersession

## Context

Supersession is the least-structured load-bearing relation in the decision corpus. Both of its
flavours rest on convention that awf cannot see:

- **Full supersession is half-parsed.** The successor declares `supersedes: [N]` and the
  predecessor carries `status: Superseded by ADR-NNNN` plus `superseded_by:`, but
  `internal/adr/adr.go`'s frontmatter struct has no `Supersedes` field at all: the successor-side
  claim is decorative. Nothing checks that the three records agree. Today's corpus happens to be
  symmetric (0032 supersedes 0003, 0115 supersedes 0113, both predecessors flipped and
  back-filled), but only because the lifecycle skill's prose was followed by hand.
- **Partial-item supersession is entirely freeform.** ADR-0116 codified the convention: the
  successor cites the overridden item in prose ("supersedes ADR-0008 Decision item 4", ADR-0105;
  five such citations in ADR-0119 alone), both sides link via `related:`, and the predecessor
  stays live. ADR-0116 Decision 5 explicitly declined a mechanical back-pointer check, partly
  because pre-existing violations would red the gate; its Decision 6 knowingly left at least ten
  historical back-pointer edges missing. Nothing verifies a cited item exists, and two live ADRs
  can silently claim the same item.
- **Invariant retirement is a parallel, frontmatter-shaped mechanism.** ADR-0031's
  `retires_invariants:` field lets an Implemented successor drop a predecessor's invariant slug
  from owed backing. Retiring an invariant is semantically a partial supersession of the
  declaring ADR, yet it uses a different channel (frontmatter list) than the one ADR-0105 chose
  when it moved invariant classification *out* of frontmatter into inline markers precisely
  because a separate list drifts from the prose it summarizes.

Two properties of this project make structure cheap to add now. First, ADR bodies are immutable
once they leave `Proposed` (ADR-0116 Decision 2), so Decision item numbers are stable anchors: a
corpus sweep confirms all ADRs carry clean sequential column-0 `1.`-style items under
`## Decision` (two ADRs, 0067 and 0115, contain indented numbered sub-lists, so enumeration must
anchor at column 0). Second, invariant slugs are already extracted by a fixed grammar
(`declRe`, `internal/invariants/invariants.go`) from each ADR's Invariants section, so refs can
share that extraction.

The user's directive for this change: supersession must be either full-ADR or partial and pointed
at superseded decisions specifically; and "changes to our ADR schema should be retrofitted, since
ADRs are our central point that awf needs to be able to understand". Retrofitting is the forcing
constraint: it rules out keeping `retires_invariants:` as a permanently-grandfathered legacy
field, and it requires a principled account of how frozen bodies may be touched by a schema
migration. In this repo, 12 ADRs carry non-empty `retires_invariants:` and 89 files carry the key
at all (the scaffolding template included it; the embedded template shipped to adopters never
did). The rendered `awf-adr-lifecycle` skill (embedded template, so every adopter has it) claims
ACTIVE.md records supersedence chains in a "Supersedence chains" section; `RenderActiveMD`
renders no such thing. That drift is repaired by making the claim true rather than deleting it.

## Decision

1. **Inline partial-supersession tokens, one key per superseded kind.** A partial supersession
   is declared by a machine-readable inline code token in the **successor's** `## Decision`
   section, at the citation site where the overriding item explains itself:
   `` `supersedes: ADR-NNNN#<item>` `` overrides a Decision item, and
   `` `supersedes-invariant: ADR-NNNN#<slug>` `` overrides an invariant. The superseded kind is
   named by the key, never inferred from the anchor's shape, matching how the inline marker
   family already namespaces kinds by key suffix (`invariant:`, `unbacked-invariant:`,
   `touches-invariant:`). One token per overridden anchor. Tokens are recognized only inside
   `## Decision`; anywhere else the same text is inert prose. Full (whole-ADR) supersession
   never uses a token; it stays in frontmatter (item 3). This follows ADR-0105's
   inline-over-frontmatter precedent: the claim lives with its rationale and cannot drift from
   a separate list.

2. **Anchor grammar.** A `supersedes:` token's anchor is a Decision item number
   (`[1-9][0-9]*`, no leading zeros), resolved against the target ADR's `## Decision` section,
   enumerating only column-0 `N.` items. A `supersedes-invariant:` token's anchor is an
   invariant slug (`[a-z0-9-]+`, the declaration grammar), resolved by a status-independent raw
   scan of the target ADR's `## Invariants` section using the same declaration grammar as
   invariant backing (not via the post-retirement declared-slug map, which would make a token
   dangle against its own retirement effect). Either token's target must not be `Proposed`: a
   Proposed body is still mutable, so its item numbers are not yet anchors, and the convention
   amends a Proposed ADR in place instead of superseding it.

3. **Full supersession becomes symmetric and checked.** `supersedes:` frontmatter (bare ADR
   numbers) is parsed, and `awf check` fails unless the three records agree: successor
   `supersedes: [N]` if and only if predecessor `status:` is `Superseded by ADR-<successor>` if
   and only if predecessor `superseded_by:` names the successor. An ADR has at most one full
   successor (`superseded_by:` is scalar); a second claimant is an error. The symmetry check
   demands nothing of `related:` on full-supersession pairs; the existing pairs are
   inconsistent on that edge (0115 lists 113, the others carry none) and may stay so.

4. **The partial back-pointer becomes a check.** When a token targets a live
   (`Accepted`/`Implemented`) ADR, `awf check` fails unless the target's `related:` contains the
   successor's number. This `supersedes: ADR-0116#5` reverses ADR-0116's choice to keep
   back-pointers procedure-only: the objection recorded there (a check would red the gate on
   pre-existing violations) is answered by this ADR's retrofit, which backfills the missing
   edges in the same effort that lands the check. ADR-0116 Decision 3's scoping of *when* a
   back-pointer is owed is unchanged; the token is now simply how an owed override is written.

5. **Flavour exclusivity and advisory degradations.** One successor must not both fully
   supersede and partially supersede the same target; `awf check` fails on it. Two conditions
   degrade to advisory notes on `awf check`'s note channel rather than errors, because tokens
   are immutable prose and a hard error could turn permanently red with no legal remediation:
   a token (of either kind) whose target ADR has since been fully superseded, and one anchor
   claimed by tokens in two or more live ADRs (clause-level splits of one item are legitimate, per ADR-0119's item 7
   treatment). These advisories live in `awf check`, not `awf audit`: audit rules are pure over
   the commit range (ADR-0025 records the single sanctioned exception), while check has native
   corpus access. The check cannot tell a token whose target died *after* authoring from one
   freshly written at a dead anchor, so the authoring-time rule is procedural: the lifecycle
   skill instructs that a new token targets the ADR that currently owns the anchor, and the
   advisory note is the mechanical net for both arrival orders.

6. **Invariant retirement is subsumed by `supersedes-invariant:` tokens.** A token
   `` `supersedes-invariant: ADR-NNNN#<slug>` `` carried by an **Implemented** ADR drops that slug from
   owed backing, exactly as `retires_invariants:` does today: same Implemented-only activation,
   same dangling-reference error when the slug resolves to nothing. Retirement lapse semantics
   are preserved: if the token-carrying ADR is itself later fully superseded, its retirements
   lapse and the new successor must re-carry any it still intends (an authoring obligation the
   lifecycle skill documents). This ADR therefore **fully supersedes ADR-0031**: all five of its
   Decision items define the replaced mechanism. Its three backed invariant slugs
   (`inv-retirement-drops-slug`, `inv-retirement-implemented-only`,
   `inv-retirement-dangling-errors`) stop being owed by the status flip itself, since declared
   slugs are read from Implemented ADRs only; this ADR declares their successors below. Between
   the flip and this ADR reaching Implemented, the mechanism is check-unenforced though still
   code-implemented: the three slugs are unowed and their proof markers surface as
   dangling-marker advisories, which the implementing effort clears by re-annotating the tests
   with the successor slugs. The same window opens whenever a full supersession of an
   invariant-declaring ADR lands under the same-commit flip convention.

7. **`retires_invariants:` is removed from the schema.** The frontmatter key ceases to exist:
   the ADR struct drops the field, the scaffolding template part drops the line, and `awf check`
   fails on any ADR whose raw frontmatter still carries the key, with guidance to run
   `awf upgrade`. The detection is a deliberate raw-key scan: non-strict YAML parsing would
   otherwise ignore the removed field silently, and a silently-dropped retirement would resurface
   retired slugs as falsely owed.

8. **`awf upgrade` gains a corpus migration.** A new schema generation (9 to 10) carries a
   migration over `<docsDir>/decisions/`: for every ADR file whose frontmatter carries
   `retires_invariants:`, the key is stripped; where the list was non-empty, a new numbered
   bookkeeping item is **appended** to that ADR's `## Decision` section carrying one equivalent
   `` `supersedes-invariant: ADR-NNNN#<slug>` `` token per retired slug, resolving each slug to its
   declaring ADR from the pre-retirement corpus view. For every token it writes, the migration
   also inserts the carrier's number into the target ADR's `related:` when absent - the item-4
   back-pointer is owed by migration-written tokens like any other, and a migration whose output
   fails the checks it accompanies would be self-defeating. A retired slug that resolves to no
   declaring ADR fails the migration loudly, naming the carrier ADR and the slug: skipping it
   would silently drop a retirement that turns live if the carrier is later Implemented, the
   exact failure item 7's raw-key scan exists to prevent. Appending never renumbers existing
   items. The ADR-0039 binary-version gate forces unmigrated projects through `awf upgrade` as
   usual.
   This is the first migration that writes outside the config tree; `internal/migrate` may
   import `internal/adr` (acyclic, off the render path) and resolves `docsDir` from loaded
   config as the close-enabled-set migration already does.

9. **The append-only rule gains an encoding carve-out.** Decision *content* remains immutable
   once an ADR leaves `Proposed`; its machine-readable *encoding* may be migrated by a
   meaning-preserving schema retrofit. Permitted under the carve-out, exhaustively: stripping or
   renaming a schema-owned frontmatter key; appending a numbered bookkeeping item that encodes
   an obligation the ADR already carried; inserting an inline token immediately adjacent to an
   existing prose citation that already states the claim the token encodes. Any edit that
   changes what was decided or why remains forbidden. This `supersedes: ADR-0116#2` narrowly:
   that item's "the body is frozen" becomes "the body's meaning is frozen"; its broadening of
   in-place edits to status plus cross-reference metadata is otherwise unchanged.

10. **Supersession renders.** ACTIVE.md gains a generated supersedence section: full chains
    (predecessor to successor), and per-ADR annotations on live ADRs whose anchors are
    superseded ("item 3 superseded by ADR-0120"; "slug x-y superseded by ADR-0120"), making the
    lifecycle skill's existing "Supersedence chains" claim true for every adopter. `awf context`
    annotates surfaced ADRs the same way, so an agent reading a partially-superseded ADR learns
    which parts no longer bind without opening the successor.

11. **This repo retrofits its own corpus in the implementing effort.** Running the new
    `awf upgrade` migrates the 12 retiring ADRs and strips the key from the rest; the freeform
    partial-supersession citations across the corpus (0105, 0119, 0081, 0108, 0020, and the
    rest of a grep sweep) are hand-tokenized under the item-9 carve-out; the back-pointer edges
    ADR-0116 Decision 6 deferred are backfilled as `related:` metadata edits; the nested example
    adopter `examples/sundial` is upgraded and re-rendered; and the prose surfaces that describe
    retirement or supersession (the embedded adr-lifecycle skill template, the AGENTS.md
    invariants bullet, the decisions README template, the catalog's ADR-state mutability
    strings, this repo's domain current-state parts, glossary and pitfalls sidecars) are updated
    in the same effort. The implementing commits run `./x sync` and stage the regenerated
    `docs/decisions/ACTIVE.md` - now carrying the item-10 supersedence section - alongside the
    rendered outputs and both lock files.

12. **The Decision-section format becomes spec, and a check enforces it.** Anchors are only as
    stable as the format they resolve against, so the authoring convention the proposing skill
    has carried as prose is promoted to schema: every ADR's `## Decision` section consists of
    column-0 numbered items (`N.`), numbered sequentially from 1 with no gaps, duplicates, or
    restarts, each a discrete commitment; introductory prose before item 1 and indented
    sub-lists inside an item remain legal, since enumeration reads only column-0 leads.
    `awf check` fails on any ADR, regardless of status, whose Decision section has no
    enumerable items or breaks the sequence - a Superseded ADR can still be an anchor target,
    so its enumerability still matters. The corpus already complies (a sweep of all 120 ADRs
    found clean sequential items), so the check lands green. The decisions README template
    documents the format spec alongside the token grammar.

## Invariants

- `invariant: supersession-token-ref-validity` - `awf check` fails on a Decision-section token
  (either key) whose target ADR does not exist, whose `supersedes:` anchor exceeds the target's
  column-0 Decision item count, whose `supersedes-invariant:` anchor matches no declaration in
  the target's Invariants section, or whose target ADR is `Proposed`.
- `invariant: supersession-full-symmetry` - `awf check` fails on any one-sided full
  supersession: a `supersedes:` entry without the predecessor's matching
  `Superseded by ADR-NNNN` status and `superseded_by:`, either of those without the successor's
  `supersedes:` entry, or two ADRs claiming the same predecessor in `supersedes:`.
- `invariant: supersession-backpointer` - `awf check` fails when a token targets a live ADR
  whose `related:` lacks the token-carrier's number.
- `invariant: supersession-flavour-exclusive` - `awf check` fails when one ADR both lists a
  target in `supersedes:` and carries a token targeting the same ADR.
- `invariant: supersession-conflict-advisory` - a token whose target was later fully
  superseded, and an anchor claimed by two or more live ADRs, each surface as `awf check`
  notes, never errors.
- `invariant: token-retirement-implemented-only` - a `supersedes-invariant:` token drops its
  slug from owed backing exactly when the token-carrying ADR's status is `Implemented`;
  carriers in any other status, including `Superseded`, leave the slug owed.
- `invariant: token-retirement-dangling-errors` - a `supersedes-invariant:` token on an
  Implemented carrier whose slug no ADR declares is an `awf check` error.
- `invariant: retires-invariants-key-refused` - `awf check` fails, with upgrade guidance, on
  any ADR whose raw frontmatter carries the `retires_invariants` key, empty or not.
- `invariant: upgrade-migrates-retirements` - the generation-10 migration strips the
  `retires_invariants` key from every ADR under the configured docs dir and appends, to each
  ADR that had a non-empty list, one bookkeeping Decision item whose tokens name each retired
  slug's declaring ADR; it inserts the carrier's number into each target's `related:` when
  absent, and fails naming the carrier and slug when a retired slug resolves to no declaring
  ADR.
- `invariant: decision-items-enumerable` - `awf check` fails on an ADR of any status whose
  `## Decision` section has no column-0 numbered items or whose item numbers are not
  sequential from 1 (gap, duplicate, or restart).
- `invariant: active-md-supersedence-rendering` - ACTIVE.md renders every full-supersession
  chain and an annotation on each live ADR that has a superseded anchor.
- `invariant: context-annotates-superseded-anchors` - `awf context` output marks a surfaced
  ADR's superseded anchors with their superseding ADR numbers.

## Consequences

- The lifecycle conventions ADR-0116 chose to keep procedural become deterministic checks, which
  is a strictness increase on every adopter: corpora with asymmetric full supersessions, missing
  back-pointers on tokenized citations, leftover `retires_invariants:` keys, or Decision
  sections that are not enumerable numbered items fail `awf check` after upgrading. The migration handles the key mechanically and writes the back-pointers its
  own tokens owe, so the migration's own output passes the checks it accompanies; a corpus with
  an asymmetric full-supersession record still fails item 3's check and is repaired by hand.
  Beyond that, back-pointers and tokens only fail where an adopter chooses to write tokens,
  since freeform prose citations stay inert.
- The append-only rule is no longer absolute. The carve-out is deliberately narrow (three
  enumerated edit shapes, all meaning-preserving), but it is a real weakening: reviewers must
  now distinguish encoding edits from content edits when a diff touches a frozen body.
- Retirement claims move from one greppable frontmatter line to tokens inside prose. The
  `awf context` and ACTIVE.md annotations (item 10) are the compensating discoverability
  surface, and `awf check` remains the enforcement point either way.
- A partially-superseded ADR keeps rendering as live with annotations; readers of the raw file
  still depend on `related:` plus the successor's prose, as today. Only tokenized overrides get
  annotations; the untokenized historical citations an adopter never retrofits stay invisible
  to the tooling.
- `internal/migrate` writing under `docsDir` is a precedent: migrations are no longer confined
  to the config tree. The migration is idempotent (a stripped corpus is a no-op) and the
  ADR-0039 gate sequences it before any gated command touches the corpus.
- The retrofit commit set for this repo is sizeable (89 frontmatter strips, 12 appended items,
  a tokenization sweep, back-pointer backfill, sundial re-render) and lands under the item-9
  carve-out; the review burden is concentrated in verifying meaning-preservation.
- `sections()` parsing, item enumeration, and token extraction become load-bearing for
  correctness of the gate; they need the same test rigor as the invariant scanner.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Structured frontmatter for partial supersession (`supersedes: [{adr, items, invariants}]`) | Re-introduces the list-drifts-from-prose failure ADR-0105 eliminated; the claim and its rationale separate. |
| Frontmatter plus inline tokens, cross-checked | Double bookkeeping on every partial supersession forever, to save one body scan. |
| `§` section-sign syntax (`ADR-0001 §3`) | Non-ASCII glyph, hostile to typing and grep; `#` carries the same meaning. |
| One `supersedes:` key with shape-inferred anchors (digits = item, kebab = slug) | Muddies which kind is superseded at every citation; needs an all-digit-slug ban to stay unambiguous; the marker family already namespaces kinds by key suffix. |
| Keep `retires_invariants:` alongside tokens | Two mechanisms for one semantic invites permanent which-one ambiguity; retirement *is* partial supersession of the declaring ADR. |
| Deprecate `retires_invariants:` without retrofit (grandfather old ADRs) | Violates the retrofit directive; leaves awf permanently parsing a dead field and adopter corpora permanently dual-schema. |
| Same-anchor conflict as an `awf audit` rule | Audit rules are pure over the commit range (ADR-0025); the conflict needs corpus state, which `awf check` already has. |
| Hard error on tokens into later-superseded targets | Tokens are immutable prose; the error would be permanently red with no legal fix. |
| Anchor Decision items by content hash instead of number | Robust to renumbering that cannot happen (bodies are frozen); unreadable in prose and hostile to authors. |
