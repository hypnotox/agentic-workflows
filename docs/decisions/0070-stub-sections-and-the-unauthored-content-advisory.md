---
status: Implemented
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [rendering, tooling, advisory]
related: [1, 11, 34, 45, 46, 54, 57, 58, 68]
domains: [rendering, tooling]
---
# ADR-0070: Stub sections and the unauthored-content advisory

## Context

A 2026-07-07 docs-currency sweep found `docs/development.md` had rendered as pure
authoring prompts ("_List the steps to a working local checkout…_") since it was
scaffolded — weeks of an enabled doc shipping nothing but placeholder text, with no
check surfacing it. The session retrospective flagged the gap: a rendered artifact whose
sections are all still must-replace prompts should not exist silently.

The root cause is that template section defaults have two natures the model does not
distinguish. Some defaults are coherent generic prose, valid to ship as-is — the
publication-safety contract (ADR-0001, ADR-0045) guarantees every default renders
sensibly. Others are deliberate authoring prompts that exist only to be replaced by a
convention part. ADR-0011 gave docs static default content and a per-doc section
taxonomy; nothing since has recorded which defaults are content and which are prompts.
The same blindspot covers `awf new skill|agent` (ADR-0068), which scaffolds a starter
`content` part that is itself an authoring prompt — and because a part exists, any
default-detection scheme that stops at "no part present" misses exactly the motivating
case.

Two adjacent facts shape the design:

- **A latent leak.** `render.ParseSections`' regex matches only the exact
  `<!-- awf:section <name> -->` form; a malformed marker (for example one carrying an
  unknown attribute) silently fails to match and leaks verbatim — markers, `awf:end`,
  and all — into rendered output. The only nets today are test-side leak assertions in
  this repo; adopter renders have no production guard. Extending the marker grammar
  with attributes makes this hole live, so it must close in the same change.
- **"Placeholder" is taken.** ADR-0057/0058 use it for the `{{=awf:…}}` sandbox
  tokens, `working-with-awf` has a section literally named `placeholders`, and the
  publication-safety prose uses it for unresolved-value tokens. The new concept needs
  its own word.

A grounding pass also established: advisory notes are printed today by `awf check` and
`awf init` only (`awf sync` prints just backup lines and "done"); domain docs render
outside `RenderAll` through their own generation path, and their `current-state`
default is an authoring prompt; and all project-local artifacts share the two base
template ids, so any per-template-id keying collapses distinct local artifacts.

## Decision

1. **`stub` section-marker attribute.** The section-marker grammar gains one optional
   attribute: `<!-- awf:section <name> stub -->`. `render.ParseSections` parses it into
   a `Stub` flag on the segment. Markers exist only in awf-owned templates, so the
   attribute lives entirely on the templated side of the ADR-0034 raw-vs-templated
   boundary; convention parts are unaffected.

2. **`<!-- awf:stub -->` part marker.** A convention part whose body contains a line
   that is exactly `<!-- awf:stub -->` (whole-line match — prose quoting the marker
   inline never counts) is unauthored. The part still renders byte-for-byte
   verbatim per ADR-0034 — the marker is detected, never stripped — and an HTML comment
   is invisible in displayed markdown. Deleting the marker is how an author declares
   the part real. `awf new skill|agent` scaffolds its starter `content` part with this
   marker (plus a line telling the author to delete it), closing the ADR-0068 starter
   gap; any hand-authored part may carry it deliberately to mark work-in-progress.

3. **Stub provenance pointer.** A stub-flagged section rendering its template default
   emits the edit pointer `<!-- awf:edit <name> — stub; replace by creating <path> -->`
   in place of `— default; create <path> to override`, so the rendered file itself
   distinguishes a must-replace default from a valid one. A part-backed section keeps
   its `— from <path>` pointer; part-level stubness is reported by the advisory, not
   the pointer.

4. **The unauthored-content advisory (`StubNotes`).** A non-failing advisory — the
   sibling of the ADR-0045 unset-var notes — printed by `awf check` and `awf init`
   (`awf sync`'s output contract is unchanged). One line per rendered artifact, keyed
   by output path (never template id: local artifacts share the base template ids, and
   all domain docs share one template), listing the artifact's stub sections still at
   default and its stub-marked parts. The domain-doc generation path participates
   explicitly, since it renders outside `RenderAll`. Both note sets are computed from
   one render pass.

5. **Residual-marker guard.** After section assembly and before template execution, any
   remaining marker-shaped comment token in the assembled skeleton — `<!--` (optional
   whitespace) followed by `awf:section` or `awf:end`, the ADR-0057 near-miss shape —
   is a hard render error. The pattern is comment-anchored, never a bare-identifier
   scan: a section default may legally quote the bare token in prose (doc-standard's
   `structure` default does today), and un-overridden defaults survive into the
   skeleton. This closes the malformed-marker leak in production, not just in this
   repo's tests. Scanning the assembled skeleton — where part bodies are NUL sentinels
   and data is uninterpolated — keeps part and data prose that quotes even the full
   comment form (the agent guide's invariants bullet, the rendering domain narrative)
   out of scope.

6. **Template sweep.** Every section default in the shipped templates is classified:
   authoring prompts get the `stub` attribute; coherent generic prose stays plain. The
   base skill/agent `content` defaults (ADR-0068) are classified stub *and* remain
   publication-safe — stub means "replace me", not "unsafe to render"; ADR-0068's
   degradation stance is unchanged.

7. **Naming.** The attribute, marker, and advisory use "stub" exclusively;
   "placeholder" stays reserved for the ADR-0057/0058 sandbox tokens.

## Invariants

- `invariant: stub-advisory-nonfailing` — unreplaced stub sections or stub-marked parts never
  by themselves cause `awf check` (or any gated command) to exit non-zero.
- `invariant: no-residual-section-marker` — an assembled skeleton still containing a
  marker-shaped `<!-- awf:section` / `<!-- awf:end` comment token is a hard render
  error in production code, not only a test-side assertion; a bare backtick-quoted
  identifier in default prose does not trip it.
- `invariant: stub-part-verbatim` — a stub-marked convention part renders byte-for-byte
  verbatim, marker included; detection never mutates part bodies.
- `invariant: stub-notes-path-keyed` — the advisory reports per output path, so artifacts
  sharing a template id (local artifacts, domain docs) each report independently.
- Textual: every shipped template section default is deliberately classified — an
  authoring prompt carries `stub`; prose valid to ship does not. New templates classify
  at authoring time.

## Consequences

- A fresh `awf init` (and the first `awf check` after enabling a doc) prints a block of
  stub notes — one line per artifact — until the project authors its parts. This is the
  intended orientation, mirroring how init already prints unset-var notes.
- Every swept template's hash changes, and stub sections' pointer text changes, so
  adopters see a large `stale` set on upgrade and land one big `awf sync` commit. This
  is ordinary template-change drift; the changelog entry says so.
- `awf new` artifacts now self-flag until actually authored, closing the loop that
  motivated the feature.
- With more than one adapter target enabled, an unauthored skill or agent reports once
  per target output path. This duplication is accepted: path keying is what keeps local
  artifacts and domain docs distinct, and each line names exactly the file that would
  change when the part is authored.
- Docs travel with the implementation, same commit: `docs/working-with-awf.md`'s
  override prose gains the `stub` attribute, the `<!-- awf:stub -->` part marker, and
  the new pointer text; the rendering domain `current-state` part is updated; the
  agent-guide invariants bullets and the regenerated `docs/decisions/ACTIVE.md`
  (`./x sync`) land with the status flip to Implemented.
- The malformed-marker leak is closed for adopters, not just for this repo's test
  suite; the cost is that a genuinely malformed template now fails at render instead of
  producing corrupt output.
- Authors who write real content but forget to delete `<!-- awf:stub -->` keep getting
  the advisory; the scaffolded marker line carries its own "delete this line" text to
  make that unlikely, and the advisory names the part path.
- Explicitly ruled out: failing `awf check` on stub content (hostile to incremental
  adoption — enabling a doc would break the gate until authored), and catalog-side
  per-section stub metadata (duplicates knowledge away from the template body it
  describes).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Hard `awf check` failure on stub content | `awf add doc <name>` would break the gate until the doc is authored; punishes incremental adoption. User chose advisory. |
| Attribute named `placeholder` / `must-replace` | "placeholder" already has three meanings in awf (ADR-0057/0058 tokens, the `placeholders` section, publication-safety prose); `must-replace` is awkward as an attribute and identifier. |
| Body directive inside the template default | Two things to keep in sync per section; a strip failure leaks the directive into output; less discoverable than the section declaration. |
| Catalog per-section metadata (`Sections` becomes a struct list) | Duplicates template knowledge away from the body, churns every spec literal and the section-parity guard for no added power. |
| Exact-text match to detect unedited `awf new` starter parts | Brittle (any one-character edit defeats it) and invisible in the file; the explicit `<!-- awf:stub -->` marker is self-documenting and deliberate. |
| Residual-marker scan over final rendered output | False-positives on legal prose that quotes the markers (agent guide, rendering domain narrative); the assembled skeleton is the correct scope. |
