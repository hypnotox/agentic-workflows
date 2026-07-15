---
status: Implemented
date: 2026-07-07
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [convention-parts, placeholder-registry]
related: [1, 15, 34, 45, 57, 58, 70]
domains: [rendering]
---
# ADR-0072: Convention parts re-inject their section default via a placeholder

## Context

A convention part overrides a rendered section by **fully replacing** its template
default (ADR-0015). ADR-0034 made parts raw input: a part body is never run through
`text/template`, so literal `{{ }}` in adopter prose stays inert. ADR-0057/0058 then
added a sandboxed `{{=awf:KEY}}` placeholder set so a raw part can still splice
awf-derived *config* values (commit scopes, prefix, gate commands) without reopening
the brace trap; every current key expands to a static, section-agnostic string frozen
once per render.

The gap: an adopter who wants to **extend** a shipped default (add a house rule after
awf's generic guidance, or a preamble before it) has no way to reference that default.
The only recourse is to copy the entire default into the part and maintain a divergent
fork. When awf later revises the default, the copy silently goes stale, and drift-check
cannot catch it: the part is authoritative raw prose, so a `check` sees no template
mismatch to flag. Adopters are pushed toward either forking (and rotting) or forgoing
awf's default entirely.

The mechanism to close this already exists in shape (the placeholder registry) but no
key expands to *this section's own default*. During design we confirmed why such a key is
not a trivial registry entry: the **rendered** default is unavailable at placeholder-
substitution time. `substitutePlaceholders` runs inside `planSections`, before the default
is parsed (`ParseSections`) and rendered (`Execute`) later in the same `renderTarget`
pipeline; and a section's default source is discarded outright the moment a part
exists (`Assemble` writes the part sentinel instead of `s.Text`). The default is also
not splice-able raw: non-stub defaults carry live actions like `{{ .layout.workingWithAwf }}`
and `{{ with .vars.gateCmd }}`, so splicing source into a verbatim-restored part would
publish dead braces and break publication-safety (ADR-0045). Any solution must render the
default, and can only do so in the render layer.

## Decision

1. **Add one placeholder key, `sectionDefault`, to the sandboxed registry (ADR-0057).**
   In a convention part it denotes the **rendered** default body of the section that part
   overrides. Text placed before the token becomes a preamble; text after it, an appendix;
   surrounding text on both sides, a wrap. This is **not** a new override mode: a part still
   replaces its section body; `sectionDefault` merely lets the replacement carry its
   default forward instead of discarding it. The marker may appear more than once in a
   part; each occurrence re-injects the rendered default, splitting the part into N+1
   verbatim fragments interleaved with N rendered defaults (no special-casing: the
   split-marker mechanism of Decision 2 generalises directly).

2. **Static split-marker sentinel; the render layer owns positioning.** The registry
   *value* for `sectionDefault` is a fixed, brace-free NUL split-marker sentinel, never the
   default content. The project-layer `substitutePlaceholders` pass substitutes the token
   through its existing, unmodified code path, staying ignorant of templating and
   positioning (it maps a key to an opaque token, as it does for every key). The render
   layer (`Assemble`) recognises that sentinel inside a part body, splits the part into the
   fragments around it, and emits `<pre-fragment sentinel> + <default template source> +
   <post-fragment sentinel>` into the pre-`Execute` skeleton. `Execute` then templates the
   default in place while the raw pre/post fragments are restored verbatim around it. Output
   = pre + rendered-default + post.

3. **ADR-0034's raw-part contract holds literally.** Only the awf-owned default source is
   passed through `text/template`; the adopter's part prose is still restored verbatim and
   never parsed. The boundary moves not at all: raw parts stay raw.

4. **A stub default cannot be re-injected.** A `sectionDefault` reference in a part
   overriding a `stub`-attributed section (ADR-0070) is a **hard render error**. A stub
   default is an authoring prompt, not shippable prose; re-injecting it is always a mistake,
   and the section must stay in must-author state.

5. **No provenance change.** A part using `sectionDefault` is still a part; its rendered
   edit-pointer stays the `: from <path>` form. No consumer distinguishes an extending part
   from a replacing one, and none is introduced.

6. **Docs reframe, not two modes.** The `working-with-awf` override section and the AGENTS
   override bullet (which today present parts as replace-only) are corrected to: a part
   replaces the section, and `sectionDefault` lets a replacement re-inject its default
   instead of discarding it. The placeholder key table gains `sectionDefault`.

## Invariants

- `invariant: section-default-splice`: When a convention part body contains the `sectionDefault`
  split-marker sentinel, `Assemble` splits the part at that marker and emits the overridden
  section's default template source between the two verbatim part fragments, so `Execute`
  renders the default in place and the part's surrounding prose is restored verbatim.
- `invariant: section-default-stub-error`: A `sectionDefault` reference in a part overriding a
  section whose default carries the `stub` marker is a hard render error, never a silent
  splice of the authoring prompt.

## Consequences

- **Closes the stale-fork failure mode.** An adopter extends a default without copying it;
  each `sync` re-renders the current default in place, so an awf revision flows through
  automatically. Drift-check reflags the artifact on a default edit for free, along two
  paths that already exist. A byte edit to a section default changes `expanded` (the
  default source sits in the template body regardless of any override), and `TemplateHash`
  hashes `expanded`, so a default-content edit already reflags today. Re-injection adds the
  dynamic half: the default source now physically lands in the `assembled` string, over which
  `artifactConfigHash` runs its drift scanners (`ReferencedVars`, `ReferencesScopes`, ...), so a
  var or scope reference living *inside* the default folds into `ConfigHash` and reflags the
  artifact when it changes. **No new confighash plumbing** is required, unlike the explicit
  scope-placeholder path.
- **Small, contained render-layer cost.** `Assemble` (or a `StubSections`-style sibling
  validator) gains an `error` return for the stub hard-error, a minor signature change
  rippling to its one caller. Stub detection scans the part body for the render-layer
  sentinel, not the original `{{=awf:sectionDefault}}` token, which `planSections` has
  already substituted away.
- **The ADR-0057 layer boundary is preserved.** The project layer still owns only `awf:`
  key→token mapping; the render layer still owns all `text/template` execution and now also
  owns splice positioning. The registry gains a new *key class*: `sectionDefault`'s value is
  a splice-point marker, not a config-value string, a deliberate widening of "what a
  placeholder may be" that ADR-0057 did not contemplate.
- **Empty-default re-injection is a harmless no-op**, not an error: re-injecting an empty
  non-stub default yields `pre + "" + post`. This is a deliberate simplification: the
  static-sentinel mechanism gives the project layer no view of default emptiness, and
  erroring would buy a special case with no real-world benefit (the stub hard-error already
  catches the meaningful mistake). This is also why `sectionDefault` does not contradict
  ADR-0057's "empty-valued key is unavailable → hard error" rule: the registry *value* is
  the always-non-empty split-marker sentinel, so the key is always available; and an empty
  splice still yields coherent prose (the part's own text stands, no `<no value>`),
  preserving publication-safety (ADR-0045).
- **Trade-off:** a part can now contain a token whose expansion depends on section context
  the author cannot see inline (the upstream default may change). That is precisely the
  intent (tracking the upstream default is the feature) and the rendered output shows the
  resolved result, so the effect is inspectable.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| **Strategy A: render the default in the project layer.** `planSections` runs `text/template` over the section default and substitutes the rendered string. | Puts template execution in the project layer and forces a pipeline reorder (`planSections` precedes `ParseSections`), eroding the ADR-0057 boundary that keeps `internal/render` the sole owner of templating. |
| **Append/prepend via filename suffix or marker** (fixed positions, handled entirely in `Assemble`, no in-body token). | Only two positions (no wrap or interleave) and it invents a second override concept ("additive part") instead of reusing the existing placeholder seam. |
| **A Mustache-style `{{ section.default }}` template variable.** | Reopens the ADR-0034 brace trap (parts would need templating) and duplicates the sandbox the project already has. |
| **Nested bespoke sentinels** (the initial framing of Decision 2). | A normal registry entry whose value is a static split-marker sentinel reaches the same effect with zero special-casing in the substitution pass, strictly simpler. |
| **Distinct provenance pointer** (`: extends default; from <path>`). | A part is a part; the distinction has no consumer and would imply a "mode" the model does not actually have. |
