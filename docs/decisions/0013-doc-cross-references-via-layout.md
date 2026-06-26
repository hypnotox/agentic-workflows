---
status: Proposed
date: 2026-06-26
supersedes: []
superseded_by: ""
tags: [docs, layout, templates]
related: [0005, 0011]
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
   `pitfallsDoc`, `roadmapDoc` → `.layout.docs.<name>`, guarded so the citation is omitted
   when the doc is not enabled; `stateDocsPath` → `.layout.domainsDir`, with "state doc"
   reworded to "domain doc".

3. **Uniform citation rule:** a cross-doc citation renders only when its target doc is
   enabled in the project. The workflow doc is the sole exception — `workflowRef` always
   resolves via the `AGENTS.md` fallback, so its citations never vanish.

4. **Remove six non-generic vars from the standard's templates entirely:** `oracleStateDoc`
   and the five `*AdrRef` vars. Where `oracleStateDoc` was cited, keep a generic
   "never adjust expected output to make a test pass" line minus the doc-artifact reference.

5. **Soften dangling workflow-doc anchor citations:** drop the quoted section names that the
   shipped workflow doc does not contain, leaving a bare "see the workflow doc" reference.
   This ADR does not author the missing sections (a separate content concern).

6. **Workflow doc carries the ADR trigger, concisely.** `templates/docs/workflow.md.tmpl`
   gains a short "when to write an ADR" note in its chain section — the one-line
   load-bearing heuristic plus a deferral to `<adrReadme>` (`.layout.adrReadme`) for the
   format and detailed criteria. `docs/decisions/README.md` remains the authoritative ADR
   format manual; the workflow doc points to it rather than restating it. Because
   `.layout.adrReadme` is awf-given and always populated (unlike `.vars`/`.data`), this
   interpolation stays publication-safe under `missingkey=zero`; it is a deliberate, narrow
   extension of ADR-0011's static-content rule, which barred only `.vars`/`.data`
   interpolation in doc bodies.

7. **First-adopter dogfood:** this repo enables docs `architecture` (already), `workflow`,
   `testing`, `development`, `pitfalls`, and `glossary`, and removes the deleted/migrated
   vars from `.claude/awf/config.yaml`. `roadmap` is deferred to the domain-docs ADR.

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
- The standard sheds project-specific cruft (`oracleStateDoc`, `*AdrRef`), making a fresh
  adopter's seeded var set smaller and more honestly generic.
- `layout()` gains a conditional branch (`workflowRef`) and a small loop (`docs`); the 100%
  coverage gate (ADR-0012) requires both `workflowRef` arms and the docs-map construction to
  be explicitly exercised.
- `<docsDir>/domains` is now a reserved, awf-given location. This unblocks — but does not
  implement — the domain-docs-with-generated-ADR-index feature, which gets its own ADR.
- Removing well-known vars is not a config-schema change (vars are free-form), so no
  `awf upgrade` migration is needed; adopters simply stop seeing the vars seeded on init.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep doc paths as vars; just enable more docs and hand-set the path vars to their layout paths | Perpetuates the exact var/layout duplication ADR-0005 set out to eliminate; the hand-set paths can still drift from where awf renders. |
| Make the workflow doc always-on (a second singleton like agents-doc) instead of a fallback | Forces a doc on every adopter and is a heavier catalog change; the `workflowRef` fallback preserves today's behaviour with no forcing. |
| Catalog-declared per-skill doc dependencies validated at config load (skill enabled ⇒ required doc enabled) | Cleaner long-term contract, but it is a validation subsystem of its own; deferred as a possible future enhancement. The omit-when-absent rule is sufficient here. |
| Reclassify `stateDocsPath` and `oracleStateDoc` together as escape-hatch vars and keep both | Conflates two different things: state/domain docs are given structure (→ layout); the oracle doc is project-custom (→ removed). Keeping either as a var was the original mistake. |
| Fold domain-doc frontmatter + generated ADR index into this ADR | Distinct load-bearing feature with its own ownership-model design (partial-file generation); bundling would make this ADR carry two decisions. |
