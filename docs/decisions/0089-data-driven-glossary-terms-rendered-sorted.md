---
status: Proposed
date: 2026-07-10
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [rendering, docs, catalog, config]
related: [11, 45, 61, 80, 82, 86, 88]
domains: [rendering, config]
---
# ADR-0089: Data-driven glossary terms rendered sorted

## Context

The glossary catalog doc renders a hand-authored markdown table from a single stub
section (`terms`), whose "keep it sorted" contract lives in prose only. It broke twice in
three days in awf's own repository (restored by one commit, re-broken the next session),
which forced a repo-local gate test (`TestGlossaryTermsSorted`) — enforcement that only
this repo gets, while adopters authoring the same table have nothing. The user direction:
"rework the glossary to simply always render it sorted. Entries should be config values
… enables us to enforce some things on the content, e.g. deterministic sorting in the
first place." Breaking the existing adopters is accepted — only fleet enables the
glossary (with an authored `terms.md` part); go-php does not.

Grounding discoveries shaping the design:

- The hand-authored table also has a live correctness bug: a `|` inside a code span
  (present in this repo's own entries) splits a GFM table cell — machine-built rows can
  escape it; prose convention cannot.
- The sidecar `data:` channel already provides everything the entries need: strict
  parsing rejects duplicate map keys natively (verified empirically against the actual
  decode path, `internal/config/config.go:173-178`), and the raw sidecar folds into the
  artifact config hash (`internal/project/confighash.go:52`), so a term edit reflags the
  doc with no new machinery.
- Two computed-doc models now exist to choose between. Domain docs regenerate outside
  the lock model because their content derives from external repo state (ADR frontmatter);
  the concurrently-proposed config reference (ADR-0088) extends that regeneration model.
  Glossary content derives solely from the artifact's own sidecar, which the lock's
  config hash already covers — so the glossary must stay in the ordinary
  `RenderAll`/lock model, and this ADR deliberately does not touch the regeneration
  path.
- The blind-spot lesson of ADR-0045 applies: whatever transform builds the table must
  sit upstream of *both* the render and `artifactConfigHash`, so a change to the
  builder's own logic (sorting, escaping) in a newer binary reflags the doc rather than
  leaving `check` clean while `sync` would rewrite.
- A template-side `{{ with .data.terms }}` reference keeps every existing oracle working
  with zero carve-outs: the ADR-0086 unused-data scan matches `.data.terms` textually in
  the assembled source; `TestDocsSectionParity` renders doc templates under empty data
  and fails on `<no value>`, which the `with/else` guard satisfies while doubling as the
  ADR-0045 graceful-degradation branch.
- Empty section defaults are supported and precedented (`internal/render/section.go`
  empty-body capture; three shipped skill sections). Stub notes key on attribute plus
  part existence, so a data-authored table must not sit in a stub section — the note
  would demand a part that is no longer how authoring works.
- yaml.v3 hands the transform two map shapes: `map[string]any` normally, but
  `map[interface{}]interface{}` the moment any key is non-string. Meanings arrive as
  `any` (string, nil for null, int for a bare number); block scalars carry a trailing
  newline.
- The ADR-0080 sweep and fallback-case guards cover skills and agents only — a doc
  template's conditional gets no machine-forced degraded-prose case, so the glossary's
  must be added voluntarily.
- ADR-0011 declares the glossary's section taxonomy as `terms` and reasons about its
  skeleton-prompt default; this ADR changes both (partial-item supersedence via
  `related`; ADR-0011 stays Implemented).
- The unexecuted ADR-0088 plan contains a task adding two rows to the glossary part this
  ADR deletes; whichever effort lands second owes a resync (the task becomes two
  `data.terms` entries).

## Decision

1. **Glossary entries are sidecar data.** The glossary doc's terms live in
   `.awf/docs/glossary.yaml` under `data.terms`, a YAML map of `term: meaning` string
   pairs. No new config surface: it is the existing per-artifact data channel, with
   duplicate exact keys already rejected by the strict decoder and term edits already
   reflagging the doc through the sidecar-in-confighash fold. The catalog ships no
   default for `terms` — glossary content is inherently project-specific.

2. **The table is forced, machine-built, and always sorted.** The glossary template's
   table renders from plain (non-section) template text — `{{ with .data.terms }}` —
   so no convention part can override it. A transform in the docs render path validates
   the authored map and replaces the `terms` value with the finished table markdown
   *upstream of both* `renderTarget` and `artifactConfigHash` (the ADR-0045
   both-consumers pattern): rows sort by `strings.ToLower(term)` with a byte-order
   tiebreak, and `|` is escaped in terms and meanings. The doc remains in the ordinary
   `RenderAll`/lock model — this is sidecar-derived content, distinct from the
   regeneration model used for repo-state-derived docs (domain indexes, ADR-0088).

3. **Content violations are hard render errors naming the sidecar.** After trimming
   surrounding whitespace: an empty term, an empty/null/non-string meaning, an interior
   newline in a term or meaning (YAML's quoted and explicit-key syntaxes permit
   newlines in map keys, and either breaks a GFM table row), a non-string map key (the
   `map[interface{}]interface{}` shape), and two terms equal case-insensitively all
   fail the render with the offending key and `.awf/docs/glossary.yaml` named. Config
   smell is an error state, never a silent repair.

4. **Framing is two empty-default sections; absent data degrades coherently.** The
   glossary's catalog sections become `prepend` and `append` — plain (non-stub), empty
   defaults, rendered above and below the table as the only override surface. There is
   no fixed intro prose: a project wanting one authors the `prepend` part. With
   `data.terms` absent or empty, the `else` branch renders a single coherent placeholder
   line that names the authoring surface (`data.terms` in `.awf/docs/glossary.yaml`) —
   never a zero-row table — since the stub-note channel disappears with the stub
   section. This supersedes ADR-0011's `glossary | terms` taxonomy row and its
   skeleton-prompt reasoning for this doc (partial item; ADR-0011 stays Implemented).
   ADR-0011's rule that doc *section defaults* interpolate no data tokens is
   deliberately not overridden: the `{{ with .data.terms }}` reference sits in plain
   non-section template text, and the `with/else` guard preserves the
   publication-safety that rule protects.

5. **Coverage travels with the change.** The new conditional gets a hand-authored
   `unsetFallbackCases` entry pinning the degraded prose, and a `TestGlossaryTemplate`
   golden listed in `nonArtifactGoldens` — voluntary additions, since the ADR-0080
   guards cover skills and agents only. A regression test pins that two sidecars with
   the same entries in different authored order render byte-identically. This repo
   converts its own entries to `data.terms`, deletes its `terms.md` part and the
   now-obsolete `TestGlossaryTermsSorted` gate test (enforcement moved into the
   renderer), and re-adds its intro line as a `prepend` part. The implementing change
   updates the rendering-domain (and, where its narration shifts, the config-domain)
   current-state part in the same change, and flips this ADR's status and regenerates
   `docs/decisions/ACTIVE.md` via `./x sync` in its final commit.

## Invariants

- `inv: glossary-terms-sorted` — the rendered glossary table's rows are ordered by
  case-insensitive term (byte-order tiebreak) regardless of authored map order; equal
  entry sets render byte-identically.
- `inv: glossary-terms-validated` — an empty term, an empty/null/non-string meaning, an
  interior newline in a term or meaning, a non-string map key, or a case-insensitive
  duplicate term in `data.terms` fails the render with the sidecar path and offending
  key named.
- `inv: glossary-table-forced` — no convention part can replace the rendered terms
  table; the only part-override surfaces on the glossary doc are the `prepend` and
  `append` sections.
- Textual: the glossary stays in the `RenderAll`/lock drift model; moving it to the
  regeneration model is a successor-ADR act.

## Consequences

Easier:
- Sort order, cell escaping, and duplicate detection are properties of the renderer, not
  of author discipline — for every adopter, not just this repo. The repo-local
  `TestGlossaryTermsSorted` and its two-regressions-in-three-days history are retired.
- Term edits live in one reviewable YAML map; the ADR-0086 closed tree and unused-data
  checks apply to them with no carve-outs.

Harder / accepted trade-offs:
- **Breaking for fleet** (accepted by the user): after upgrade, its authored
  `terms.md` part becomes orphaned-part drift with the section gone; the remedy —
  convert rows to `data.terms`, optionally keep framing prose as `prepend` — ships in
  the changelog Breaking entry. go-php is unaffected (glossary not enabled).
- Meanings are single-line YAML strings; genuinely multi-line definitions are out of
  scope for a table-shaped glossary (the hard error makes the constraint explicit).
- Escaping `|` as `\|` inside a code span renders the backslash literally on some
  viewers — a known GFM-table wart, strictly better than the current silently-broken
  cell.
- A future change to the table builder's output (sort collation, escaping) reflags every
  adopter's glossary as stale on their next check — by design, via the post-transform
  config hash; the normal `awf sync` resolves it.
- The `awf:edit` pointer comments for `prepend`/`append` render even when both sections
  are empty — two comment lines of cosmetic noise in the published doc, consistent with
  every other managed doc.
- The unexecuted ADR-0088 plan edits the deleted part; its resync owes a one-line task
  rewrite (two `data.terms` entries instead of two table rows) — and, once both land,
  ADR-0088's configspec data-parity check will demand a description entry for the
  glossary's `terms` data key; whichever effort lands second inherits that obligation
  (caught mechanically by that gate, named here so the resync scope is complete).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the prose part, add a sort check (advisory or failing) to `awf check` | A content lint on adopter-authored prose is a new check category; the renderer owning the format eliminates the failure mode instead of reporting it. |
| Sorted-by-default section that a part can still override | "Always sorted" becomes "sorted unless overridden"; the enforcement value the rework exists for evaporates. |
| Domain-doc/ADR-0088-style regeneration path | That model exists for content derived from external repo state; glossary content is the artifact's own sidecar data, already hash-covered — regeneration would discard working drift machinery. |
| Template-native map range (Go templates iterate maps key-sorted) | Byte-order collation only, no cell escaping, no validation — and the engine has no FuncMap to add them template-side; a Go-side transform does all three. |
| Entries as a YAML sequence of `{term, meaning}` objects | Loses native duplicate-key rejection; order-independence must then be asserted instead of being structurally meaningless. |
| Rejecting `\|` in meanings instead of escaping | This repo's own entries legitimately contain pipes inside code spans; rejection would ban real content. |
| Schema migration converting an authored `terms.md` part into `data.terms` | Parsing hand-authored markdown tables with arbitrary framing prose is unreliable; exactly one adopter is affected and the user explicitly accepted the break — a changelog recipe beats a fragile rewriter. |
