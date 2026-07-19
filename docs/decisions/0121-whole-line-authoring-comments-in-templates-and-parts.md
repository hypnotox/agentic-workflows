---
status: Implemented
date: 2026-07-16
tags: [authoring-comments, convention-parts, invariant-backing, marker-scoping]
related: [8, 34, 52, 57, 70, 72, 82, 83, 100, 105, 120]
domains: [rendering, invariants, config]
---
# ADR-0121: Whole-Line Authoring Comments in Templates and Parts

## Context

There is no strip-at-render comment in the rendering pipeline: everything written in an
embedded template body or a convention part renders into the adopter-facing artifact.
Some comments rely on that deliberately - the catalog ships pass-through guidance like
`<!-- customise: ... -->` (refactor-coupling-audit skill) and `<!-- Authoring: ... -->`
(agents-doc template) that is *meant* to be read in rendered output. But the missing
authoring-side comment blocks a concrete, valuable use: tagging template and part content
with `touches-invariant: <slug>` markers (ADR-0105) so `awf context` surfaces
invariant-related prose. Written today, such a tag would leak into every rendered skill,
doc, and - for embedded templates - every adopter's tree.

The invariant scanner is nearly ready for these files already: it matches a configured
literal line-prefix marker per source glob (`internal/invariants/invariants.go`,
ADR-0008), so `invariants.sources` could name markdown files with an HTML-comment marker
today. Two gaps remain. First, the leak above. Second, a block-comment wart: the
`touches-invariant:` note grammar captures everything after the slug, so a closing
`-->` (or `*/` in C-family languages) pollutes the note - and a note-less
`<!-- touches-invariant: x -->` yields the phantom note `-->`, wrongly suppressing the
bare-touches advisory (ADR-0105 item 5).

Constraints in force:

- ADR-0034 established that convention parts are raw input, with the backed invariant
  `parts-raw`: never templated, "appears verbatim in rendered output". ADR-0057
  (sandboxed placeholders) and ADR-0072 (sectionDefault splice) later carved substitution
  channels into part bytes without retiring the slug, reading it narrowly as
  "never passed through text/template". Removing author-written lines is a different
  class of mutation: it makes the verbatim clause false in a new way.
- ADR-0083 recorded, for accidental section-marker residue, that "rejecting or mutating
  part content is off the table", and chose a whole-line-vs-inline advisory instead. That
  stance targeted *accidental* residue with legitimate inline quoters; it does not bind a
  deliberate, namespaced, opt-in directive - but the distinction must be explicit, and
  this ADR reuses 0083's two load-bearing discriminators: whole-line-vs-inline, and
  scanning raw bytes so substituted values can never create or mask a match.
- ADR-0100's in-place sections read their bodies back from rendered *output*; an
  authoring construct of config-tree and template sources must never mutate them.
- A grounding sweep confirmed the single template-source choke point (`renderTarget`:
  read, `ExpandIncludes`, then `ParseSections`) covers every render unit including the
  shell ones, which already carry HTML-form section markers in source (the runner
  template); and that `refs.WithoutFences` drops lines rather than preserving them, so a
  mutating strip needs its own keep-lines fence tracker.

The obvious simpler standard - treat every whole-line HTML comment as authoring-side and
strip it - was considered and rejected: it silently deletes content existing adopter
parts were written to render (a trust-eroding upgrade surprise with no error to find),
removes the pass-through-guidance capability the catalog itself uses, and the "comments
never render" story would be false anyway (inline comments cannot be stripped safely,
and the renderer itself emits `awf:edit` pointers and the generated banner). A
namespaced directive matches the existing `awf:` grammar family and keeps the ADR-0034
carve-out minimal.

## Decision

1. **The `awf:comment` authoring directive.** A line whose trimmed form begins with the
   exact literal `<!-- awf:comment` followed by whitespace, an immediate `-->`, or the
   end of the line (the token boundary), and closes with `-->` at the end of the same
   line, is an authoring comment. It is removed
   at source ingestion, together with its trailing newline, and never reaches rendered
   output. The token boundary is load-bearing: `<!-- awf:commentary -->` is not the
   directive. The literal is exact - a whitespace variant such as `<!--  awf:comment` is
   not the directive and passes through (visibly, by design: a variant that stripped
   here but failed the literal marker match in the invariant scanner would lose tags
   silently). Mid-line occurrences are inert prose, per the ADR-0083 whole-line-vs-inline
   distinction. The directive is single-line; there is no block form.

2. **Two ingestion seams, and only those two.** The strip runs (a) over template source after
   `ExpandIncludes` and before `ParseSections`, covering every render unit (markdown and shell
   alike, plus include partials and non-section skeleton text), and (b) over each convention
   part's raw on-disk bytes in `planSections`, *before* placeholder substitution (ADR-0083
   Decision 4 (`cites: ADR-0083#4`) rationale: a substituted value must never create or mask a
   whole-line match; and an unknown `{{=awf:key}}` demonstrated inside a comment must not
   hard-error). Downstream part scanners - the stub marker, the part-marker advisory, placeholder
   var refs, and the confighash placeholder-folding detectors - read the stripped body, so a
   placeholder or marker mentioned only inside an authoring comment does not count as present.
   ADR-0083 Decision 4's raw-bytes contract for the part-marker advisory is preserved in effect,
   not overridden: the strip runs pre-substitution and removes only whole lines opening with the
   directive literal, which the `awf:section`/`awf:end` prefix matcher can never match, so it
   provably cannot add or remove a marker-shaped line; no override token is owed. `TemplateHash`
   and the part-byte `ConfigHash` inputs remain the *unstripped* bytes: a comment-only edit
   reflags the artifact stale and the next sync settles it with byte-identical output. In-place
   read-back bodies (ADR-0100) are never stripped: they come from rendered output, a different
   channel, and a directive-shaped line an adopter writes there survives re-render verbatim.

3. **Fences preserve; malformed openers fail.** Inside a fenced code block, directive
   lines (and malformed openers) are preserved verbatim, so parts and templates can
   demonstrate the syntax. Outside a fence, a whole line that opens per Decision 1's
   token boundary (the literal followed by whitespace, `-->`, or end-of-line) but is
   not the directive - it does not end with `-->`, whether because the close is missing
   (including the bare `<!-- awf:comment` line) or because text trails it
   (`<!-- awf:comment x --> extra`) - is a hard render error naming the source
   (template id or part path): the directive is new and namespaced, no legacy corpus
   exists in this tree, a malformed opener has no legitimate use, and passing it
   through would leak. A prefix-sharing token (`<!-- awf:commentary`), closed or
   unclosed, fails the boundary and is inert prose, never an error.
   This is the first rejection of part content; it does not disturb ADR-0083's advisory
   (whose matcher names only `awf:section`/`awf:end` and cannot fire on this directive),
   and the accidental-residue stance recorded there stands for everything that is not
   this deliberate opt-in grammar.

4. **`parts-raw` is retired and succeeded.** This item carries
   `refines: ADR-0034#1` and `supersedes-invariant: ADR-0034#parts-raw`: ADR-0034
   Decision 1's "verbatim content" contract now reads "verbatim except whole-line
   `awf:comment` lines". Unlike ADR-0057/0072's substitutions - which splice in content
   the part author requested at the marked spot - the strip *removes* author-written
   lines, so the retirement is owed rather than another narrow re-reading. The successor
   invariant `parts-raw-except-authoring-comments` is declared below; ADR-0034's
   `related:` back-pointer (the ADR-0120 item-4 obligation) landed with this proposal,
   and the implementing effort renames the existing `parts-raw` proof and touches
   markers to the successor slug in the flip commit.

5. **`invariants.sources` entries gain an optional `close` token.** A source entry may
   declare `close: "-->"` (any literal); when a marker line matches, one trailing close
   token plus surrounding whitespace is stripped from the line end before the
   `invariant:` / `touches-invariant:` grammar is applied, in both scan paths (the
   backing scan and the context query scan). An absent or empty `close` means no
   stripping - the scanner stays a dumb literal matcher and existing configs are
   untouched. This fixes the block-comment note wart for every `/* */`-family language,
   not just markdown. Following the `testGlobs` and `runner` precedent, an additive
   optional key needs no schema-generation bump and no `awf upgrade` migration; the
   config-spec walker and the generated config reference document it.

6. **Wiring is a documented recipe; awf dogfoods it.** No scaffold seeding: the docs
   show the `invariants.sources` entry for tagging parts and templates -
   `marker: '<!-- awf:comment'` with `close: '-->'` over parts-scoped globs - so only
   stripped comments can carry tags; a tag in a pass-through comment (which would leak)
   is never scanned. awf's own config adds the entry over `.awf/**/parts/**/*.md` and
   `templates/**` (parts-scoped deliberately: `.awf/memory/` is session state and must
   not back or touch anything), and the implementing effort retroactively tags existing
   parts and templates with `touches-invariant` markers where content narrates or
   depends on a declared invariant, each with a non-empty note. Documented limitations:
   fenced demo lines that carry a real tag grammar are still seen by the fence-unaware
   scanner (break the token in demos; the dangling-marker advisory nets accidents);
   comment text inside include partials must avoid the `awf:include`/`awf:section`/
   `awf:end` substrings (ADR-0052's partial guard); comment text in embedded templates
   must stay slug-only and identity-free (the ADR-0082 residue scan) and plain-ASCII
   (ADR-0115/0119) everywhere.

## Invariants

- `invariant: authoring-comment-stripped`, a whole-line `<!-- awf:comment ... -->` in a
  template source (including an include partial) or a convention part never appears in
  rendered output, across every render unit.
- `invariant: authoring-comment-whole-line-only`, a mid-line `awf:comment` occurrence
  and a fenced whole-line directive both render verbatim; the strip never rewrites
  either, and a non-directive token sharing the prefix (`<!-- awf:commentary`, closed
  or unclosed) is untouched and never an error.
- `invariant: authoring-comment-malformed-fails`, a whole line outside a fence that
  opens per the Decision 1 token boundary but does not end with `-->` (missing close,
  or text trailing the close) fails the render with an error naming the source; inside
  a fence it is preserved.
- `invariant: authoring-comment-inplace-inert`, an in-place section body read back from
  rendered output is never subject to the strip: a directive-shaped line inside an
  in-place region survives re-render byte-for-byte.
- `invariant: parts-raw-except-authoring-comments`, a convention part body is never
  passed through `text/template` and appears verbatim in rendered output, apart from
  the sandboxed placeholder and sectionDefault substitution channels (ADR-0057,
  ADR-0072) and the removal of whole-line `awf:comment` lines at ingestion (successor
  to `parts-raw`, ADR-0034).
- `invariant: invariant-marker-close-token`, when an `invariants.sources` entry declares
  a `close` token, one trailing close token is stripped from a matched marker line
  before slug and touches-note parsing in both the backing scan and the context query
  scan; a note-less touches marker then correctly fires the bare-touches advisory.

## Consequences

- Parts and templates become taggable with `touches-invariant` markers that never ship:
  the motivating capability, available to awf and adopters alike through one documented
  recipe. General authoring notes ride the same directive.
- Existing trees are untouched: the directive is opt-in per line, pass-through comments
  keep rendering, and a close-less `invariants.sources` entry behaves exactly as before.
  No schema bump, no migration.
- Accepted leak modes, all visible rather than silent: a whitespace-variant opener and a
  mid-line occurrence render into output as ordinary comment text. The rendered docs
  state the whole-line, exact-literal rule loudly.
- The "no legacy corpus" claim holds for this tree but is an assumption over adopter
  trees: an adopter part already carrying a whole-line `<!-- awf:comment` opener written
  for pass-through would be stripped on the first post-upgrade sync - or, if malformed,
  hard-error naming the part path. The namespaced literal makes the collision improbable
  where strip-all made it certain, and the malformed case at least fails loudly; the
  residual risk is accepted.
- When this ADR goes live, the ADR-0120 same-anchor advisory notes that `ADR-0034#1` is
  claimed by both ADR-0057 and this ADR. That is the designed signal for a legitimate
  clause-level split (ADR-0120 item 5): ADR-0057 carved the substitution channels into
  item 1's contract, this ADR carves whole-line removal. The standing note is accepted,
  not a defect.
- A part whose only content is directive lines strips to an empty body, which per
  ADR-0034 Decision 4 renders the section empty rather than dropping it - coherent, but
  silent; the docs note it. Newline consumption is pinned (the line and its trailing
  newline) so mid-part comments leave no blank-line residue.
- Comment-only edits reflag artifacts stale and self-settle on the next sync (hashes see
  unstripped bytes). This keeps the drift model simple at the cost of an occasional
  no-op-looking rewrite.
- The dogfood config entry re-renders the guide's invariant-marker table with the
  `<!-- awf:comment` literal in backticked prose - inline, hence neither stripped nor
  scanned; cosmetic only.
- The strip needs its own keep-lines fence tracker (`refs.WithoutFences` drops lines and
  fences, so it cannot be reused for mutation). Fence state is computed over source; a
  fence opener emitted conditionally by template logic can diverge from rendered fence
  state - accepted as a theoretical edge with no current instance.
- The retirement opens the usual ADR-0120 window: between nothing (this ADR flips
  straight to Implemented with the code) and the flip commit, `parts-raw` markers are
  renamed to the successor slug so no dangling-marker advisories survive.
- Downstream doc obligations for the implementing effort: `docs/working-with-awf.md`
  (directive syntax, recipe, limitations), the generated config reference (`close` key),
  `docs/architecture.md` render-flow note, the rendering, invariants, and config domain
  current-state narratives (this ADR carries all three domains, so ADR-0033's
  ADR-to-domain-index co-change applies to `docs/domains/rendering.md`,
  `docs/domains/invariants.md`, and `docs/domains/config.md`), the glossary (authoring
  comment), a changelog entry for the adopter-facing directive and `close:` key
  (ADR-0073), and the status-flip commit regenerating `docs/decisions/ACTIVE.md` and the
  three domain indexes via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Strip every whole-line HTML comment (no directive) | Silently deletes content existing adopter parts render today; kills the deliberate pass-through guidance the catalog ships; inline and renderer-emitted comments falsify the "comments never render" story anyway. |
| Strip at the final-output seam (one pass over the rendered string) | Mutates in-place-editable regions the user owns (ADR-0100), and makes fence tracking global across assembled sources instead of local per body. |
| Go template comment actions (`{{/* ... */}}`) for the template seam | Already never render and could carry scanner tags via a `marker: '{{/*'` entry with zero pipeline code - but parts are never templated (ADR-0034), so the parts seam cannot use them: two grammars for one concept and no uniform recipe. The strip buys one grammar across both seams. |
| Sidecar metadata (`touches:` list) instead of comments | Loses line-level co-location, gives no general authoring comments, and does not work inside embedded templates. |
| Scanner-only fix, no strip | Tags leak into every rendered artifact and adopter tree; the motivating use stays blocked. |
| Hardcode `-->` handling for HTML-comment markers | Leaves the `*/`-family note wart and breaks the marker-is-an-opaque-literal principle; the generic `close` token is the same size. |
| Fence-aware invariant scanner | Pushes markdown-specific fence logic into the deliberately language-agnostic scanner for a demo-only edge the dangling-marker advisory already nets. |
| Keep `parts-raw` under a narrower reading (0057/0072 precedent) | Those substitutions splice in author-requested content; removing author-written lines makes the invariant's verbatim clause false in kind, not degree - an honest retirement with a successor slug is owed. |
