---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [context, config, adr-system, invariants, tooling]
related: [86, 88, 92, 98, 99, 102]
domains: [config, adr-system, invariants, tooling]
---
# ADR-0103: Governed Tag Vocabulary and Metadata Revival

## Context

`awf context <paths>` (ADR-0092) reflects committed `.awf/` config back to the workflow: for a set
of repo-relative paths it reports the owning domains, backed invariants, related ADRs, the plans
linked through those ADRs (ADR-0098), and the pitfalls owning those domains (ADR-0099). A real run
against three source files exposed the weakness the whole `awf context` relevance rework targets:
relatedness is resolved by **domain membership**, so any file under `internal/render/**` pulls
*every* ADR and pitfall tagged `rendering` — a three-file query dumped ~90 ADRs and ~40 pitfalls,
essentially the entire corpus. Coarse relatedness drowns the signal. The fix is a finer,
path-precise relevance currency; **this ADR builds and governs that currency but does not yet spend
it.** The relevance-tiering consumer — spending tags to narrow `awf context` output and reconciling
the surfacing invariants of ADR-0098/0099 — is a deliberately separate follow-up ADR, because it is
a distinct load-bearing decision (what to surface and how to tier it) that should settle on its own
merits once the currency exists. This is the second slice of a three-slice effort; the first shipped
`awf context --uncovered` (ADR-0102), the domain-coverage report that finds the unowned code the
tiering consumer will later need domains for.

Grounding the current tree surfaced the enabling facts:

- **`tags:` and `related:` are already authored but dead.** Every ADR carries `tags: [...]` (a
  cross-cutting keyword list, finer than `domains:`) and `related: [...]` (an ADR→ADR number list)
  in its frontmatter — 102 ADRs today. `internal/adr`'s `adrFrontmatter` lifts only
  `status`/`domains`/`superseded_by`/`retires_invariants`; both `tags:` and `related:` are parsed
  past and dropped. We never *lost* tags — we stopped *consuming* them while continuing to author
  them, and the authored corpus has drifted (`render` the tag vs `rendering` the domain;
  `documentation` vs `docs`; a long synonym tail of ~100 distinct labels).
- **Pitfalls have domains and `related:` but no tags.** `pitfallEntry` (`.awf/docs/pitfalls.yaml`,
  ADR-0099) carries `{title, domains, related, body}`; `pitfallEntryFrom` reads exactly those keys
  and silently ignores any other. A `tags:` field would be inert until parsed and validated.
- **The path→precision bridge already exists and is free.** `invariants.MarkersUnder` scans the
  actually-queried files for `invariant: <slug>` markers, and each ADR declares the invariant slugs
  it owns. So `path → present invariant markers → declaring ADR(s) → those ADRs' tags` is a precise,
  path-scoped tag set with **no new territory to maintain** — it rides existing markers rather than
  a new per-tag or per-invariant `paths:` field (which would duplicate territory and go stale). The
  tiering consumer will use this bridge; this ADR only has to make the tags it lands on trustworthy.
- **A vocabulary needs governance to be a currency.** For tags to be a relevance currency they must
  be a *closed, meaningful* set, not free text — otherwise `render`/`rendering` never match and the
  precise join silently under-reports. The project already governs the analogous surfaces: pitfall
  `domains:` must resolve to a configured domain (ADR-0099 `pitfall-domains-resolved`), and pitfall
  and plan `related:` must resolve to real ADRs (ADR-0099/0098 link checks). A tag vocabulary is the
  same shape of governance, one rung up.

Grounding also fixed the boundaries this decision must respect:

- **The corpus normalization is a one-time in-repo edit, not a schema migration.** `internal/migrate`
  is *adopter-facing*: it runs on every adopter tree and only ever writes structural ports under
  `.awf/`. awf's ADRs live in `docs/decisions/` (outside `.awf/`), and adopters author their *own*
  ADRs and their *own* tags — a hardcoded awf synonym map shipped as a migration would silently
  rewrite an adopter's tags into awf's vocabulary. So normalizing awf's ~102 ADRs and ~43 pitfalls
  to the curated vocabulary is a plain repo commit governed by the new check, never a migration.
- **The new config key is additive and absent-safe, so no schema bump.** Schema generation
  (`internal/migrate.Current()`, at 9) is an explicit integer bumped only when a migration is
  registered — it is not derived from the config struct shape. A top-level `tags:` map that
  degrades to inert-when-empty adds no migration and needs no `minVersionBySchema` bump (ADR-0049).
- **`configspec` parity and the config reference must cover the key** (ADR-0088): a `map[string]string`
  top-level field is one freeform-namespace leaf (like `vars`), requiring one `configspec` `KeyEntry`
  and a `config-reference.md` regeneration. A top-level config key consumed by Go code is outside
  ADR-0086's authored-but-unconsumed rule (which keys off `vars`/`data` template references).

## Decision

1. **Lift `tags:` and `related:` into `adr.ADR`.** `adrFrontmatter` and `adr.ADR` gain
   `Tags []string` (frontmatter `tags:`) and `Related []int` (frontmatter `related:`), parsed by the
   existing `parse`. This is purely additive parsing — the fields' *consumers* for relevance tiering
   are the follow-up ADR; this ADR consumes them only for governance (items 4–5). Absent fields
   parse to `nil` and render nothing.

2. **Give pitfall entries a `tags:` field.** `pitfallEntry` gains `Tags []string`, parsed by
   `pitfallEntryFrom` from an optional `tags:` list-of-strings (same shape discipline as `domains:`;
   a non-list or non-string element is a hard parse error, joining the ADR-0099 `pitfall-data-validated`
   structural family). The render transform for `docs/pitfalls.md` is unchanged in output — tags are
   validate-and-consume metadata for `awf context`, not rendered prose, in this slice.

3. **Introduce a governed vocabulary as a top-level `tags:` config key.** `config.Config` gains
   `Tags map[string]string` (`yaml:"tags"`) — a map from tag name to a one-line meaning. The meaning
   is load-bearing: it is the human-readable "what this tag preserves" that a reader (and the
   config reference) needs, and it forces each tag to be a deliberate, documented member rather than
   an accidental typo. The key is opt-in: an absent or empty vocabulary disables governance (item 4),
   keeping the feature publication-safe for adopters who have not curated one. `configspec` gains its
   one `KeyEntry`, and `config-reference.md` is regenerated (ADR-0088).

4. **Govern tag membership in `awf check`.** When the `tags:` vocabulary is non-empty, `awf check`
   fails on any tag used by an ADR (frontmatter `tags:`) or a pitfall (`tags:`) that is not a declared
   member of the vocabulary, and on any vocabulary entry whose meaning is empty. When the vocabulary
   is empty or absent, the membership rule is inert (no findings) — the opt-in degradation. This
   mirrors `pitfall-domains-resolved` structurally: a used label must resolve to a configured member.

5. **Resolve ADR `related:` links in `awf check`.** Now that `related:` is parsed, `awf check` fails
   an ADR whose `related:` names an ADR number with no matching file under `docs/decisions/` —
   structurally identical to the pitfall (`pitfall-adr-link-resolved`, ADR-0099) and plan
   (`plan-adr-link-resolved`, ADR-0098) link checks. This is unconditional (independent of the tag
   vocabulary): a dangling `related:` is drift regardless of whether tags are curated.

6. **Curate a tight vocabulary and normalize awf's own corpus in the implementing commit.** A
   deliberate ~30–40 tag vocabulary is authored into awf's `.awf/config.yaml` `tags:`, each with a
   one-line meaning, merging the drifted synonyms (`render`→`rendering`, `documentation`→`docs`,
   `query`→`context`, and the rest of the long tail) into their canonical members. awf's own ~102
   ADRs and ~43 pitfalls are re-tagged to the curated set in the same commit — a one-time in-repo
   edit, **not** a schema migration (per Context). The curation is quality-first (a tight, meaningful
   set), not a mechanical union of every label in use.

## Invariants

Each slug below is backed by a `// invariant: <slug>` marker (comment or test) in the implementing
commit, per the backed-invariants rule (ADR-0008); `awf check` enforces them once this ADR is
`Implemented`.

- `inv: tag-vocabulary-governed` — with a non-empty `tags:` vocabulary, `awf check` fails on any tag
  used by an ADR or a pitfall that is not a declared vocabulary member, and on any vocabulary entry
  whose meaning is empty; with an empty or absent vocabulary the membership rule is inert (opt-in
  degradation).
- `inv: adr-related-link-resolved` — `awf check` fails an ADR whose `related:` names an ADR number
  with no matching file under `docs/decisions/`; this holds independent of the tag vocabulary.

## Consequences

Easier:
- Tags become a trustworthy, closed, documented relevance currency — the precondition the follow-up
  tiering ADR needs to replace coarse domain-membership relatedness with a path-precise
  invariant→declaring-ADR→tags join, without which that consumer cannot be built.
- Two authored-but-dead frontmatter fields (`tags:`, `related:`) are revived into live, governed
  metadata; the immediate governance win (dangling-`related:` detection, tag-typo detection) lands
  now, before any tiering consumer exists.
- The governance reuses proven shapes — `configspec` parity, the resolve-against-a-configured-set
  check family (`pitfall-domains-resolved`, `pitfall-adr-link-resolved`, `plan-adr-link-resolved`) —
  rather than new subsystems.

Harder / accepted trade-offs:
- **A curated vocabulary is now a maintained artifact.** Adding a genuinely new cross-cutting concept
  means adding a vocabulary member with a meaning; using an undeclared tag is a check failure, not a
  silent add. This friction is the point (it keeps the set closed and meaningful), but it is real
  ongoing cost for awf's own tree.
- **A one-time re-tagging of the whole corpus** touches ~102 ADRs and ~43 pitfalls in a single
  commit. It is large but mechanical-with-judgment (synonym merges), auditable in review, and never
  repeated. Because it is not a migration, adopters are unaffected and untouched.
- **The vocabulary is opt-in, so an adopter gets no governance until they curate one.** An adopter
  with authored ADR tags and an empty `tags:` config sees no findings (correct — awf must not impose
  its vocabulary), but also no protection. Governance is a deliberate adopter choice, consistent with
  the invariants/domains opt-in surfaces.

Ruled out / deferred (to the follow-up tiering ADR, not this one):
- **Spending tags in `awf context`** — the relevance-tiering consumer (path→invariant→declaring-ADR
  precise tag set → tiered ADR/pitfall surfacing), the many-to-one slug→declaring-ADR tag union with
  a Superseded/retired filter, reviving `related:` as a Tier-2 signal, retiring
  `context-surfaces-pitfalls` (ADR-0099) and reconciling `context-surfaces-linked-plans` (ADR-0098),
  and output compaction. This ADR changes *nothing* about `awf context` output on purpose — it only
  makes the currency exist and be trustworthy.
- **Path-scoped tags** (a `paths:` field on tags, invariants, or domains) — rejected as duplicated,
  stale-prone territory; path precision rides existing invariant markers, not a new declaration.
- **Invariant-level tags** — invariants inherit their tags from their declaring ADR (ADRs are the
  topical unit); no per-invariant tag field.
- **Collapsing domains into tags** — domains stay a distinct coarse unit (they carry the domain
  docs, indices, and staleness audit); tags are an orthogonal finer axis.

Downstream work unblocked by this ADR: an implementation plan covering the `adr.ADR` `Tags`/`Related`
lift; the pitfall `tags:` parse; the `config.Tags` key + `configspec` entry + `config-reference.md`
regeneration; the two `awf check` rules (vocabulary governance + ADR `related:` resolution) backed
with `inv:` markers and tests; the curated vocabulary authored into `.awf/config.yaml`; the one-time
corpus re-tagging of awf's ADRs and pitfalls; and doc currency (the AGENTS.md invariants list, the
`config`/`invariants`/`tooling` domain current-state parts, `config-reference.md`, and a changelog
`[Unreleased]` entry). When this ADR flips to `Implemented`, the same commit regenerates
`docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Fold tiering into this ADR (one ADR: vocabulary + consumer) | Two distinct load-bearing decisions — *what the currency is* and *how to spend it* — with independent risk. Bundling them makes the vocabulary hostage to unresolved tiering questions (retirement of ADR-0098/0099 surfacing, union rules, compaction) and produces one oversized, hard-to-review change. Slicing lets the governance land and be dogfooded before the consumer is designed. |
| Free-text tags, no vocabulary | The drifted corpus (`render`/`rendering`, ~100 labels) *is* the failure mode: without a closed set the precise join silently under-reports and tags cannot be a currency. Governance is the whole point. |
| Per-tag / per-invariant / per-domain `paths:` field for path precision | Duplicates territory the invariant markers already express; a second source of path truth goes stale. The `invariant-marker → declaring-ADR → tags` bridge is free and self-maintaining. |
| Normalize the ADR corpus via a schema migration | `internal/migrate` is adopter-facing and writes only `.awf/`; a hardcoded synonym map would rewrite adopters' own tags into awf's vocabulary. awf's `docs/decisions/` corpus is normalized by a plain in-repo commit, governed by the new check. |
| Seed-union vocabulary (auto-collect every label in use, dedupe) | Produces a large, meaning-less set that preserves the synonym drift rather than resolving it. The user chose curate-tight + normalize-corpus: full quality over mechanical union. |
