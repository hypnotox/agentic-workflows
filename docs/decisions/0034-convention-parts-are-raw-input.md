---
status: Implemented
date: 2026-06-29
supersedes: []
superseded_by: ""
tags: [convention-parts, section-assembly]
related: [1, 15, 57, 121]
domains: [rendering]
---
# ADR-0034: Convention Parts Are Raw Input

## Context

A project adopting awf ported existing hand-written docs into convention parts
(`.awf/<kind>/parts/<target>/<section>.md`). Several of those docs contained template-shaped
example snippets (Jinja, Go `text/template`, mustache) and any literal `{{` in a part broke
`awf sync` with an opaque error (`skill:NN: unexpected "}}" in define clause`). The error names
`skill` even for a doc or domain part, and there is no escape mechanism. This was the single
sharpest adoption trap reported, because template-shaped examples are common in real prose.

The breakage is structural, not cosmetic. ADR-0015 Decision item 4 established that a convention
part replaces a section body through a dedicated channel into `render.Assemble` (not the removed
`replaceWith` field), but it left *unstated* whether the part body is subsequently subject to the
template engine. In the current pipeline it is: `Assemble` (`internal/render/render.go:35-54`)
interleaves template-default text and part bodies into one string, and `Execute`
(`render.go:58-68`) runs `text/template` over the *whole* assembled string under
`missingkey=zero`. So a part body is templated today purely as an accident of the single-pass
design, never as a deliberate, documented capability.

That accidental capability is the root cause. As long as parts pass through the engine, a literal
`{{` is genuinely ambiguous (it could be an intended action), so no escaping scheme can be both
robust and discoverable. No existing convention part under `.awf/**/parts/**` uses `{{ }}`, so the
capability is unused in practice. The cleaner contract is the one the adopter intuited: awf owns
templating; the user supplies content. This decision makes that contract explicit.

This ADR refines, and does not replace, ADR-0015 Decision item 4 (the part-delivery channel) and
ADR-0001 (the rendering engine and its publication-safety contract). ADR-0015 stays live.

## Decision

1. **Convention parts are raw input.** A convention part body is verbatim content and is never
   parsed or executed by `text/template`. Templating (variable interpolation, conditionals,
   ranges, and the ADR-0001 publication-safety wrapping) is performed *only* over awf-owned
   embedded template defaults under `templates/`. A literal `{{`, `}}`, or any template-shaped text
   in a part renders byte-for-byte into the output. A part consequently *cannot* reference
   `{{ .Vars.x }}`; surfacing a variable inside a section that a project overrides is awf's
   responsibility in the template default, not the part author's.

2. **Placeholder-protected single pass.** `Assemble` keeps producing one assembled string for one
   `Execute` pass, but where a section is satisfied by a part it emits a unique, deterministic
   sentinel token in that slot instead of the raw body, and returns a `sentinel → raw body` map.
   `Execute` parses and executes the default-only skeleton (sentinels pass through the engine
   untouched), then substitutes raw part bodies back in. The sentinel is comment-shaped and
   brace-free so it is inert to both the template parser and the rendered markdown; its exact form
   is an implementation detail fixed by the plan.

3. **Real target name in render errors.** `Execute` is given the target's identity rather than the
   hardcoded literal `"skill"`, so a parse/execute failure in a template default names the actual
   target (its kind and name) instead of always reporting `skill`. With parts no longer templated,
   such failures can only originate in awf-owned defaults; the correct name aids awf's own
   template authoring.

4. **Empty is not drop.** An empty part file yields an empty section body (the section renders with
   no content), distinct from a sidecar `drop`, which removes the section and its `awf:edit`
   pointer entirely. Dropping remains the only way to remove a section.

## Invariants

- `invariant: parts-raw`: A convention part body is never passed through `text/template`; it appears
  verbatim in rendered output. Backed by a render-layer test asserting that a part containing
  literal `{{`/`}}` and template-shaped text renders byte-for-byte, and that a part is not
  variable-interpolated while the surrounding default sections are.
- Template control-flow actions (`if`/`with`/`range`/`define`) in an embedded template default
  never span a section-marker boundary, so a part-slot sentinel always resides within a single
  templated region and can never land inside an open control-flow block. (Textual contract; the
  plan decides whether to back it with a guard test.)

## Consequences

- The literal-brace trap is eliminated by construction rather than patched: there is nothing to
  escape because parts are never parsed. Adopters can paste template-shaped prose into parts freely.
- The part contract becomes simple and honest: awf templates, the user supplies content. This
  matches what every existing part already does (none use `{{ }}`), so the change is non-breaking
  and produces zero drift in awf's own rendered tree: the drift check is the proof.
- A part can no longer interpolate a variable. This is an accepted, deliberate loss: a section that
  must surface a project variable should remain a template default (optionally with the variable),
  not be offered as a part override. No current part needs this.
- The render pipeline gains a protect/restore step and a sentinel contract. Risk: a sentinel
  colliding with real content, or perturbing drift detection. Mitigated by a brace-free,
  comment-shaped, deterministic token. The drift risk is concrete and must be neutralised in
  implementation: `targetConfigHash` (`internal/project/confighash.go:30`) hashes each consumed
  part's bytes separately, but it also derives the referenced-var set via
  `render.ReferencedVars(assembled)` over the *full* assembled string, part bodies included, not
  the default skeleton alone (invisible today only because no part contains `{{ }}`). The
  implementation must keep the inputs to `targetConfigHash` byte-identical so every `ConfigHash`
  is unchanged and the tree shows zero drift: in particular, deciding whether the hash sees the
  sentinel skeleton or the raw bodies, and ensuring a literal `{{ .vars.x }}` in a raw part is not
  spuriously folded into the referenced-var set now that the part is never interpolated. The
  zero-drift `awf check` over awf's own tree is the proof.
- Render error messages stop misattributing failures to `skill`.

Doc-currency obligations the implementing commit(s) must satisfy:
- The `parts-raw` invariant's backing test (`// invariant: parts-raw`) lands in the same change
  that flips this ADR to `Implemented`; no existing Implemented invariant is retired
  (`retires_invariants: []`).
- `docs/architecture.md` updates its render-flow note (currently "assembles section overlays ...
  then executes the template", layout/render-flow sections) to record that convention-part bodies
  are protected from `text/template` and substituted after execution.
- The `rendering` domain narrative (`.awf/domains/parts/rendering/current-state.md`) notes that
  parts are raw, never templated (ADR-0019 staleness fires for a rendering-domain ADR reaching
  `Implemented`).
- The status flip to `Accepted`/`Implemented` regenerates `docs/decisions/ACTIVE.md` and
  `docs/domains/rendering.md` (this ADR carries `domains: [rendering]`, so ADR-0033's
  ADR→domain-index co-change applies) via `./x sync`, staged in the same commit.
- No `docs/decisions/README.md` row is owed: the index is the generated `ACTIVE.md`; the README
  is a how-to (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Document the Go idiom (`{{ "{{" }}`) and improve the error only | Leaves parts templated, so the ambiguity and trap remain; relies on the author knowing an obscure trick. The adopter explicitly wanted a real fix. |
| A raw-fence marker (`awf:raw ... awf:endraw`) inside otherwise-templated parts | Keeps parts templated by default and forces authors to fence every example; partial protection with the same trap one forgotten fence away. |
| Auto-escape braces that do not parse as a valid action | Magical and fragile: indistinguishable from a typo'd action; surprising failure modes. |
| Make parts raw via per-segment execution (template defaults individually, append parts raw) | Breaks if a control-flow action ever spans a section boundary; the placeholder single-pass is robust regardless. |
