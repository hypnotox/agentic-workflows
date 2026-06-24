---
status: Accepted
date: 2026-06-24
supersedes: []
superseded_by: ""
tags: [tooling]
related: []
---
# ADR-0001: Template Overlay Rendering Engine

## Context

The `awf` generator must render project-specific `.claude` skill/agent/hook files from shared
templates plus a small per-project `awf.yaml` config. Two engine shapes were considered:

- **Engine A** — a DSL or structured format where sections are declared explicitly and composed
  at a data level before any rendering.
- **Engine B** — marker-delimited markdown + overlay: templates are plain markdown that read as
  finished skills, with Go `text/template` interpolation and named section markers
  (`<!-- awf:section name -->` / `<!-- awf:end -->`) that the overlay can address per project.

Templates need to accommodate variable interpolation (`{{ .vars.testCmd }}`), data loops
(`{{ range .data.testSurfaces }}`), and per-project section overrides (replace a section body
with a project-authored file, or drop it entirely). The rendered output must contain **no**
generator metadata in its body — provenance lives only in `awf.lock`.

Three reference projects (`a reference project`, `a reference project`, `a reference project`) share near-identical skill families that
have already drifted. The engine must remain legible to skill authors reading raw templates.

## Decision

Use **Engine B**: marker-delimited markdown + overlay rendered with Go `text/template` and
`missingkey=zero`.

Overlay resolution per section, in priority order:
1. `drop: true` — omit section and markers entirely.
2. `replaceWith: parts/<file>.md` — replace section body with project-authored content (still
   interpolated).
3. Default — render the spine section with the project's `vars`/`data`.

Section markers are stripped from rendered output. The `missingkey=zero` option means a skill
enabled before its optional `data` is filled renders absent fields as empty rather than failing
`awf sync` — so `awf add <skill>` always works.

The typo/required-input safety net is provided by catalog-declared **required vars/data**
validated at config load time (`config.Validate`), not by template execution errors. A
`<no value>` render error in output is treated as a validation failure.

## Invariants

- Rendered skill/agent bodies contain **zero** `awf` markers or provenance metadata.
- `missingkey=zero` is always set on the template executor; changing it to `error` is a
  breaking change requiring a new ADR.
- Required vars declared in `catalog.yaml` are validated before rendering, not discovered
  by inspecting render output.
- Section markers (`<!-- awf:section … -->` / `<!-- awf:end -->`) appear only in `.tmpl`
  sources, never in committed rendered files.

## Consequences

Easier:
- Templates remain readable as real skills; skill authors can read and edit them without
  understanding the overlay system.
- `awf add <skill>` always succeeds even when optional `data` fields are not yet populated.
- Overlay actions (drop/replace/default) compose independently per section.

Harder:
- Section boundary parsing must be robust against nested or malformed markers.
- Template authors must remember to wrap sections that projects may want to tune.

Ruled out:
- Runtime template execution errors as the safety net for missing required vars (moved to
  `config.Validate`).

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Engine A (DSL / structured composition) | Requires skill authors to reason in a non-markdown format; templates no longer read as finished skills; higher authoring friction. |
| `missingkey=error` | Breaks `awf add <skill>` before optional data is populated; required-var validation at config load is the correct layer for this check. |
| No section markers (full-file replace only) | Eliminates per-section tuning; projects must copy entire template to change one section, defeating the point of a shared spine. |
