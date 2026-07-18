---
status: Proposed
date: 2026-07-18
tags: [adr-parsing, audit-rules, domain-index, context-query, invariant-backing]
related: [14, 17, 92, 104, 120, 128, 129]
domains: [adr-system, tooling, invariants]
supersedes: []
superseded_by: ""
---
# ADR-0130: One Parsed ADR Corpus View for Every Consumer

## Context

`adr.ParseDir` (`internal/adr/adr.go:171`) is the only way into the corpus, and ten production
sites call it independently. A single `awf check` run parses the whole decisions directory at
least eight times: five in `internal/project/check.go` (`:81`, `:615`, `:702`, `:754`, `:799`),
twice in `internal/project/supersession.go` (`:23`, `:69`, whose own comment acknowledges the
double parse), and once more per configured domain through `internal/project/render.go:764`.
Four of those parses exist only to rebuild the same `known[a.Number]` existence set.

Repetition is the cheap half of the problem. The expensive half is that the repeated
computations have drifted apart.

- **"Is this ADR live?" has three implementations**, two of them inside the same function:
  `adr.go:133` spells it as a negation, `supersession.go:169` as a disjunction, and
  `supersession.go:207` inlines the disjunction a second time.
- **"Is it superseded?" is a `HasPrefix` test at five sites** (`adr.go:28`, `adr.go:128`,
  `context.go:189`, `supersession.go:155`, `supersession.go:212`), and the exact status string
  `"Superseded by ADR-" + N` is reconstructed from parts three separate times
  (`supersession.go:146`, `:156`, and the render path).
- **The supersession relation is derived twice, differently.** `SupersessionIndex`
  (`adr.go:117-156`) builds chains and overrides one way; `computeSupersession`
  (`supersession.go:110-228`) rebuilds its own `byNum` map and a `claimants` inverse
  independently; and `domain.go:38` ignores both, rendering from the raw `SupersededBy` scalar.
  That third path is why the domain indexes are blind to partial supersession, the defect
  ADR-0129 exists to close.
- **`internal/audit` does not use `internal/adr` at all.** It re-declares anonymous
  frontmatter structs to pull status and domains out of git blob text (`audit.go:399`,
  `audit.go:487`), duplicating `adrFrontmatter` (`adr.go:198-205`) and the `"Implemented"`
  literal that `invariants.go:135` also carries.
- **Consumers disagree on ADR identity.** `invariants.DeclaringADRs` keys by filename
  (`invariants.go:159`) while every other consumer keys by number, so `context.go:145-148`
  maintains a translation map for no reason but that mismatch.

ADR-0129 introduces a coverage model that every supersession consumer reads. Without a corpus
view to hang it on, it becomes a *fifth* independently-constructed structure over the same
records, threaded by hand to each of the ten load sites. The two decisions are cheap together
and expensive apart.

## Decision

1. **The corpus is parsed once per invocation and threaded, never re-parsed.** A `Corpus`
   value is constructed from a single `ParseDir` and passed to every consumer that needs ADR
   facts. The five `check.go` sites, the two in `supersession.go`, and the render paths all
   receive it rather than loading for themselves; they already share a `*Project` receiver, so
   the threading is a field, not a new parameter on every signature.

2. **The view answers every category of ADR fact.** State (liveness, supersession status,
   ACTIVE.md bucket), decisions (item enumeration and count), supersession (ADR-0129's
   anchor-coverage model, constructed as a facet of this view rather than beside it),
   declared invariant slugs, metadata lookups, and existence tests. A consumer asks the view a
   question; it does not read fields and reimplement the question.

3. **Shared predicates are defined once.** Liveness, superseded-ness, and bucket membership
   each exist as one named predicate on the view. No consumer compares a status against a
   string literal; the literals live in `internal/adr` and nowhere else. This is what makes
   the three-way `live` divergence and the five-way `HasPrefix` divergence unrepeatable rather
   than merely repaired.

4. **The ADR number is the single identity key.** `internal/invariants` stops returning
   filenames to identify declaring ADRs, and `context.go`'s filename-to-number translation map
   is deleted. Filename remains a field for the consumers that render links; it stops being an
   identity.

5. **A bytes-level parse seam serves the git-blob consumers.** `internal/adr` exports an entry
   point that parses one ADR from bytes, which today exists only as the unexported `parse`
   (`adr.go:208`). `internal/audit` uses it against blob text, and `statusOf` (`audit.go:487`)
   and `domainsOf` (`audit.go:399`) are deleted. Audit reads history rather than the working
   tree, so it takes the seam rather than the `Corpus`; what it shares is the parser and the
   frontmatter schema, which is where the duplication actually was.

6. **Raw-byte access is enumerated and closed.** Exactly two consumers legitimately work below
   the semantic layer: `internal/migrate`, which performs offset surgery using `DecisionEnd`
   to append bookkeeping items, and the retired-key scan at `supersession.go:46`, which looks
   for a frontmatter key the parser deliberately drops. Both keep raw access through a named
   accessor on the view. Any third such consumer is a signal that the view is missing a
   question, not a licence to re-read the file.

## Invariants

- `invariant: corpus-parsed-once` - a single `awf check`, `awf sync`, or `awf context`
  invocation parses the decisions directory exactly once.
- `invariant: corpus-answers-every-category` - the corpus view exposes state, decision-item,
  supersession, declared-invariant, metadata, and existence queries, and ADR-0129's
  anchor-coverage model is reachable through it rather than constructed independently.
- `invariant: corpus-owns-status-literals` - no file outside `internal/adr` compares an ADR
  status against a string literal or tests it with a status prefix, save `awf context`'s
  Tier-2 exclusion as ADR-0129 item 4 enumerates.
- `invariant: corpus-single-identity-key` - every cross-package function that identifies an ADR
  does so by number; no exported signature returns or accepts a filename as an ADR identity.
- `invariant: audit-shares-adr-parser` - `internal/audit` parses ADR frontmatter through
  `internal/adr`'s exported bytes-level seam, and declares no frontmatter struct of its own.
- `invariant: corpus-raw-access-enumerated` - raw-byte access to an ADR file happens in exactly
  two places, the migration's offset surgery and the retired-key frontmatter scan, both through
  a named accessor rather than an ad-hoc re-read.

## Consequences

- The eight-parse `awf check` becomes a one-parse `awf check`. The gain is correctness before
  speed: a re-parse between two checks is a window in which two consumers can disagree about
  the corpus, and at 129 ADRs the wall-clock saving is not the reason to do it.
- ADR-0129 lands as a facet of an existing view instead of a fifth structure threaded by hand.
  That is the ordering argument for doing these together, and it is also why this ADR cannot
  be deferred past ADR-0129 without paying for the threading twice.
- `internal/adr` gains exported surface (the `Corpus` type, its query methods, the bytes-level
  parse seam) while losing some (`SupersessionIndex`, `Override`, `Label()`, per ADR-0129 item
  4). Net public API grows; the package becomes the single place ADR semantics live, which is
  the point, but it also becomes a larger dependency for every other package.
- Roughly 90 field reads across 10 loader sites move behind the view. Most are mechanical, but
  `supersession.go` carries about 58 of them and is simultaneously being rewritten by ADR-0128
  and ADR-0129, so that file is the plan's coupling point and its riskiest single step.
- Deleting `statusOf` and `domainsOf` puts `internal/audit` on the same frontmatter schema as
  everything else, so a future frontmatter change stops needing to be made twice. It also
  means a malformed historical ADR now fails audit through the shared parser rather than
  audit's more forgiving ad-hoc reader; audit rules must keep tolerating a parse failure on an
  old commit rather than aborting the run.
- Enumerating raw access makes the two legitimate cases visible and any third one obviously
  wrong. It does not prevent a consumer from reaching for `Path` and calling `os.ReadFile`;
  the invariant is the deterrent, and it is greppable.
- Nothing here changes an authored artifact. This ADR is invisible in `docs/`, which is why it
  is worth recording: an architectural commitment with no surface in the rendered output is
  exactly the kind that erodes without a decision behind it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Leave the parsing as is and only unify supersession | ADR-0129's model would become a fifth structure over the same records, threaded by hand to ten load sites. The threading cost is paid either way; paying it once buys the rest. |
| Fold this into ADR-0129 | ADR-0129 is review-converged and specifically about anchors and coverage. Parse-once-and-share is a separate commitment that outlives any particular supersession encoding, and stays citable when that encoding changes. |
| Cache `ParseDir` results behind the existing API | Invisible coupling: callers would silently share mutable slices, and a cache keyed on directory path invites staleness across a sync that rewrites ADRs. Threading a value makes the sharing explicit. |
| Keep `internal/audit` on its own parsers | Its inputs are git blobs rather than the working tree, which is a real difference, but the duplication is in the frontmatter schema, not the source of bytes. A bytes-level seam shares exactly the part that was duplicated. |
| Route `internal/migrate` through the view too | It performs byte-offset surgery to append Decision items; a semantic view cannot express that without exposing offsets, which would defeat the abstraction for its one consumer. |
| Make every ADR field unexported and force accessor use | Mechanically stronger, but it churns every renderer and test for a guarantee the greppable status-literal invariant already provides at a fraction of the cost. |
