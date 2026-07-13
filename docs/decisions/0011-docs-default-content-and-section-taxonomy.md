---
status: Implemented
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [section-assembly, publication-safety]
related: [4, 9, 86]
domains: [rendering]
---
# ADR-0011: Docs Default Content and Per-Doc Section Taxonomy

## Context

ADR-0004 introduced the opt-in docs module: a `docs:` catalog group whose eight entries
(`architecture`, `workflow`, `testing`, `development`, `debugging`, `pitfalls`, `glossary`,
`roadmap`) render to `<docsDir>/<name>.md` and are drift-checked like any other target. ADR-0009
later moved enablement to a flat `[]string` array and overrides to per-target sidecars plus
convention parts at `.claude/awf/<kind>/parts/<target>/<section>.md`.

What never materialised is the *content*. Each docs template is a `# Heading` followed by a single
`<!-- awf:section body -->` block whose only contents are an HTML comment instructing the reader to
"Override via `docs.<name>.sections.body` … in `.claude/awf.yaml`". That guidance is doubly stale:
ADR-0009 deleted `.claude/awf.yaml`, and the convention-part mechanism means no `replaceWith`
pointer is needed. So a fresh adopter who enables a doc gets an empty file pointing at a removed
config path. The docs module ships structure but no default — the opposite of `AGENTS.md.tmpl`,
the project's exemplar, where every marker section carries real generic prose the project edits in
place.

Two further gaps surfaced while scoping this (verified against source):

- **Docs are decomposable but undecomposed.** The render engine treats docs sections identically
  to skill sections — `renderTarget` overlays catalog-declared sections with sidecar overrides and
  convention parts with no doc-specific special-casing (`internal/project/project.go:383-389`), and
  `catalog.DocSpec.Sections []string` already exists (`internal/catalog/catalog.go:15-19`). Every
  doc nonetheless declares the single section `body`, so an adopter can only override a whole doc
  at once, never one part of it.
- **The override surface is not drift-enforced at section granularity.** `awf check`'s orphan
  detection flags a whole `<kind>/parts/<target>/` directory only when the *target* is disabled
  (`internal/project/project.go:609-625`); it does not flag a part file whose *section* is no
  longer declared. A convention part at a mistyped or removed section name is therefore silently
  ignored rather than reported — the exact failure the drift oracle exists to catch. (The *sidecar*
  half is already covered: `checkSectionsAllowed` rejects a sidecar `sections` key not in the
  target's declared set as a hard render error during `RenderAll`,
  `internal/project/project.go:77,117`. The gap is specifically the convention-part file, which is
  read by filesystem probe in `overlaySections` and never validated against the declared set.)

Adopters also lack an always-on explanation of *how to interact with awf* in a rendered project:
that `.claude` artifacts are generated, that config lives under `.claude/awf/`, that a doc section
is overridden by dropping a convention part, and that edits are followed by sync then check.
`AGENTS.md` has you-and-this-project/identity/invariants/workflow/commands/document-map but no
such section.

## Decision

1. **Docs templates ship default content.** Every docs template's body carries author-facing
   default content, never a bare placeholder comment. Content is *hybrid*: docs awf is
   authoritative about render as real generic prose true-by-default for any awf project (the
   workflow chain, the gate, the command-runner convention); inherently project-specific docs
   render a visible skeleton — `##` section headings with a one-line italic prompt under each — that
   the project fills in place.

2. **Each doc is decomposed into a named section taxonomy**, declared in `templates/catalog.yaml`
   and matched one-to-one by `<!-- awf:section NAME -->` marker blocks in the template:

   | Doc | Sections |
   |---|---|
   | `workflow` | `principles`, `chain`, `commit-discipline`, `doc-currency` |
   | `testing` | `gate`, `tiers`, `layout` |
   | `development` | `setup`, `command-runner`, `dependencies` |
   | `architecture` | `overview`, `components`, `data-flow`, `dependencies` |
   | `debugging` | `surfaces`, `recipes` |
   | `roadmap` | `ideas`, `deferred` |
   | `glossary` | `terms` |
   | `pitfalls` | `entries` |

   This taxonomy is the adopter override surface: a convention part at
   `.claude/awf/docs/parts/<name>/<section>.md` replaces exactly that section, inheriting the rest
   of the doc's defaults.

3. **A new always-on AGENTS.md section `awf-setup` ("Working with awf")** is added to
   `agentsDoc.sections` as the first section (before `you-and-this-project`), with a matching
   `<!-- awf:section awf-setup -->` block first in `agents-doc/AGENTS.md.tmpl`, with default prose:
   `.claude` skills/agents/hooks and the agent guide are
   rendered by awf from `.claude/awf/` and must not be hand-edited; targets toggle via the
   `config.yaml` enable arrays; a doc section is overridden by dropping
   `.claude/awf/docs/parts/<name>/<section>.md`; after any config or part edit, run sync then check.

4. **`awf check` gains section-level orphan detection for convention parts.** A convention part
   `<kind>/parts/<target>/<section>.md` whose `<section>` is not among the target's
   catalog-declared sections, for an otherwise-enabled target, is reported as drift. This extends
   the existing target-level orphan check in `orphans()`; it does not replace it. (Undeclared
   *sidecar* `sections` keys are already a hard render error via `checkSectionsAllowed` and need no
   change; the new code closes only the convention-part-file gap.)

5. **Doc default content stays static.** Default bodies contain no `.vars.X` / `.data.X`
   interpolation, so they render publication-safe under `missingkey=zero` with no `<no value>`
   token and no per-project seeding.

6. **The stale override hints are removed and this repo self-migrates.** The
   `.claude/awf.yaml` / `docs.<name>.sections.body` guidance is deleted from every template. This
   repo, which enables only `architecture` and overrides it via a single
   `.claude/awf/docs/parts/architecture/body.md`, redistributes that content into the four new
   section parts (`overview.md`, `components.md`, `data-flow.md`, `dependencies.md`); the stale
   `body.md` is removed and rendered output re-synced.

## Invariants

Checkable contracts that must hold while this decision stands. Tagged slugs are backed by tests
landing with implementation (enforced by `awf check` once this ADR is `Implemented`; ADR-0008);
untagged bullets are textual contracts.

- `invariant: docs-section-parity` — For every doc in the catalog, the set of declared sections equals
  the set of `<!-- awf:section NAME -->` marker blocks in its template, and the doc renders from
  template defaults with no `<no value>` token.
- `invariant: section-orphan-flagged` — `awf check` reports a convention part
  `<kind>/parts/<target>/<section>.md` as drift whenever `<section>` is not among the enabled
  target's catalog-declared sections. (An undeclared sidecar `sections` key is separately rejected
  as a render error by `checkSectionsAllowed`, covered by the existing config tests.)
- Every rendered doc body contains author-facing default content — generic prose or a visible
  `##` skeleton — and never consists solely of an HTML comment.
- Doc default content interpolates no `.vars.X` or `.data.X` token, preserving publication-safety
  under `missingkey=zero` (ADR-0001).
- The `awf-setup` section is a member of `agentsDoc.sections` and is rendered by default,
  suppressible only by an explicit per-section `drop` in the agents-doc sidecar.

## Consequences

- A fresh adopter enabling any doc receives a structured, self-explanatory file instead of an empty
  one, and a correct, current pointer to where overrides live. The brainstorming step that tells
  agents to "read architecture, workflow, testing" (ADR-0004 Context) now has real content to read.
- The section taxonomy becomes a binding surface: renaming or removing a doc section is a breaking
  change for adopters who pin a convention part to it. awf is pre-1.0/private with no stability
  guarantee, so this is acceptable now; the taxonomy is recorded here so a future change is a
  deliberate, ADR-tracked decision rather than silent drift.
- Section-level orphan detection makes a mistyped or stale part a hard `awf check` failure for
  every adopter — catching a real footgun, but also flagging parts that were previously tolerated.
  The repo's own `architecture/body.md` is migrated in the same change so the repo stays green.
- Rendered output for any project that already enabled a doc will change on the next `awf sync`
  (the body gains content); a project that hand-edited a rendered doc instead of using a part will
  see that overwritten — the intended ADR-0004 contract (diverge via parts, not by hand-editing
  rendered files), now with content worth keeping.
- No schema or struct change is required: `DocSpec.Sections` and the part/sidecar overlay already
  support arbitrary section names; the work is catalog content, template content, one Go change to
  `orphans()`, and a new render test.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep a single `body` section per doc, just fill it with content | Loses granular override; an adopter wanting to tweak one part of `workflow` must replace the whole doc and re-track upstream changes to the rest. |
| Generic prose for every doc (no skeleton) | Reads well for `workflow`/`testing` but produces misleading filler for inherently project-specific docs (`architecture`, `glossary`) — a skeleton prompts the author honestly instead. |
| Put the awf-interaction guidance in the `development` doc | Only surfaces if the project enables that opt-in doc; the interaction model applies to every adopter, so it belongs in the always-on agent guide. |
| Defer section-level orphan detection to a later ADR | Without it the new override taxonomy is not drift-enforced, and the repo's own `body.md` migration would leave a silently-ignored stale part; the enforcement and the taxonomy are one decision. |
