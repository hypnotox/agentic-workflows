---
status: Proposed
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [pitfalls, docs, context, rendering, feedback-loop, sidecar-derived-doc]
related: [67, 86, 88, 89, 90, 92, 98]
domains: [rendering, tooling]
---
# ADR-0099: Structured Domain-Tagged Pitfalls Surfaced by awf context

## Context

`docs/pitfalls.md` is the durable home for tricky, non-mechanically-checkable knowledge — the
retrospective's rung-4 destination (ADR-0067). Today it is a single toggleable rendered doc
authored as one free-prose part blob (`.awf/docs/parts/pitfalls/entries.md`, ~520 lines, entries
delimited by `## ` headings). That shape has one structural weakness: the knowledge is invisible
to the workflow at the moment it is most useful. An agent editing `internal/git/` cannot be told
"there is a pitfall about go-git repo-opening in this area" — the pitfall is undifferentiated
prose with no machine-readable per-entry metadata.

Meanwhile `awf context <path>` (ADR-0092) already reflects committed `.awf/` config back to the
workflow: for a set of repo-relative paths it reports the owning domains, backed invariants,
related ADRs, and — since ADR-0098 — the plans linked through those ADRs, all from one read-only
`ContextResult`. Pitfalls are the obvious missing surface: the same path→domain resolution that
finds an area's ADRs and plans could find its pitfalls, if each pitfall declared the domain(s) it
belongs to. Surfacing the relevant tricky knowledge at edit time is exactly the "reflect config
back to the workflow" philosophy ADR-0092 established.

The enabling machinery already exists. ADR-0089 introduced the **sidecar-derived doc** model: a
`renderKindSpec.transform` computes a doc's rendered content from the artifact's own sidecar
`data` upstream of both render and config-hash, keeping the doc in the ordinary `RenderAll`/lock
drift model. The glossary is its first occupant (`.awf/docs/glossary.yaml` `data.terms` →
`docs/glossary.md`). Pitfalls is a natural second occupant: structured entries in a sidecar,
rendered to the same skimmable single doc, and now *also* readable by `awf context`.

This decision is the **artifact** half only. The *convention* — what belongs in a pitfall, and
the promotion ladder that routes a recurring finding to the strongest deterministic rung — already
lives in ADR-0067 and is unchanged. Structuring rung 4 is orthogonal to that ladder: it makes
rung-4 knowledge discoverable, not a graded gate. In particular it does **not** resurrect
ADR-0067's ruled-out "findings ledger" (Approach B) — there is no recurrence counter, no
auto-promotion, no deterministic recurrence signal; promotion stays a main-thread judgment call.

Grounding surfaced the constraints this decision must respect:

- **The `entries` section is a stub.** Like the glossary before it (ADR-0089), a data-authored
  body cannot sit in a `stub` section — the unauthored-content advisory (ADR-0070) would demand a
  part that no longer exists. The template must move to plain text with an empty-data fallback.
- **`unused-data-drift` (ADR-0086) keys off the template's textual reference**, not the transform.
  The rendered template must literally contain `.data.pitfalls` or the sidecar key false-flags as
  unused.
- **The example adopter carries the old shape.** `examples/sundial` enables `pitfalls` as a part
  (ADR-0090), and is re-rendered by `./x sync`, not `awf upgrade` — so a schema migration will not
  touch it. Its `pitfalls/entries.md` must be hand-converted in the implementing commit or the
  example's zero-notes/zero-drift gate (ADR-0090) fails.
- **`configspec` parity (ADR-0088)** requires a description entry for the new `data.pitfalls` key.
- **Adopter migration is a deliberate departure from ADR-0089's precedent.** ADR-0089 explicitly
  ruled out converting an authored part into `data` ("parsing hand-authored markdown is
  unreliable; a changelog recipe beats a fragile rewriter") and instead accepted breaking adopters
  with a changelog recipe and no migration. The distinguishing fact here is that pitfalls split on
  top-level `##` headings — a far narrower and more tractable transformation than parsing arbitrary
  glossary table framing — and the user asked to attempt it. The split is *not* absolutely
  reliable: a `##` line inside a fenced code block or a nested sub-heading in an adopter's body
  could still be mis-split, and "top-level heading" is a property of awf's own flat corpus, not one
  the migration can prove from raw markdown. The migration reduces that risk mechanically (it skips
  `##` lines inside fenced code) but the actual safety is **auditability**, not perfect mechanics:
  the per-entry provenance and mandatory review instruction (Decision item 6) mean a mis-split
  surfaces for human correction rather than landing silently. That is what makes the departure
  defensible where ADR-0089's silent-rewrite concern was not.

## Decision

1. **Model pitfalls as sidecar-derived doc data.** Pitfalls entries live in a new sidecar
   `.awf/docs/pitfalls.yaml` under `data.pitfalls`, an **ordered list** (authored order is the
   stable source of truth — a YAML sequence is deterministic without sorting, so no
   glossary-style sort-invariant is needed). Each entry is
   `{title: string, domains: []string, related: []int, body: string}` where `domains` and
   `related` are optional and `body` is a markdown block scalar.

2. **Render via the ADR-0089 transform seam.** A `renderKindSpec.transform` (a sibling of
   `glossaryTransform` in `internal/project/glossary.go`) replaces `data.pitfalls` with the
   assembled markdown — each entry as a `## <title>` section, an optional domain line, an optional
   `Related:` line of `ADR-NNNN` references (link-validated against `docs/decisions/` by
   `checkPitfalls`, but rendered as plain text — the transform receives only the sidecar and cannot
   resolve numbers to filenames, and plain `ADR-NNNN` is the convention `awf context`, the glossary,
   and the pitfall bodies already use) built from `related:`, and its `body` — computed upstream of
   render and config-hash so the doc stays in the ordinary drift model. Rendering `related:` gives
   the field a reader in the doc itself, so it is projected data rather than validate-only metadata.
   Malformed data (empty title, a newline in a title, empty body) is a hard render error.

3. **Retire the stub; render from plain template text.** The `pitfalls` `DocEntry` sections change
   from the single `entries` stub to plain framing sections plus a body that textually references
   `{{ with .data.pitfalls }} … {{ else }} … {{ end }}`, mirroring the glossary. The `{{ else }}`
   branch renders publication-safe placeholder prose under empty data. `pitfalls` stays a
   toggleable (`Mandatory: false`) doc — a singleton doc with many entries, toggled with
   `awf enable/disable doc pitfalls`; there is no per-entry `awf new` command.

4. **Surface pitfalls in `awf context`.** `ContextResult` gains a `Pitfalls []PitfallRef` field;
   `ContextFor` reads `.awf/docs/pitfalls.yaml` and includes every pitfall whose `domains:` names
   an owning domain of a queried path — surfacing by the pitfall's *own* domain tag, the way ADRs
   surface (via their frontmatter `domains`), not transitively via a link the way plans do. The
   human and `--json` renderings both derive from that one field. The read is pure; the command
   remains read-only, output-parity-preserving, and static-fallback-degrading (ADR-0092). A
   domainless pitfall (a cross-cutting or process note) is valid and simply never surfaces.

5. **Validate the data in `awf check`.** Each entry's `domains:` must resolve to a configured
   domain, and each `related:` number must resolve to an existing ADR under `docs/decisions/`
   (structurally identical to the plan→ADR link check, ADR-0098). Unparseable sidecar data is a
   hard check error. These join the render-time structural validation of item 2.

6. **Ship a schema migration that auto-splits the existing part.** A new schema-9 migration
   converts a present `.awf/docs/parts/pitfalls/entries.md` into `data.pitfalls` entries — each
   top-level `## ` heading *outside a fenced code block* becomes an entry's `title`, the text
   beneath becomes its `body`, and `domains`/`related` are left empty for the adopter to fill in.
   Skipping fenced code narrows the residual mis-split risk; auditability (below) covers the rest.
   The migration **prints one
   provenance line per created entry** and closes with an explicit instruction to review the split
   and tag domains; it **deletes** the now-orphaned part file (else it becomes ADR-0086
   orphaned-part drift); and it is idempotent (a no-op once the sidecar exists / the part is
   absent) and atomic (`manifest.WriteFileAtomic`, ADR-0076). It registers a `minVersionBySchema`
   entry and a version bump (ADR-0049).

7. **Convert awf's own and the example adopter's pitfalls by hand in the implementing commit.**
   Both `.awf/docs/parts/pitfalls/entries.md` and `examples/sundial/.awf/docs/parts/pitfalls/
   entries.md` are hand-converted to their `pitfalls.yaml` sidecars with domains tagged, mirroring
   how ADR-0089 hand-converted awf's own glossary. `configspec` gains the `data.pitfalls`
   description entry (ADR-0088).

## Invariants

Each slug below is backed by a `// invariant: <slug>` marker (comment or test) in the implementing
commit, per the backed-invariants rule (ADR-0008); `awf check` enforces them once this ADR is
`Implemented`.

- `inv: pitfall-data-validated` — `awf check` fails on unparseable `.awf/docs/pitfalls.yaml`
  data, and on an entry with an empty/newline-bearing title or an empty body; the transform that
  renders `docs/pitfalls.md` is a hard error on the same malformed data.
- `inv: pitfall-domains-resolved` — `awf check` fails a pitfall entry whose `domains:` names a
  domain not configured in the project; an entry with no `domains:` is valid and never surfaces
  via `awf context`.
- `inv: pitfall-adr-link-resolved` — `awf check` fails a pitfall entry whose `related:` names an
  ADR number with no matching file under `docs/decisions/`.
- `inv: context-surfaces-pitfalls` — `awf context` surfaces every pitfall whose `domains:` names
  an owning domain of a queried path, on the single `ContextResult`, preserving the ADR-0092
  read-only / output-parity / static-fallback guarantees.

## Consequences

Easier:
- Tricky knowledge is delivered at edit time: `awf context <path>` now answers "what should I know
  about this area" with the relevant pitfalls alongside its domains, invariants, ADRs, and plans —
  closing a gap between the retrospective's rung-4 output and the workflow that needs it.
- Pitfalls become machine-readable without a new artifact kind, a new directory, or new
  scaffolding: they reuse the proven sidecar-derived-doc seam (ADR-0089), and the change is one
  more occupant of an existing pattern rather than new subsystem.
- The single-doc reading experience is preserved — `docs/pitfalls.md` still renders as one
  skimmable document; only its authoring source and its queryability change.

Harder / accepted trade-offs:
- **Breaking for adopters**, mitigated by the auto-split migration. An adopter's hand-authored
  `pitfalls/entries.md` is converted to `data.pitfalls` with empty domains; the migration surfaces
  exactly what it did and instructs a review, but tagging domains is manual follow-up work. Adopter
  bodies now live as YAML block scalars.
- **No deterministic "untagged" nag.** Because the stub section is retired, a migrated-but-untagged
  entry does not trip the ADR-0070 unauthored-content advisory. The migration's review instruction
  is the only prompt to tag domains — an accepted consequence of domains being optional.
- **`awf context` gains a new input class.** It reads a doc sidecar for the first time (today it
  reads only domain sidecars and ADR/plan frontmatter). The read is pure and preserves read-only,
  but it is genuinely new surface in the assembly.
- **The implementing commit must convert two parts by hand** (awf's own and `examples/sundial`),
  because the migration runs under `awf upgrade`, not the `./x sync` that re-renders the example.
- **The migration departs from ADR-0089's ruled-out row.** Justified by the narrower, more
  tractable `##`-heading split and made safe by auditability (per-entry provenance + review
  instruction), not by claiming perfect mechanics — a deliberate case-specific exception that the
  ADR records rather than hides.

Ruled out / deferred:
- **`awf new pitfall`** — not applicable to a singleton doc with many entries; entries are authored
  directly into the sidecar (by the retrospective or by hand).
- **A `promotion:` field** (rung-owed + occurrence-count) that would give deferred promotions a
  durable, surfaced home. Deferred as a follow-up; it must **not** become an automatic promotion
  signal without revisiting ADR-0067, since an occurrence-count-driven signal is exactly that
  ADR's ruled-out Approach B.
- **A per-entry `paths:` surfacing key** — surfacing is domains-only, for consistency with the
  path→domain→artifact model; a pitfall about a specific file tags that file's domain.

Downstream work unblocked: an implementation plan covering the sidecar model + transform + template
retirement of the stub; the `ContextResult.Pitfalls` field + `ContextFor` reader + `awf context`
rendering; the four new `inv:` slugs backed with markers + tests; the `awf check`
domain/ADR-link/parse validation; the schema-9 auto-split migration
(+ `minVersionBySchema` + version bump); the hand-conversion of awf's own and the example adopter's
parts; the `configspec` data-key entry; and the doc currency (AGENTS.md invariants list, the
`rendering`/`tooling` domain current-state parts, `config-reference.md`, glossary, and a changelog
`[Unreleased]` entry with the adopter migration recipe). When this ADR flips to `Implemented`, the
same commit regenerates `docs/decisions/ACTIVE.md`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Directory of per-entry files (the ADR/plan model): `docs/pitfalls/<slug>.md` with frontmatter + `internal/pitfall` ParseDir + `awf new pitfall` | Richer bodies and natural scaffolding, but the largest new surface (new artifact kind, directory, index generation, scaffolding, validation) for entries that are often short, and it converts the single skimmable doc into a directory. Model A reuses an existing seam for the actual goal (domain-tagging + context surfacing) at a fraction of the cost. |
| In-doc HTML-comment metadata under each `##` heading (keep one authored file, add a metadata line per entry) | A third bespoke parsing dialect alongside frontmatter-files and sidecar-data; fragile and against the project's grain, which prefers frontmatter or sidecar `data`. |
| Break adopters with a changelog recipe and ship no migration (ADR-0089's chosen path for the glossary) | Rejected *for this case*: the top-level `##`-heading split is materially narrower and more tractable than glossary table parsing, and an auditable auto-split (per-entry provenance + mandatory review, with fenced-code `##` lines skipped) contains the residual mis-split risk rather than relying on perfect mechanics — so the ADR-0089 "fragile rewriter" objection is met by auditability, not by a reliability claim; the user asked to attempt the migration; and it is materially friendlier than hand-reconstructing a ~520-line doc. |
| Make pitfalls first-class as a persisted per-effort retrospective document | Explored and rejected upstream: it re-opens ADR-0067's ruled-out findings-ledger (Approach B), adds an incentive regression (a dumping ground that lets authors skip promotion), and duplicates homes the plan Notes tail and pitfalls.md already provide. Structuring the existing rung-4 home is the surgical change. |
| Surface pitfalls transitively via linked ADRs (as plans surface) | A pitfall's relevance is to a *code area*, not to a decision; tagging its own `domains` (like an ADR) is the direct model. `related:` ADRs are rendered as a plain `ADR-NNNN` line in the doc and link-validated, but are not a `awf context` surfacing path. |
