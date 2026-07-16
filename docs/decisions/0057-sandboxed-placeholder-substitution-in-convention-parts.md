---
status: Implemented
date: 2026-07-04
supersedes: []
superseded_by: ""
tags: [convention-parts, placeholder-registry]
related: [1, 12, 15, 34, 45, 51, 55, 56]
domains: [rendering, config]
---
# ADR-0057: Sandboxed placeholder substitution in convention parts

## Context

ADR-0034 made convention parts **raw input** (a part body is never parsed or executed by
`text/template`, and a literal `{{` renders byte-for-byte) to kill an adoption trap where
template-shaped example prose (`{{ ... }}`) broke `awf sync` with an opaque error. That contract is
right for its purpose, but it has a cost: a part cannot interpolate *any* awf-owned value. So the
`docs/workflow.md` commit-discipline taxonomy (a raw part) must hand-write the commit-scope tokens,
re-creating exactly the drift ADR-0051 and ADR-0055 removed on the templated surfaces (guide,
reviewing skills). Editing `audit.allowedScopes` silently leaves that table stale.

We want a raw part to consume a **closed, curated** set of config-derived values with zero drift,
without reopening the ADR-0034 trap. The key realisation: the trap comes from running parts through
`text/template`, where a literal `{{` is genuinely ambiguous. A *literal substitution* of a small,
namespaced, exact-match sentinel set is a different mechanism entirely: it never invokes the
template engine, so a stray `{{ }}` in prose stays inert, and the only tokens it touches are ones no
prose contains by accident. ADR-0056 gives scopes a `meaning` field; this ADR is the general
mechanism that lets a part render that structured data (and other config-derived values).

The render pipeline (ADR-0034 Decision item 2): `render.Assemble` emits a brace-free sentinel where
a section is satisfied by a part and returns a `sentinel → raw body` map; `render.Execute` runs
`text/template` over the sentinel skeleton, then restores raw part bodies verbatim
(`internal/render/render.go`). Two facts shape this ADR. First, `Execute` returns only the final
string (after restoration a part body is indistinguishable from default text), so a post-`Execute`
pass could not be scoped to parts. Second, the project layer reads each raw part body from disk in
`planSections` (`internal/project/render.go`) *before* handing it to `Assemble` as
`SectionPlan.PartBody`; that is the one place a part body is isolated and the project layer (which
knows the config) is in control. The drift oracle hashes consumed parts by their **raw bytes**
(`internal/project/confighash.go`) and folds scope data only when the *template* source matches
`render.ReferencesScopes` (`\{\{[^{}]*[.$]commitScopes[^{}]*\}\}`); neither notices a placeholder
in a part.

## Decision

1. **Sandboxed placeholder syntax `{{=awf:identifier}}`.** A closed, namespaced token resolved by
   literal substitution. The `awf:` namespace is self-documenting and avoids the Mustache
   `{{=...=}}` set-delimiter prefix overlap; it is not valid Go/Jinja/Mustache. The strict grammar is
   a single identifier: `\{\{=awf:([A-Za-z][A-Za-z0-9]*)\}\}`.

2. **Literal substitution in the project layer, never `text/template`.** Substitution runs in
   `planSections`, on the raw part body read from disk, *before* it becomes `SectionPlan.PartBody`.
   `internal/render` stays ignorant of `awf:` semantics; the substituted body flows through the
   existing sentinel/restore path unchanged: `Assemble` stands it behind a NUL sentinel (the body
   itself never enters the `assembled` skeleton) and `Execute` restores it verbatim into the final
   rendered `content`, which the existing `<no value>` publication-safety check scans
   (`internal/project/render.go`). Because substitution happens pre-`Assemble`, a part is still
   identifiable when it is rewritten; this is *not* a post-`Execute` pass, where a restored body is
   indistinguishable from default text. The placeholder mechanism's own failure modes (an
   unknown-or-empty key, a malformed residual) are hard errors raised at substitution time (Decision
   items 4-5), not deferred to the `<no value>` check.

3. **A dynamic, general registry of config-derived string generators.** Each registered key maps to
   a generator computed from the resolved config/render context. A key is **available only if it
   evaluates to a non-empty string this render.** The initial inclusive set: `commitScopeList` (the
   comma display), `commitScopeTable` (a markdown `name | meaning` table, hard-depends on
   ADR-0056's `Meaning`), `commitScopeSentence`, plus general values such as `prefix` and the gate
   commands. The registry is designed to grow; scopes are its first consumers. The same generators
   feed the Go render context where a templated default wants them, so there is one source per value.

4. **Unknown or empty key → hard error.** `{{=awf:key}}` where `key` is not registered **or**
   evaluates to empty is a hard render error, both treated identically. The message names the part,
   the offending key, and the **available** keys, where "available" lists only keys whose value is
   non-empty this render, so it never advertises a key that would produce nothing.

5. **A second, looser leak guard.** After substitution, any residual `{{=awf` prefix in a part body
   is a hard error. The strict grammar in item 1 intentionally does not match near-misses
   (`{{=awf:}}` empty ident, `{{= awf:x}}` space, `{{=awf:commit-scope}}` hyphen); without this
   guard those would render verbatim and slip past the `<no value>` check, violating ADR-0045's
   no-unresolved-token contract. The guard turns every malformed `awf:` placeholder into a
   fail-loud error instead of published noise.

6. **Placeholder-aware confighash reflag.** The drift oracle folds resolved scope data into the
   confighash of any artifact whose raw part body references a `{{=awf:commitScope*}}` placeholder,
   generalising ADR-0051's `scopes-in-confighash` (which scans template source for `.commitScopes`).
   Raw part bodies are already read at hash time, so a scopes edit reflags a placeholder-using part
   stale in `awf check` instead of leaving it silently out of date.

**Relationship to ADR-0034.** This is a partial-item change to ADR-0034 Decision item 1, recorded
via `related` with ADR-0034 left `Implemented`: the partial-item supersedence convention
(`related` linkage, no predecessor status flip, explicit citation of the overridden item), not a
wholesale supersedence. It narrows item 1's *universal* byte-for-byte promise: a part is still never
run through `text/template`, but the closed `{{=awf:...}}` sentinel set no longer renders verbatim:
it is resolved by literal replacement before assembly. The `parts-raw` invariant is **not** retired:
its load-bearing clause ("a part body is never passed through `text/template`") stays true, and its
backing test (bare `{{`/`}}` and template-shaped prose rendering byte-for-byte) stays green,
because the sentinel is a distinct token no existing part contains. Only the invariant's "appears
verbatim in rendered output" clause admits the narrow sentinel exception. It also inverts item 1's
*guidance* that a value should be surfaced through a template default rather than a part; that
inversion is deliberate and argued in Alternatives. ADR-0001 (the `text/template` engine) is
unaffected: `{{=awf:...}}` is not a template action.

## Invariants

- `invariant: part-placeholder-sandboxed`: a `{{=awf:key}}` placeholder in a convention part is resolved
  by literal substitution (never `text/template`) against the closed dynamic registry; an
  unknown-or-empty key, or any residual `{{=awf` token surviving substitution, is a hard render
  error that fails both `awf sync` and `awf check`. Backed by substitution tests carrying the
  marker.
- `invariant: part-scopes-in-confighash`: a raw part body referencing a `{{=awf:commitScope*}}`
  placeholder folds resolved scope data into that artifact's confighash, so an `audit.allowedScopes`
  edit flags the artifact stale in `awf check`. Backed by a drift test parallel to the existing
  template-path `TestScopesEditReflagsReferencingArtifacts`.

## Consequences

- A raw part can now render a config-derived value (the workflow.md taxonomy becomes
  `{{=awf:commitScopeTable}}`), closing the last hand-written scope-token drift surface; the effort
  ADR-0055/ADR-0056 began completes here.
- The mechanism is general: future config-derived values join the registry by adding a generator, so
  adopters get a growing, drift-free vocabulary for their own overrides.
- ADR-0034's adoption-safety holds: a literal `{{ }}` in prose is still inert (only the exact
  `{{=awf:...}}` sentinel is touched), so the trap ADR-0034 closed stays closed.
- **Asymmetry, deliberate:** a part using `{{=awf:key}}` *cannot* degrade gracefully (an empty
  value hard-errors), the inverse of ADR-0045's degrade-to-generic-prose posture for awf-owned
  template *defaults*. The `{{=awf:}}` surface is opt-in override content, where fail-loud is the
  right posture; defaults keep using Go `{{ with ... }}{{ else }}...{{ end }}` for publication safety. A
  part author who wants graceful absence must not use a sometimes-empty placeholder.
- The single-storage contract gains a consumer syntax the ADR-0051 `commit-scope-single-storage`
  check (template-scoped) does not see; storage is still single-sourced in `audit.allowedScopes`, so
  this is acceptable, and the new `part-scopes-in-confighash` invariant covers the drift-detection
  gap the ADR-0051 check leaves for parts.
- New non-test code (the registry, the substitution pass, the two guards, the confighash extension)
  carries branches the 100% gate (ADR-0012) requires covering: no-placeholder fast path, known
  non-empty key, unknown key, empty-value key, near-miss residual-guard, each generator with
  populated and empty scopes, and the reflag positive/negative cases.

Doc-currency obligations the implementing commit(s) must satisfy (landing through the shared
ADR-0056 plan, ordered after ADR-0056's `Meaning` field):
- The two invariants' backing tests (`// invariant: part-placeholder-sandboxed`,
  `// invariant: part-scopes-in-confighash`) land in the same change that flips this ADR to
  `Implemented`; no existing Implemented invariant is retired (`retires_invariants: []`).
- `docs/architecture.md`'s render-flow note (currently: sentinel-fill → execute → restore verbatim)
  gains the pre-`Assemble` `{{=awf:...}}` substitution step, so the raw-part narrative stays accurate.
- The `rendering` domain narrative (`.awf/domains/parts/rendering/current-state.md`) notes that a
  raw part may consume the closed `{{=awf:...}}` sandbox (ADR-0019 staleness fires for a
  rendering-domain ADR reaching `Implemented`).
- The status flip to `Accepted`/`Implemented` regenerates `docs/decisions/ACTIVE.md` and the
  `rendering`/`config` domain indexes (this ADR carries `domains: [rendering, config]`, so ADR-0033's
  ADR→domain-index co-change applies) via `./x sync`, staged in the same commit.
- No `docs/decisions/README.md` row is owed: the index is the generated `ACTIVE.md` (ADR-0005).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Move the taxonomy into the workflow.md template *default* (ranging over ADR-0056's meaning-bearing scope data) and drop awf's raw part override | This is ADR-0034 item 1's prescribed way to surface a config value, and it does fix awf's own `docs/workflow.md`. But it only reaches sections awf ships as defaults. An adopter authors *their* section overrides as raw parts (ADR-0034), and those cannot be turned into template defaults on the adopter's behalf, so they stay drift-bound. The value asked for is a drift-free vocabulary for *adopter-authored* parts, not just awf's own; un-parting one section leaves the general capability unbuilt. |
| Make convention-part overrides fully `text/template` again | Directly reopens the ADR-0034 adoption trap: a literal `{{` in pasted prose breaks the build, with no robust escape. |
| Reuse Go template delimiters (`{{ .commitScopeTable }}`) for the sandbox | Indistinguishable from a real template action, so it cannot coexist with the raw-prose guarantee; a namespaced non-Go sentinel is what keeps prose inert. |
| Empty value → drop the placeholder's line instead of erroring | Silent line removal risks losing adjacent authored prose and hides a real misconfiguration; fail-loud is consistent with awf's gate philosophy and the user-facing contract. |
| Run substitution as a post-`Execute` pass over the final string | After restoration a part body is indistinguishable from default text, so the pass could not be scoped to parts and would force `internal/render` to learn `awf:` semantics. |
| Scope the registry to commit scopes only | The mechanism is inherently general; a closed-but-growable registry serves adopters' other overrides at no extra cost and is the value the user asked for. |
