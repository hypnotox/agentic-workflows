---
status: Proposed
date: 2026-07-05
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [invariants, rendering, config, drift, init]
related: [0008, 0029, 0045, 0051, 0055, 0057]
domains: [rendering, config, invariants]
---
# ADR-0064: Config-derived invariant comment markers

## Context

Invariant backing comments are language-specific: the checker (ADR-0008) matches
`<marker> invariant: <slug>` where the `marker` is drawn from `invariants.sources[]` in
`.awf/config.yaml` — a **list**, each source pairing `globs` with its own `marker`. A polyglot
project legitimately carries several markers at once (`//` for `*.go`, `#` for `*.py`).

Two places in the codebase contradict this list-of-markers model by asserting a single marker:

1. **Hardcoded `//` in guidance.** The rendered ADR-system README's "Invariant tagging" section
   instructs adopters to "add a matching `` `// invariant: <slug>` `` comment". The `//` is a
   literal in both the template default (`templates/adr-readme/README.md.tmpl`) and awf's own
   section override (`.awf/parts/adr-readme/invariants.md`, which is why awf's rendered
   `docs/decisions/README.md` shows `//` — the part wins over the template default). A Python or SQL
   adopter is told the wrong marker.
2. **Single-marker init descriptors.** ADR-0029 Decision item 1 models the marker as two init
   descriptors, `invariantsMarker` (target `invariants-marker`) and `invariantsGlobs` (target
   `invariants-globs`), which `initspec.Resolve` assembles into a **single-source**
   `config.InvariantConfig`. This presents a one-marker mental model at init and is the origin of the
   recurring misconception that a flat `invariantsMarker` var is reachable from templates (it is not —
   it feeds `invariants.sources`, never the `.vars` render namespace). Both descriptors default to
   `""`, so default and non-interactive init already seed **no** invariants config; the descriptors
   only act in interactive init or via `--set invariantsMarker=…`.

The project already has the machinery to derive doc prose from config: the commit-scope taxonomy
(ADR-0051/0055/0057) compiles `audit.allowedScopes` into a `text/template` render key
(`commitScopesDisplay()` → `.commitScopes`), a set of `{{=awf:commitScope…}}` placeholders for RAW
convention parts, and a confighash fold (`render.ReferencesScopes` for the template path plus a
part-bytes analog) so a scopes edit reflags every dependent artifact. Invariant markers should be
derived the same way rather than hand-asserted.

Grounding check confirmed: `{{=awf:key}}` substitution runs only over RAW convention parts, not
`.tmpl` output (`substitutePlaceholders` in `planSections`); a `.vars`-independent render key does
not trip `var-descriptor-parity`; and removing the two init descriptors needs no config
schema-generation bump or `awf upgrade` migration (existing configs already carry
`invariants.sources`).

## Decision

**Thesis: `invariants.sources` is the single source of truth for invariant comment markers.**
Nothing else models a marker — not init descriptors, not hardcoded doc strings; guidance derives the
marker mapping from it.

1. **Derive the marker mapping into guidance, in both render forms, config-hash-folded.**
   - Add a `(*Project)` method (mirroring `commitScopesDisplay()`) that renders the glob→marker
     mapping from `p.Cfg.Invariants.Sources` and expose it as a `text/template` render key,
     `.invariantMarkers`, in the `data()` map. It renders one entry per source; a source's multiple
     globs are comma-joined (e.g. `` `*.py`, `*.pyi` → `#` ``). Ordering follows slice order
     (deterministic). The mapping derives from `Sources` regardless of `Invariants.Disabled` — the
     mapping documents the marker convention; `disabled` governs enforcement, not the convention. It
     returns `""` when `Invariants` is nil or `Sources` is empty.
     `.invariantMarkers` is an **inline sentence** form (the `commitScopesDisplay()` analog), consumed
     in the adr-readme template default with an ADR-0045 graceful fallback: `{{ with .invariantMarkers
     }}…{{ . }}…{{ else }}using your project's comment marker{{ end }}`. This is the one place the live
     mapping is rendered into a template default.
   - Add `{{=awf:invariantMarkerSentence}}` and `{{=awf:invariantMarkerTable}}` to
     `placeholderRegistry()` (backed by `invariantMarkerSentence()` / `invariantMarkerTable()` methods,
     the `commitScopeSentence()` / `commitScopeTable()` analogs), for RAW convention parts,
     present-only-when-non-empty (ADR-0057). `working-with-awf.md.tmpl` **documents** these placeholder
     keys in its existing placeholder-key table (mirroring how it documents `commitScope…`); it does
     not inject a live table — matching the commit-scope precedent, which deliberately documents rather
     than live-renders a mapping in that guide.
   - **Fold `invariants.sources` into the artifact config hash on BOTH paths.** Add a
     `render.ReferencesInvariantMarkers` analog of `ReferencesScopes` for the template-reference path
     AND a part-bytes placeholder analog in the `confighash.go` part-reading loop, folding the marker
     mapping into the hash. Both are required: when a section is part-overridden the default body's
     `.invariantMarkers` reference is replaced by a sentinel in `assembled`, so only the part-bytes
     analog catches awf's own dogfooded override. Omitting either reintroduces the drift-oracle blind
     spot (`awf check` clean while `awf sync` rewrites).
   - **Dogfood the placeholder.** Edit the existing `.awf/parts/adr-readme/invariants.md` override so
     its guidance sentence is **reworded** around `{{=awf:invariantMarkerSentence}}` (not a literal
     token-swap of the self-contained sentence into the inline code-span slot, which would read
     ungrammatically) in place of its hardcoded `` `// invariant: <slug>` ``, so awf's own render
     exercises the placeholder path under the gate.

2. **Drop the single-marker init descriptors.** Remove the `invariantsMarker` and `invariantsGlobs`
   catalog descriptors, the `case "invariants-marker"`/`"invariants-globs"` collection and the
   single-source `InvariantConfig` assembly switch in `initspec.Resolve`, the now-dead `*config.InvariantConfig`
   return threaded through `cmd/awf/init.go` into `project.ScaffoldConfig` (drop the param rather than
   pass a permanently-nil value), the `invariants-marker`/`invariants-globs` entries in the
   `descriptor_parity_test.go` `validTargets` list and the `catalog.go` target-enumeration doc
   comment, and the corresponding `initspec_test.go` / `cmd/awf/init_test.go` cases (including the
   "must be set together" error-path case). Every remaining `ScaffoldConfig` call site
   (`internal/project/scaffold_test.go`, `cmd/awf/list_add_test.go`) and the `initspec.Resolve`
   caller at `cmd/awf/init.go` drop to the new arity. Adopters configure `invariants.sources` by hand. This
   partially supersedes **ADR-0029 Decision item 1** (its invariants marker/globs descriptor clause
   only); ADR-0029 stays `Implemented`.

## Invariants

- `inv: invariant-markers-derived` — the adr-readme invariant-tagging guidance renders its
  comment-marker mapping from `invariants.sources` (via the `.invariantMarkers` render key in the
  template default and `{{=awf:invariantMarkerSentence}}` in awf's own section override), never a
  hardcoded marker literal; with no sources configured it degrades to marker-agnostic generic prose.
  Backed by a real-render (`RenderAll`/`SyncReport`) test that configures a multi-source
  `invariants.sources` and asserts each source's marker appears in the rendered README — not a
  `renderGolden` hand-injected-data test, which bypasses `data()`.
- `inv: invariant-markers-in-confighash` — an artifact that references the marker mapping (template
  render key or part placeholder) folds `invariants.sources` into its config hash, so editing
  `invariants.sources` reflags that artifact stale in `awf check`. Backed by a confighash test on
  both the template-reference and part-placeholder paths.
- `inv: no-single-marker-init-descriptor` — the catalog exposes no `invariants-marker` or
  `invariants-globs` var descriptor; the marker reaches config only through `invariants.sources`.
  Backed by the descriptor-parity / init-resolution tests.

## Consequences

- Guidance is correct for every configuration: single-language, polyglot, or none. awf's own README
  now renders `` `*.go` → `//` `` derived from its config instead of a hardcoded token.
- The marker-injection mechanism becomes symmetric with the commit-scope taxonomy — same render-key +
  placeholder + confighash-fold shape — so there is one pattern to learn and extend.
- `awf init` no longer prompts for a marker/globs or accepts `--set invariantsMarker=…`. The
  out-of-box default is unchanged (both defaulted `""`, seeding no invariants config), so only the
  interactive/`--set` seeding path is lost; adopters enabling invariants write `invariants.sources`
  directly, which the working-with-awf note now documents.
- awf's own config is single-source (`*.go → //`), so the polyglot multi-source rendering is **not**
  exercised by awf's dogfood and must be covered by explicit multi-source tests.
- No config schema-generation bump and no `awf upgrade` migration: the change is init-only plumbing;
  the `config.InvariantConfig` struct and lock schema are untouched.

Doc-currency obligations the implementing commit(s) must satisfy:

- The rendered `docs/decisions/README.md` ("Invariant tagging") and `docs/working-with-awf.md`
  re-render via `./x sync` in the same commit as the template / part / `data()` change.
- When this ADR flips to `Implemented`, `./x sync` regenerates `docs/decisions/ACTIVE.md`. No
  `docs/decisions/README.md` index row is owed (that README is the rendered how-to guide; `ACTIVE.md`
  is the generated index — ADR-0003/0004), and AGENTS.md needs no change (its "Backed invariants"
  bullet already describes the marker generically as `<marker>`).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the hardcoded `//`, just reword marker-agnostic ("your project's comment marker") | Correct but strictly worse: the concrete glob→marker mapping is available in config and more useful to an agent writing a backing comment; deriving it costs one method and mirrors an existing pattern. |
| Expose a single derived marker (e.g. first source's) as a `.vars`-style value | Misrepresents polyglot configs (arbitrarily picks one of N markers) — the exact defect being removed. |
| Render key only, no `{{=awf:}}` placeholder | Leaves convention-part overrides (including awf's own dogfooded section) unable to inject the mapping, and would not exercise the RAW-part path. Both forms mirror the commit-scope precedent. |
| Two separate ADRs (derive; drop descriptors) | The init-descriptor removal is justified *by* the derived mapping making sources the sole marker source — one thesis, kept as one record with two decision items. |
| Keep the single-marker init descriptors | Perpetuates the one-marker mental model and the "flat `invariantsMarker` var" misconception that produced the original hardcoded-marker drift. |
