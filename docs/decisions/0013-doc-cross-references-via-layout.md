---
status: Implemented
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [docs, layout, templates]
related: [5, 11, 81]
domains: [rendering]
---
# ADR-0013: Doc Cross-References via Awf-Given Layout, Not Vars

## Context

ADR-0005 established that documentation *paths* are awf-given structure: they are
computed from `docsDir` and exposed to templates under the `.layout` namespace, never
configured through `.vars`. The rationale was to kill a class of drift — a hand-set path
var diverging from where awf actually renders a file.

Several skill templates never moved to that model. They still locate docs through hand-set
vars, in two distinct flavours:

1. **Catalog-doc path pointers** — `workflowDoc`, `debuggingDoc`, `pitfallsDoc`,
   `roadmapDoc`. Each names the path of a doc that *awf itself renders* under `docsDir`.
   The path is already derivable via `internal/project.docOutPath` (`<docsDir>/<name>.md`);
   holding it as a var duplicates awf-given structure — exactly the drift class ADR-0005
   removed for `adrDir`/`plansDir`/`activeMd`.
2. **A doc-location pointer that is also given structure** — `stateDocsPath`. The "state
   doc" / current-position-per-domain concept is generic to the workflow ethos, and its
   home is fixed: `<docsDir>/domains`. It belongs in the layout, not in vars.

This duplication is not cosmetic. It masks live defects:

- `workflowDoc` in this repo is set to `AGENTS.md`, but the chain skills cite workflow-doc
  sections that exist in **neither** `AGENTS.md` nor the shipped `templates/docs/workflow.md.tmpl`
  ("Refactor playbook", "Planning files", "Auto-commit when green", "Regression-test
  discipline under the tier split", "Commit granularity vs gate cost"). These anchors were
  inherited from richer reference-project workflow docs and dangle.
- `stateDocsPath` is blank **and** ungated in `brainstorming`/`proposing-adr`/`adr-lifecycle`,
  so the rendered skills read "Check state docs at ``." — an empty-backtick artifact.
- `pitfallsDoc` is ungated in `bugfix`, so an adopter who enables `bugfix` without the
  pitfalls doc renders the same empty-backtick artifact.
- `roadmap-graduation` interpolates `roadmapDoc` into its frontmatter `description`. Because
  `awf init` enables every skill while docs are opt-in (ADR-0004), every fresh adopter renders
  that skill with a blank doc reference — a malformed description — until they enable a roadmap
  doc the skill exists to manage.

A third group of vars is neither given structure nor generic. `oracleStateDoc` (a golden-file
testing artifact) and the `*AdrRef` family (`autonomousAdrRef`, `hostGitAdrRef`,
`keyInvariantAdrRef`, `noDivingAdrRef`, `perTaskReviewAdrRef`, pointers into a *specific*
project's ADR set) are things a project documents for itself. They do not fit the general
ethos the standard targets and should not ship in it; all are blank in this repo and wrapped
in `{{ if }}` guards, so removal drops only optional clauses.

A grounding sweep confirmed: `Config.Vars` is a free-form `map`, `render.ReferencedVars` is
the sole var-seeding path, and no Go code outside `templates/` references any of these var
names — so removing the template references is a clean delete with no schema change.

A separate, more ambitious feature — domain docs carrying frontmatter tags that drive a
generated per-domain ADR index — is deliberately **out of scope** here and reserved for its
own brainstorm/ADR. This ADR only establishes `<docsDir>/domains` as the awf-given home those
docs will live in.

## Decision

1. **Extend the `.layout` contract** (`internal/project.layout`) with three computed members:
   - `docs` — a map of every enabled doc name (each entry of `config.Docs`, local docs
     included, since a local doc still exists at that path and is citable) to its output
     path `<docsDir>/<name>.md`. A key is present iff the doc is enabled.
   - `workflowRef` — the workflow doc's path (`.layout.docs.workflow`) when the workflow doc
     is enabled, otherwise the literal `AGENTS.md`. Always resolves, so it needs no guard.
   - `domainsDir` — `<docsDir>/domains`. Always present.

2. **Migrate every doc reference in skill/agent/AGENTS.md templates from `.vars.*` to
   `.layout.*`:** `workflowDoc` → `.layout.workflowRef` (unguarded); `debuggingDoc`,
   `pitfallsDoc` → `.layout.docs.<name>`, guarded so the citation is omitted when the doc is
   not enabled; `stateDocsPath` → `.layout.domainsDir`, with "state doc" reworded to
   "domain doc". `roadmapDoc` is handled by the doc-gated-skill rule (item 4), not a guard.

3. **Uniform citation rule:** a cross-doc citation renders only when its target doc is
   enabled in the project. The workflow doc is the sole exception — `workflowRef` always
   resolves via the `AGENTS.md` fallback, so its citations never vanish.

4. **Doc-gated skills.** A skill whose entire purpose is bound to one doc declares that
   dependency in the catalog (a `requiresDoc` field on the skill's catalog entry) and is
   omitted from the render set when that doc is not enabled. `roadmap-graduation` requires the
   `roadmap` doc. This is *suppression*, not config-load validation: `awf init` enables every
   skill while docs are opt-in (ADR-0004), so a hard validation error would fail a fresh
   `awf sync`; suppression instead renders the coherent subset. Because a doc-gated skill
   renders only when its doc is enabled, its body may reference `.layout.docs.<doc>` unguarded
   yet safe — `roadmap-graduation` cites `.layout.docs.roadmap` directly (in both its body and
   its frontmatter `description`, replacing the current `.vars.roadmapDoc` interpolation) without
   a guard.

5. **Remove six non-generic vars from the standard's templates entirely:** `oracleStateDoc`
   and the five `*AdrRef` vars. Where `oracleStateDoc` was cited, keep a generic
   "never adjust expected output to make a test pass" line minus the doc-artifact reference.

6. **Soften dangling workflow-doc anchor citations:** drop the quoted section names that the
   shipped workflow doc does not contain, leaving a bare "see the workflow doc" reference.
   This ADR does not author those missing *deep-link sections* ("Refactor playbook",
   "Planning files", etc.) — a separate content concern. The concise ADR-trigger note added
   in item 7 is a cross-reference into the existing ADR README, not new section content.

7. **Workflow doc carries the ADR trigger, concisely.** `templates/docs/workflow.md.tmpl`
   gains a short "when to write an ADR" note in its chain section — the one-line
   load-bearing heuristic plus a deferral to `<adrReadme>` (`.layout.adrReadme`) for the
   format and detailed criteria. `docs/decisions/README.md` remains the authoritative ADR
   format manual; the workflow doc points to it rather than restating it. Because
   `.layout.adrReadme` is awf-given and always populated (unlike `.vars`/`.data`), this
   interpolation stays publication-safe under `missingkey=zero`; it is a deliberate, narrow
   extension of ADR-0011's static-content rule, which barred only `.vars`/`.data`
   interpolation in doc bodies.

8. **First-adopter dogfood:** this repo enables docs `architecture` (already), `workflow`,
   `testing`, `development`, `pitfalls`, and `glossary`, and removes the deleted/migrated
   vars from `.claude/awf/config.yaml`. `roadmap` is deferred to the domain-docs ADR, so
   `roadmap-graduation` — though it stays in the skills enable list — is suppressed by the
   item-4 rule and its rendered file is removed until that ADR enables the roadmap doc.

## Invariants

Constraints that must hold while this decision stands; a violation should trigger a new ADR.

- `inv: no-doc-path-vars` — No template under `templates/` references any of the migrated or
  deleted vars: `workflowDoc`, `debuggingDoc`, `pitfallsDoc`, `roadmapDoc`, `stateDocsPath`,
  `oracleStateDoc`, `autonomousAdrRef`, `hostGitAdrRef`, `keyInvariantAdrRef`, `noDivingAdrRef`,
  `perTaskReviewAdrRef`.
- `inv: layout-docs-enabled-only` — `.layout.docs` maps exactly the names in `config.Docs`,
  each to `<docsDir>/<name>.md`, and contains no other keys.
- `inv: workflow-ref-fallback` — `.layout.workflowRef` equals `<docsDir>/workflow.md` when the
  workflow doc is enabled and the literal `AGENTS.md` when it is not.
- `inv: domains-dir-given` — `.layout.domainsDir` equals `<docsDir>/domains`.
- `inv: doc-gated-skill-suppressed` — a skill whose catalog entry declares `requiresDoc: D`
  is present in the render set if and only if `D` is in the project's enabled docs. Checkable by
  rendering twice — with and without `D` enabled — and asserting the skill's output path
  (e.g. `.claude/skills/<prefix>-roadmap-graduation/SKILL.md`) appears in the render set only when
  `D` is enabled.
- **Publication-safe under all toggles** (textual) — every catalog skill/agent renders without
  a `<no value>` token and with valid frontmatter regardless of which docs are enabled,
  including the empty-docs case. (The render-all frontmatter test
  `TestAllTemplatesProduceValidFrontmatter` must be **extended** to seed the new `.layout`
  members — `docs`, `workflowRef`, `domainsDir` — and to parametrise over both the
  docs-enabled and empty-docs layouts; as written its static layout fixture omits these keys
  and never varies docs enablement, so it does not yet exercise this invariant.)

## Consequences

- The drift class ADR-0005 removed for ADR/plan paths now also covers doc paths: a doc's
  citation can never diverge from where awf renders it, because both derive from one source.
- Three live rendering defects are fixed: the dangling workflow anchors, the `stateDocsPath`
  empty-backtick artifact, and the `pitfallsDoc`-when-disabled artifact.
- A fourth default-experience defect is fixed: because `awf init` enables all skills while
  docs are opt-in, every fresh adopter rendered `roadmap-graduation` with a blank doc
  reference in its frontmatter `description`. The doc-gated-skill rule suppresses it until the
  roadmap doc is enabled, so the broken-description case can no longer occur.
- The catalog skill entry gains an optional `requiresDoc` field (a schema addition to
  `templates/catalog.yaml` + `internal/catalog`); it is hand-maintained and embedded, outside
  the per-project config tree, so it needs no `awf upgrade` migration. A doc-gated skill is
  "enabled-but-suppressed" when its doc is off: it stays in the enable list (its sidecar/parts
  are not orphaned) but is dropped from the render set, and `awf sync` removes any previously
  rendered file.
- The standard sheds project-specific cruft (`oracleStateDoc`, `*AdrRef`), making a fresh
  adopter's seeded var set smaller and more honestly generic.
- `layout()` gains a conditional branch (`workflowRef`) and a small loop (`docs`), and
  `RenderAll`'s skill loop gains a `requiresDoc`-not-enabled skip; the 100% coverage gate
  (ADR-0012) requires both `workflowRef` arms, the docs-map construction, and both arms of the
  suppression skip (doc enabled → skill rendered; doc disabled → skill skipped) to be explicitly
  exercised.
- `<docsDir>/domains` is now a reserved, awf-given location. This unblocks — but does not
  implement — the domain-docs-with-generated-ADR-index feature, which gets its own ADR.
- Removing well-known vars is not a config-schema change (vars are free-form), so no
  `awf upgrade` migration is needed; adopters simply stop seeing the vars seeded on init.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep doc paths as vars; just enable more docs and hand-set the path vars to their layout paths | Perpetuates the exact var/layout duplication ADR-0005 set out to eliminate; the hand-set paths can still drift from where awf renders. |
| Make the workflow doc always-on (a second singleton like agents-doc) instead of a fallback | Forces a doc on every adopter and is a heavier catalog change; the `workflowRef` fallback preserves today's behaviour with no forcing. |
| Doc-gated skills via config-load *validation* (error when a skill is enabled without its required doc) | Rejected in favour of *suppression* (Decision item 4): `awf init` enables all skills while docs are opt-in, so validation would fail a fresh `awf sync`. Suppression renders the coherent subset instead. |
| Reclassify `stateDocsPath` and `oracleStateDoc` together as escape-hatch vars and keep both | Conflates two different things: state/domain docs are given structure (→ layout); the oracle doc is project-custom (→ removed). Keeping either as a var was the original mistake. |
| Fold domain-doc frontmatter + generated ADR index into this ADR | Distinct load-bearing feature with its own ownership-model design (partial-file generation); bundling would make this ADR carry two decisions. |
