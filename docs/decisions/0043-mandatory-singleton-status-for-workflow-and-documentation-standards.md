---
status: Implemented
date: 2026-07-01
supersedes: []
retires_invariants: [workflow-ref-fallback]
superseded_by: ""
tags: [doc-singletons, schema-migration]
related: [4, 13, 18, 20, 21, 22, 29, 31, 37]
domains: [rendering, config]
---
# ADR-0043: Mandatory Singleton Status for Workflow and Documentation Standards

## Context

`workflow`, `doc-standard`, and `agents-md-standard` are `core: true` entries in the toggleable
`docs:` catalog (`templates/catalog.yaml`). ADR-0022 gives `core` one meaning only: pre-selected at
`awf init`, never a guarantee of presence; its Decision item 4 states this explicitly: "`core`
governs only the init default, never availability... a non-core doc is opted in by adding its name
to the `docs:` array directly." Any of the three can be individually removed via
`awf remove doc <name>` or ADR-0029's catalog-trim multiselect at init.

This surfaced a live bug: `templates/docs/agents-md-standard.md.tmpl:3` links unconditionally to
`doc-standard.md`. Enabling `agents-md-standard` while disabling `doc-standard` (a reachable state
today) makes ADR-0020's dead-reference gate fail `awf check`. Reproduced directly: initializing a
project with `docs=agents-md-standard,workflow` (no `doc-standard`) yields
`dead-reference docs/agents-md-standard.md: doc-standard.md`.

The project owner's judgment (verbatim): "Let's make them core to the workflow and not togglable.
Both standard docs and workflow must exist, since they are loadbearing to explain the workflow and
how documentation should look like. All else that can be toggled should only be referenced if they
are enabled." A repo-wide sweep found no other unconditional reference to a toggleable doc: the
existing `.layout.docs.pitfalls`-style conditionals and the `requiresDoc` skill-gate (ADR-0013's
`doc-gated-skill-suppressed`) already cover the rest correctly, so the second half of that judgment
requires no new mechanism, only documenting the rule.

Four artifacts are already always-on, non-togglable singletons: `agents-doc` (ADR-0004),
`adr-readme` and `adr-template` (ADR-0021), and `plans-readme` (ADR-0021). `agents-doc` is
special-cased in its own code path (root-level output path, CLAUDE.md-bridge interplay, ADR-0037);
the other three share one hand-rolled render loop and a second, independently hand-duplicated
validation loop (`internal/project/render.go:195-217`, `internal/project/validate.go:34-51`):
already two copies of the same tuple list with nothing to catch drift between them.

A grounding sweep found two things a first pass at this design missed, both concrete rather than
hypothetical:

- **This repo's own tree would break on landing a config-only migration.** `.awf/config.yaml`
  currently enables `workflow`/`doc-standard`/`agents-md-standard` as plural `docs:` entries, and
  `.awf/docs/parts/workflow/local-hooks.md` is a real convention-part override. Singleton kinds
  resolve sidecar and part paths differently from plural kinds
  (`internal/config/config.go` `Sidecar`/`PartPath`, the `IsSingletonKind` branch): `.awf/<kind>.yaml`
  and `.awf/parts/<kind>/<section>.md`, not `.awf/docs/<kind>.yaml` and
  `.awf/docs/parts/<kind>/<section>.md`. A migration that only strips the `docs:` array entry would
  silently orphan that override. `internal/migrate/treelayout.go`'s `portAgentsDoc` already solved
  this exact problem when `agents-doc` first became a singleton and is the precedent to follow.
- **The Document Map would silently lose these three entries.** `templates/agents-doc/AGENTS.md.tmpl:75-84`
  already hardcodes static lines for the other singletons ("ADR index", "Active ADRs", "Plans") ahead
  of the `{{ range .docs }}` loop that lists plural, toggleable docs. `workflow`/`doc-standard`/
  `agents-md-standard` render into that same loop today (via their `DocSpec` title/desc). Once they
  leave `p.Cfg.Docs`, they vanish from the loop with nothing replacing them, directly undermining
  the stated reason for this change (they're meant to be *more* discoverable, not less).

A second finding narrowed the design rather than blocking it: `internal/config` depends on nothing
but the standard library and `yaml.v3`: it does not import `internal/catalog`, and `internal/catalog`
does not import `internal/config` either. Only `config → project` would cycle (`project` already
imports both). So a single exported name list can live in `internal/catalog` and be imported by both
`internal/config` and `internal/project`, rather than settling for two independently-maintained lists
merely guarded by a parity test.

## Decision

1. **Promote `workflow`, `doc-standard`, `agents-md-standard`** from toggleable `docs:` catalog
   entries to always-on singletons, joining `adr-readme`/`adr-template`/`plans-readme` (`agents-doc`
   stays separately special-cased). They keep the existing singleton family's `local: true` sidecar
   escape hatch: "not togglable" means "cannot be removed from the render set," not "can never be
   hand-maintained."

2. **`internal/catalog` becomes the single source of truth for singleton identity.** Replace the
   `AdrReadme`/`AdrTemplate`/`PlansReadme` named `TargetSpec` fields on `catalog.Catalog` with one
   `Singletons map[string]TargetSpec`, loaded from one `singletons:` map in `templates/catalog.yaml`
   (six kebab-case keys: `adr-readme`, `adr-template`, `plans-readme`, `workflow`, `doc-standard`,
   `agents-md-standard`) instead of three separate top-level keys plus three new ones. Export a
   `catalog.SingletonKinds() []string` (or equivalent) derived from that map's keys. Remove the
   `workflow`/`doc-standard`/`agents-md-standard` entries from the `docs:` catalog block, and delete
   `DocSpec.Core` (dead once no `docs:` entry sets `core: true`; `SkillSpec.Core` is untouched: core
   skills remain toggleable-but-default-selected).

3. **`internal/config.IsSingletonKind`** becomes a membership check against `catalog.SingletonKinds()`
   instead of a hardcoded 4-case switch; `internal/config` importing `internal/catalog` introduces
   no cycle (verified: `catalog` has zero internal dependencies).

4. **New `internal/project/singleton.go`** declares one package-level table, `plainSingletons`:
   the six non-`agents-doc` singletons, each an entry of `{kind, templateID, outPath func(Layout)
   string, sections func(*catalog.Catalog) []string}`. This is the single source of truth within
   `internal/project` for what a plain singleton is and how it renders; a test asserts its kind names
   match `catalog.SingletonKinds()` exactly (minus `agents-doc`).

5. **`render.go`'s hand-rolled loop (lines 195-217) is deleted**, replaced by one `renderKind(spec)`
   call per `plainSingletons` entry (`renderKind` already exists and is already reused for
   `docs`/`skills`/`agents`; `render.go:93-113`). Each spec uses `names: []string{""}` and closures
   that return that kind's fixed template ID / sections / output path, ignoring the unused name and
   target arguments. This removes duplicated *logic*, not just a duplicated data table: `renderKind`'s
   sidecar-lookup → skip-if-local → render → append shape is proven identical for a single-name,
   neutral-target case (`Sidecar`/`PartPath` already special-case `IsSingletonKind` and ignore the
   artifact name; `outPath`/`tid`/`sections` closures for a neutral singleton ignore `Target`; a nil
   `gate` always renders).

6. **`validate.go`'s separately hand-duplicated 3-tuple list (lines 34-51) is replaced** by a loop
   over the same `plainSingletons` table. Its check shape (section-override validation only) does not
   map onto `renderKind`, so it keeps its own small loop, but references the one shared table instead
   of maintaining a second hardcoded list.

7. **`internal/project/layout.go`** gains `DocStandard`/`AgentsMdStandard` string fields (matching
   the existing `AdrReadme`/`AdrTemplate`/`PlansReadme` pattern) and matching `templateMap()` keys, so
   every existing template keeps citing `.layout.*` names unchanged: no template-facing contract
   break. `WorkflowRef` keeps its existing key name, but its `AGENTS.md` fallback branch is deleted:
   workflow can no longer be disabled, so the path is always `<docsDir>/workflow.md`. This retires
   `inv: workflow-ref-fallback` (ADR-0013) via this ADR's `retires_invariants` frontmatter; the two
   backing test cases for the deleted fallback arm (`internal/project/project_test.go:543,563`) are
   deleted in the same change, not just the invariant tag.

8. **A new migration** (`internal/migrate`, registry entry `{To: 6, Name: "singleton-standard-docs"}`)
   ports each of the three promoted docs, mirroring `treelayout.go`'s `portAgentsDoc` precedent: for
   each name, relocate `.awf/docs/<name>.yaml` to `.awf/<name>.yaml` if present, relocate
   `.awf/docs/parts/<name>/` to `.awf/parts/<name>/` if present, then strip `<name>` from the `docs:`
   array if present (membership-checked before removal: `internal/config`'s `SetArrayMember` errors
   on removing an absent member or from an absent key).

9. **`internal/project/scaffold.go`**: `ScaffoldConfig` no longer seeds a `docs:` array from
   `core: true` doc entries: after this change none remain, so the generated `docs:` key is empty or
   omitted, matching how an empty `domains:` is already omitted.

10. **`templates/agents-doc/AGENTS.md.tmpl`'s document-map section** gains three more hardcoded
    static lines (`Documentation Standard`, `Agent Guide Authoring Standard`, `Workflow`) citing
    `.layout.docStandard` / `.layout.agentsMdStandard` / `.layout.workflowRef`, in the same style as
    the existing "ADR index"/"Active ADRs"/"Plans" lines, ahead of the `{{ range .docs }}` loop.

11. **`templates/docs/agents-md-standard.md.tmpl:3`**'s hardcoded `doc-standard.md` link target
    becomes `{{ .layout.docStandard }}`, per ADR-0013's "cross-references via layout, not hardcoded
    paths" principle (verified: both docs render to the same `docsDir`, so no relative-path
    adjustment is needed beyond what `.layout.docStandard` already provides). Both this field and
    (per item 10) `.layout.agentsMdStandard`/`.layout.workflowRef` are guaranteed non-empty once
    their docs become mandatory singletons (items 1, 7), so no template here can render a no-value
    token for these vars: the `missingkey=zero` constraint (ADR-0001) holds without a conditional
    guard.

12. **`docs/doc-standard.md`'s Rules section** gains one line: reference an optional/toggleable doc
    only when it is enabled, citing ADR-0020's dead-reference gate as the mechanical enforcement for
    markdown links.

13. **This ADR partially reverses ADR-0022's Decision item 4** for exactly these three docs.
    ADR-0022 is not superseded or status-flipped; it continues to govern core-but-toggleable skills
    and any future core doc, should one be added without warranting mandatory status.

## Invariants

Checkable constraints that must hold as long as this decision stands: conditions that
should trigger a new ADR if violated.

- `invariant: singleton-kind-single-source`: `config.IsSingletonKind` and `internal/project`'s
  `plainSingletons` table both derive their kind-name membership from `catalog.SingletonKinds()`; a
  test asserts the two sets are identical (accounting for `agents-doc`, which `IsSingletonKind`
  includes but `plainSingletons` deliberately excludes).
- `invariant: plain-singleton-via-renderkind`: a table-driven test exercises `RenderAll` for each of the
  six plain singletons (`adr-readme`, `adr-template`, `plans-readme`, `workflow`, `doc-standard`,
  `agents-md-standard`) and asserts each produces its expected output path and content through
  `plainSingletons` + `renderKind`.
- `invariant: singleton-doc-migration-relocates-parts`: the `singleton-standard-docs` migration relocates
  both the sidecar file and the convention-part directory for each promoted doc, not only its
  `docs:` array entry.
- `invariant: document-map-lists-mandatory-docs`: `AGENTS.md`'s document-map section always cites
  `.layout.workflowRef`, `.layout.docStandard`, and `.layout.agentsMdStandard`, regardless of the
  project's `docs:` array contents.
- `invariant: mandatory-docs-not-in-docs-catalog`: `templates/catalog.yaml`'s `docs:` block contains no
  entry named `workflow`, `doc-standard`, or `agents-md-standard`, and `DocSpec` carries no `Core`
  field.

## Consequences

`workflow`/`doc-standard`/`agents-md-standard` can never silently disappear from a project, and
every existing or future cross-reference to them can rely on unconditional presence instead of a
fallback or a conditional guard: the `workflow-ref-fallback` invariant's removal is itself evidence
of that simplification. Consolidating the plain-singleton family onto one table and the existing
`renderKind` primitive means a future singleton addition touches one table entry instead of
hand-editing `render.go` and `validate.go` separately, with no automated check today catching drift
between them.

The trade-off: an adopter who previously removed one of these three docs, or excluded it via
ADR-0029's catalog-trim multiselect, has it forcibly reintroduced by the migration: only
hand-authoring via `local: true` remains available, matching the other four singletons. These three
names also disappear entirely from `awf list doc` output, consistent with (not worse than) the
existing gap that none of the other four singletons appear in `awf list` output either; this ADR
does not add a replacement listing surface. `DocSpec.Core` is deleted outright rather than left
inert, since nothing sets it once these three entries leave the `docs:` catalog.

The migration must be written and tested against this repo's own current state
(`.awf/docs/parts/workflow/local-hooks.md`, all three names enabled in `.awf/config.yaml`) as a real
fixture, not a hypothetical: a migration that only edits the `docs:` array, without the sidecar/part
relocation, would silently regress awf's own dogfooded workflow doc on the same commit that lands
this change.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep these three core-but-toggleable and make every reference to them conditional | Rejected outright by the project owner ("must exist"); also does not scale: every future reference would need permanent defensive gating instead of relying on guaranteed presence |
| Extract a dedicated `internal/singleton` package to own rendering | Would need `internal/project`'s `renderTarget`/`Layout`/`Sidecar` machinery, creating an import cycle back to `project`, or duplicating that machinery for no benefit over reusing the `renderKind` generalization already in `internal/project` |
| Two independently-maintained singleton kind-name lists (config, project), guarded only by a parity test | Superseded once grounding confirmed `internal/catalog` has zero internal dependencies and can be safely imported by `internal/config`: a genuine single source of truth is achievable, so a tested-but-duplicated pair would be needlessly weaker |
| No `local: true` escape hatch for these three (fully un-escapable) | Needless special-casing relative to the existing singleton family; "not togglable" means "cannot disappear," not "can never be hand-maintained" |
