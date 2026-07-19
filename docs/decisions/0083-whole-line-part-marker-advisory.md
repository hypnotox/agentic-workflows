---
status: Implemented
date: 2026-07-09
tags: [convention-parts, marker-scoping]
related: [34, 70, 77, 82, 121]
domains: [rendering]
---
# ADR-0083: Whole-line part-marker advisory

## Context

An upgrade rehearsal against a real adopter tree (v0.5.0 → 0.12.0) probed the
residual-marker guard's edges: a convention part containing the line
`<!-- awf:section bogus -->` syncs without error, the marker lands verbatim in the
rendered artifact, and `awf check` reports clean. An adopter who typos a section marker
inside a part (or believes a part can *declare* a section) gets no signal that the
marker is inert.

This is not a guard bug. ADR-0070 Decision 5 deliberately scopes
`render.CheckResidualMarkers` to the assembled skeleton, where part bodies are NUL
sentinels and data is uninterpolated, precisely because legitimate quoters of the full
comment form exist: this repo's rendering-domain narrative
(`.awf/domains/parts/rendering/current-state.md`) quotes `<!-- awf:section -->` inline
in prose, and parts render byte-for-byte verbatim per ADR-0034; rejecting or mutating
part content is off the table. Any hard-error extension of the guard into parts would
break the dogfood on day one.

The distinguishing observation: every legitimate quote found is *inline* (the marker
form embedded mid-sentence or backticked in prose) while a confused override attempt
is a *whole line* that is nothing but the marker. A whole-line grep
(`^\s*<!--\s*awf:(section|end)\b`) over this repo's entire `.awf/` tree matches
nothing today. The same whole-line-versus-inline distinction already separates the
legal `<!-- awf:stub -->` part marker from prose that merely mentions it (ADR-0070
Decision 2), and ADR-0077's `domain-code-staleness` rule set the precedent for a
part-keyed advisory.

Terminology note: this ADR concerns *rendered-content* marker residue in adopter
parts, distinct from ADR-0082's *template-source* residue guard, which sweeps
awf-owned template sources for ADR citations and identity literals.

## Decision

1. **Detection rule.** A consumed convention part whose body contains a line that,
   after trimming surrounding whitespace and excluding fenced code blocks, *begins
   with* a marker-shaped comment opener (`<!--`, optional whitespace, `awf:section`
   or `awf:end` at a word boundary) is flagged. Prefix-anchoring on the trimmed
   line deliberately covers all three residue shapes (the exact closed marker, an
   unclosed opener missing `-->`, and a marker followed by trailing text): none has
   a legitimate quoter, since prose that quotes the form always precedes it on the
   line with a backtick, list bullet, or sentence text. This is exactly the matcher
   the Context's exploratory grep used, which found zero hits in this repo. The
   `<!-- awf:stub -->` marker (ADR-0070 Decision 2) is out of scope by construction:
   the pattern names only `awf:section`/`awf:end`. Inline quoting (the marker form
   appearing mid-line) never fires. Fence exclusion reuses the `refs.WithoutFences`
   precedent from the dead-skill-reference scan, so a part demonstrating marker
   syntax in a fenced example stays silent.

2. **Advisory, never a failure.** The flag is a non-failing note on the existing
   advisory channel (`Project.AdvisoryNotes`), printed by `awf check` and `awf init`
   only; `awf sync`'s advisory silence (ADR-0070 Decision 4) is preserved. The note
   states the fact, the consequence, and the remedy, e.g.:
   `part .awf/skills/parts/foo/bar.md contains a marker-shaped line: section markers
   have no effect inside convention parts; fence the example to silence this note`.
   The note is self-documenting: no separate doc carries the remedy.

3. **Part-keyed, deduplicated.** The note is keyed by the part's config-tree path,
   a deliberate deviation from the output-path keying of stub notes (ADR-0070
   Decision 4, `stub-notes-path-keyed`): the actionable file is the part itself, and
   one part may feed artifacts rendered once per enabled target. Because multi-target
   rendering consumes the same part repeatedly, notes are deduplicated by part path
   (the seen-map idiom of the unset-var notes). No line numbers: parts are small and
   the raw-body/rendered-body offset bookkeeping is not worth the precision.

4. **Scan the raw part bytes.** Detection runs over the part body as read from disk,
   before sandbox-placeholder substitution: substituted values can be multi-line and
   must never create or mask a whole-line match. The sibling stub detection at the
   same seam (`HasStubMarker`) scans the *substituted* body today; that divergence is
   deliberate and out of scope here; its semantics predate this rationale and no
   registry value carries a marker token. The domain-doc generation path
   participates explicitly, as it rebuilds its rendered-file records outside the main
   render loop.

5. **Skeleton guard unchanged.** `render.CheckResidualMarkers` keeps its ADR-0070
   Decision 5 scope exactly: assembled-skeleton hard error, parts and data out of
   scope. This ADR adds a sibling signal; it retires nothing.

## Invariants

- `invariant: part-marker-advisory`: a trimmed part line beginning with a marker-shaped
  `awf:section`/`awf:end` comment opener, outside fenced code, yields a
  part-path-keyed note from `awf check` and `awf init`, and never by itself causes
  any command to exit non-zero; inline prose quoting the marker form mid-line and a
  fenced whole-line example produce no note.

## Consequences

- A confused override attempt (the only realistic way a marker-shaped line enters a
  part) now gets a signal at `awf check`/`awf init` instead of silently rendering
  inert markup. The false-positive channels (inline prose, fenced demos, the stub
  marker) are all excluded by construction; this repo's tree produces zero notes.
- The advisory channel gains its second keying scheme (part path, after ADR-0077's
  part-keyed staleness warning).
- The implementing commit sharpens the guide's existing "Stub advisory,
  residual-marker guard" invariants bullet (its `.awf/agents-doc.yaml` entry) to
  state the skeleton scope explicitly (so the hard-error guard is no longer
  over-read as covering parts) and adds a new bullet for `part-marker-advisory`,
  both re-rendered via `./x sync` in that same commit. The rendering-domain
  current-state part (`.awf/domains/parts/rendering/current-state.md`) gains its
  sentence on the advisory in the same commit, with the regenerated
  `docs/domains/rendering.md`.
- A part that deliberately displays a whole-line marker outside a code fence will
  carry a permanent note; the note text itself names fencing as the remedy.
- A changelog entry travels with the implementation (adopter-facing `awf check`/`init`
  output change).
- The commit flipping this ADR to Implemented regenerates `docs/decisions/ACTIVE.md`
  via `./x sync`, staged in the same commit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Hard render error on whole-line markers in parts | Contradicts ADR-0034's parts-render-verbatim contract and fails legitimate documentation (fenced examples); an advisory matches the severity of an inert cosmetic leak. |
| Post-Execute scan of final rendered output | Fires on the legitimate inline quoters ADR-0070 D5 explicitly protects (the rendering-domain narrative): breaks the dogfood immediately. |
| Prose-only clarification of the invariant wording | Leaves the confused adopter with no signal; the wording fix is folded into this ADR's implementation anyway. |
| Line-numbered notes | Raw-versus-substituted body offsets add bookkeeping for negligible value; parts are short files. |
| Audit-rule warning (ADR-0077 channel) | `awf audit` scans a commit range, but a marker-shaped part line is standing tree state; it must surface on every `awf check`, not only when the part changed in range. |
