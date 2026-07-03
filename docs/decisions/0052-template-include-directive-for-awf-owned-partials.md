---
status: Proposed
date: 2026-07-03
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [render, templates, partials, drift]
related: [0001, 0015, 0027, 0034, 0045, 0046]
domains: [rendering]
---
# ADR-0052: Template Include Directive for Awf-Owned Partials

## Context

The three reviewer agent templates — `templates/agents/adr-reviewer.md.tmpl`,
`plan-reviewer.md.tmpl`, `code-reviewer.md.tmpl` — duplicate a "review-discipline spine"
(the **Finding schema**, **Classification rules**, **Dedup rule**, **Review procedure**, and
**Digest format** sections) near-verbatim. The block sits *outside* `awf:section` markers, so
it is neither overridable nor deduplicated — an edit to the shared discipline (e.g. the
classification vocabulary or the 3-round soft cap) must be hand-propagated to three files,
and the three drift apart silently. This is the last item on the reviewer-cleanup backlog and
has been deferred twice for want of a source-level sharing mechanism the render engine does
not have.

The engine parses each artifact standalone: the per-artifact pipeline is
`ParseSections → Assemble → Execute` (`internal/render/`), orchestrated in `renderTarget`
where raw template bytes are read (`internal/project/render.go:343`) and handed to the parser
(`:351`). There is no cross-file composition primitive — no `text/template` associated-template
set, no include. `awf:section`/`awf:edit`/`awf:end` (project-facing section overlay,
[ADR-0015](0015-in-file-provenance-and-convention-only-overrides.md)) and `awf:part` (the
raw-convention-part sentinel, [ADR-0034](0034-convention-parts-are-raw-input.md)) are the only
marker vocabulary today.

Grounding discoveries that shape the design:

- **Only the three reviewer agents carry the full spine.** No skill template duplicates the
  schema/procedure blocks; the `reviewing-*` skills reference the discipline as thin
  dispatchers. So the immediate consumer is small and single-purpose.
- **The differences between the three are ~4 localized points, not structural:** the
  finding-schema subject noun ("the ADR/plan/diff generally"), review-procedure step 1 (what to
  read), review-procedure step 5 (only the code-reviewer carries the "as new commits + run the
  gate before each fix commit + never `--amend`" variant), and the digest label + three summary
  bullet lines. Everything else is byte-identical. These points already have a home:
  per-artifact catalog `data:` defaults ([ADR-0045](0045-out-of-box-render-completeness.md)),
  which the reviewers already use (`focusItems`, `docCurrencyItems`, `correctnessTraps`) and
  which merge into `sc.Data` before *both* render and `artifactConfigHash`
  (`internal/project/render.go:172-175`).
- **The drift oracle is blind to a naive splice.** `TemplateHash = manifest.Hash(src)` hashes
  the *raw* pre-expansion `.tmpl` bytes (`internal/project/render.go:366`); `artifactConfigHash`
  hashes referenced vars/skills/scopes and consumed convention parts, not the assembled literal
  text. A partial spliced at render time but excluded from both hashes would let an edit to the
  partial change rendered output while `awf check` stays green and `awf sync` silently rewrites —
  violating the "`awf check` is the drift oracle" invariant. This is the same blind-spot class
  [ADR-0046](0046-skill-reference-integrity.md) and ADR-0051 had to close.
- **Nothing renders `templates/` by glob.** `RenderAll` is catalog-name-driven, so partial
  files are never mistaken for artifacts. The embed directive (`templates/embed.go:6`) does *not*
  list `partials`, and there are zero non-`.tmpl` files under `templates/` today, so
  `partials/*.md` is a clean new shape that must be added to the glob to be readable.
- Partials are awf-owned, embedded, templated content — squarely on the templated side of
  ADR-0034's raw-vs-templated boundary, not the project-supplied raw-convention side.

The user chose a **general** directive (usable by any awf template, not hard-wired to
reviewers) restricted to **awf-owned/embedded partials only** (no project-authored partials),
so the capability is reusable across the catalog while adding zero new project config or drift
surface.

## Decision

1. **`awf:include` directive.** A new pre-pass, `expandIncludes`, runs at the front of the
   per-artifact pipeline — before `ParseSections`, wrapping the raw source read in
   `renderTarget` (`internal/project/render.go:351`). It replaces each `<!-- awf:include NAME -->`
   line with the verbatim contents of the embedded, awf-owned partial file
   `templates/partials/NAME.md`. Because expansion precedes section-parsing, spliced text is
   thereafter indistinguishable from inline template content: `awf:section` overlay, the
   `<no value>` publication-safety check, and the dead-reference and dead-skill-reference scans
   (all post-Execute) apply to it unchanged, with no edits to `Assemble`/`Execute` or the scan
   code. The directive is general — available to any awf template — and its partial source is
   `templates/partials/`, which is added to the `go:embed` glob.

2. **The template hash covers the expanded source.** `TemplateHash` is computed over the
   *post-expansion* source, so an edit to any partial changes the lock `TemplateHash` of every
   artifact that includes it, and `awf check` flags those artifacts stale. This keeps the drift
   oracle authoritative across the splice and is the load-bearing reason the pre-pass hooks
   ahead of both parsing and hashing rather than being a cosmetic string substitution.

3. **Partials are awf-owned and embedded only.** There are no project-authored partials in this
   decision: partials live under `templates/partials/` in the binary like every other template
   default, so they carry no new `.awf/` config surface and no new lock/config-hash machinery —
   a partial edit is a binary change caught by item 2's expanded-source hash, exactly as a base
   template edit is today.

4. **Fail-loud v1 guards.** Three conditions are hard render errors, each named with the
   offending target: (a) an `awf:include` naming a partial file that does not exist; (b) a
   partial whose body itself contains an `awf:include` (nested includes are unsupported — no
   cycle detection is needed because none can be authored); (c) a partial whose body contains an
   `awf:section` or `awf:end` marker (overlay semantics across a splice boundary are
   unspecified). Lifting any guard is a later decision under its own ADR.

5. **First application: the reviewer spine.** The shared spine moves into awf-owned partials
   under `templates/partials/` (two partials, since the spine is two non-contiguous clusters
   split by the per-reviewer lens/focus sections), included from all three reviewer templates.
   The ~4 per-reviewer differences become per-artifact catalog `data:` defaults with generic
   `{{ with .data.X }}…{{ else }}…{{ end }}` fallbacks (the ADR-0045 mechanism), so an adopter's
   unset render still degrades to coherent prose. The defaults reproduce each reviewer's current
   wording exactly, so the migration is output-preserving — the three rendered agent files are
   byte-for-byte unchanged, which is the regression safety net.

## Invariants

- `inv: include-splice` — an `<!-- awf:include NAME -->` directive in an awf template renders
  the verbatim body of `templates/partials/NAME.md` in its place, expanded before section
  parsing.
- `inv: include-missing-fails` — an `awf:include` naming a nonexistent partial is a hard render
  error, not a silent empty splice.
- `inv: include-no-nested` — a partial whose body contains an `awf:include` directive is a hard
  render error.
- `inv: include-no-sections` — a partial whose body contains an `awf:section` or `awf:end`
  marker is a hard render error.
- `inv: include-in-templatehash` — an artifact's lock `TemplateHash` is computed over the
  post-expansion source, so editing an included partial changes that artifact's `TemplateHash`
  and `awf check` reports the artifact stale.

## Consequences

Easier:
- The review-discipline spine has a single source of truth; an edit to the shared schema,
  classification rules, dedup rule, procedure, or digest format propagates to all three
  reviewers automatically, and they can no longer drift apart.
- Any future cross-template prose reuse in awf's own catalog has a sanctioned mechanism instead
  of copy-paste, without opening a project-facing config surface.

Harder / accepted trade-offs:
- A second marker concept (`awf:include`) joins the `awf:` vocabulary — a general reader now
  learns overlay, raw-part, and include semantics. Mitigated by the fail-loud guards and by the
  directive being author-facing only (it never appears in rendered output).
- The per-reviewer differences move from inline prose into catalog `data:` defaults, which is
  slightly less legible at a glance than inline text; covered by the byte-identical golden
  regression test.
- Existing adopters see the three reviewer agents flagged stale after upgrading: restructuring
  their templates changes both the lock `TemplateHash` (item 2's expanded-source hash) and the
  `configHash` (the new per-artifact catalog `data:` keys fold into `artifactConfigHash` per
  ADR-0045), even though rendered output is byte-identical. `awf sync` resolves it — the normal
  upgrade path (ADR-0039). Artifacts carrying no `awf:include` are untouched: expansion is a
  no-op for them, so their `TemplateHash` is unchanged.
- Nested includes and project-authored partials are deferred; a future consumer needing either
  reopens the decision.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- All five `inv:` slugs above land a matching `// invariant: <slug>` source marker under
  `./internal/...`, each backed by a test, in the same range that flips this ADR to
  `Implemented`; otherwise the ADR-0008 backed-invariants gate fails. No existing Implemented
  invariant is retired (`retires_invariants: []`).
- The `awf:include` directive materially shifts the `rendering` domain narrative
  (`docs/domains/rendering.md`) and the render-pipeline description in `docs/architecture.md`;
  both are refreshed in the implementing range.
- A one-line clarifying nod in the rendering narrative that awf-owned partials are templated
  content spliced pre-parse (reinforcing, not contradicting, ADR-0034's raw-convention
  boundary).
- No `docs/decisions/README.md` row is owed (the index is the generated `ACTIVE.md`; the README
  is a how-to, ADR-0005), and no AGENTS.md invariant entry is owed: `awf:include` is an
  author-only, awf-internal template mechanism an adopter never writes, not a project-facing
  rule.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Go-native `{{ template "spine" . }}` associated templates | Introduces a second composition model with different semantics — included content is not `awf:section`-overlayable and gets no `awf:edit` provenance — and forces `Execute` to parse into a shared set. The source-splice reuses 100% of the existing overlay/provenance/scan machinery. |
| One parameterized `reviewer.md.tmpl` rendered 3× | The five universal lenses differ completely per reviewer, so a single template fills with large `{{ if eq .kind }}` branches and changes how artifacts are instantiated — trades duplication for conditional sprawl. |
| Allow project-authored partials now | Adds a new `.awf/` input that must fold into `artifactConfigHash` and the drift model, for no current consumer — premature (ADR-0045's no-abstraction-without-a-call-site principle). Awf-owned only keeps the drift model unchanged. |
| Splice but hash the raw pre-expansion source | Leaves `awf check` green while a partial edit changes `awf sync` output — a drift-oracle blind spot. Item 2 hashes the expanded source instead. |
| Leave the spine duplicated (status quo) | The three files keep drifting apart on every discipline edit; the whole point of the effort. |
