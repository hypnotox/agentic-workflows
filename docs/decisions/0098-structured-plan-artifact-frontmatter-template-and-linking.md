---
status: Proposed
date: 2026-07-12
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [workflow, plans, frontmatter, tooling, rendering]
related: [6, 60, 92, 95, 97]
domains: [rendering, tooling]
---
# ADR-0098: Structured Plan Artifact: Frontmatter, Template, and Linking

## Context

ADR-0097 settled the plan-authoring convention but deliberately left it a set of *textual*
contracts: a plan's structure, its plan→ADR link, and its lifecycle status are all prose —
unenforced and unreadable by tooling. The 67-plan dogfood corpus showed the cost: at least six
distinct plan→ADR citation forms, no canonical header, and a freeze marker
(`# Implementation complete`) only a human can spot. Meanwhile ADRs already carry machine-readable
frontmatter and ship both a `docs/decisions/template.md` singleton and an `awf new adr` scaffold;
plans have neither — which is precisely why ADR structure converged and plan structure did not.

This ADR gives plans the same machine-readable spine: a frontmatter schema, a `plans-template`
singleton and an `awf new plan` scaffold mirroring the ADR pair, and the enforcement that mechanizes
ADR-0097's convention — a plan→ADR link-consistency check and `awf context` integration.

Grounding that shaped the design:

- `internal/frontmatter` (ADR-0006) parses YAML frontmatter and degrades gracefully when it is
  absent (`found=false`, input unchanged) — so a frontmatter-keyed check naturally no-ops on the
  grandfathered corpus.
- The unified compile-time doc model (ADR-0060, ADR-0061) derives rendering, the `.layout` map, and
  drift-tracking from a single `DocEntry`, so `plans-template` is one catalog entry; `plansDir`
  already exists as a layout key, and `plansTemplate` is new.
- `awf context` (ADR-0092) assembles one `ContextResult`, read-only, with an outside-tree static
  fallback and human/`--json` output parity — all of which a plan data source must preserve.
- Plans live in `docs/plans/` (not the `.awf/` closed tree), and singleton docs plus layout keys
  are outside `internal/configspec` — so no closed-config-tree (ADR-0086) or config-reference
  (ADR-0088) entry is implicated.

## Decision

1. **Plan frontmatter schema.** A plan carries YAML frontmatter with `date` (ISO-8601; the scaffold
   fills it to match the filename — a convention the checks do not separately enforce), `adrs` (a
   possibly-empty list of linked ADR numbers — the structured plan→ADR link
   replacing prose citation), and `status` (`Proposed` or `Implemented`, per ADR-0097). The title is
   not a frontmatter field — it stays the `# Plan: <Title>` H1, mirroring the ADR template (whose
   title is likewise its H1). Frontmatter is required on new plans and absent on the grandfathered
   corpus.

2. **`plans-template` singleton.** A new mandatory doc renders to `docs/plans/template.md` from
   `templates/plans-template/template.md.tmpl`, added to the compile-time catalog as one `DocEntry`
   (`plansTemplate` template + layout key), mirroring `adr-template`. It embodies ADR-0097's section
   taxonomy — the frontmatter block, the `# Plan:` title, the four narrative header fields, a worked
   phase with a task and its closing commit, and the optional Verification/Notes tails — with the
   standard opener and closing-commit boilerplate pre-filled so authors copy rather than
   re-improvise.

3. **`awf new plan "<Title>"`.** A new scaffold command creates
   `docs/plans/YYYY-MM-DD-<kebab-title>.md` from the plans template with today's date filled and
   marker comments stripped, refusing to overwrite an existing file. Unlike `awf new adr` it
   allocates no sequential number (plans are date-prefixed) and seeds no vars; its scaffolding logic
   is factored into a home independent of `internal/adr` (whose scaffold is coupled to ADR
   numbering).

4. **Plan→ADR link-consistency check.** `awf check` gains a check that reads the plan files under
   `docs/plans/` matching the `YYYY-MM-DD-*.md` filename pattern — which excludes the `template.md`
   singleton and `README.md`, just as the numeric `NNNN-*.md` pattern excludes them for ADRs — and,
   for each plan carrying frontmatter, fails when an `adrs:` entry names a number with no
   `docs/decisions/NNNN-*.md` file. Plans without frontmatter (the grandfathered corpus) are skipped.
   This is distinct from the rendered-file dead-reference check (ADR-0020), which scans only
   awf-managed rendered output; the disk-scan and filename-exclusion precedent is `adr.ParseDir`.

5. **`awf context` plan integration.** `awf context <paths>` additionally surfaces the plans linked
   (via `adrs:`) to the ADRs it already reports for those paths, carried in the single assembled
   `ContextResult`. The command stays read-only, keeps its outside-tree static fallback, and reports
   the same plan set in human and `--json` renderings. Because plans declare `adrs:` and not
   `paths:`, the join is transitive: path → owning domain → ADR → plans linking that ADR.

6. **Frontmatter parse-validation.** Over the same `YYYY-MM-DD-*.md` scan set, `awf check` fails on
   a plan whose frontmatter is present but malformed — unparseable YAML, or a `status` outside
   {`Proposed`, `Implemented`}. Frontmatter-less plans pass unchanged.

## Invariants

Tagged below; each is backed with a `// invariant: <slug>` comment by the implementing plan (which
links ADR-0097 and this ADR). `awf check` enforces them once this ADR is `Implemented`.

- `inv: plan-frontmatter-validated` — `awf check` fails on a plan with present-but-malformed
  frontmatter (unparseable YAML, or a `status` not in {`Proposed`, `Implemented`}); a plan with no
  frontmatter passes.
- `inv: plan-adr-link-resolved` — `awf check` fails when a frontmatter-bearing plan's `adrs:` entry
  names an ADR number with no matching `docs/decisions/NNNN-*.md`; frontmatter-less plans are
  skipped.
- `inv: plans-template-taxonomy` — the rendered `plans-template` (`docs/plans/template.md`) carries
  the frontmatter block (`date` / `adrs` / `status`) and ADR-0097's section taxonomy (the `# Plan:`
  title, the four narrative header fields, a phase, and the optional Verification/Notes tails) — a
  plans-specific check beyond the doc model's generic render-and-leak-free coverage.
- `inv: plan-new-unnumbered` — `awf new plan "<Title>"` scaffolds `docs/plans/YYYY-MM-DD-<slug>.md`
  from the plans template with today's date and no sequential number, refusing to overwrite an
  existing file.
- `inv: context-surfaces-linked-plans` — `awf context` includes, in its single `ContextResult`
  (human and `--json` alike), the plans whose `adrs:` link an ADR it surfaces for the queried paths,
  without mutating anything.

## Consequences

- **Easier:** plans become machine-readable and self-describing; the plan→ADR link is structured,
  enforced, and queryable; `awf new plan` gives adopters a one-command canonical plan, closing the
  ADR/plan template asymmetry that caused the header divergence; `awf context` now answers "what
  plans touch this?".
- **Trade-offs accepted:** a new check and a new command expand the tooling surface and the
  100%-coverage burden. The transitive path→plan join means a plan surfaces for a path only through a
  shared ADR/domain, so a plan linking no ADR (non-ADR work) is invisible to path queries — accepted,
  since such plans have no ADR anchor to join on. Frontmatter validation binds only
  frontmatter-bearing plans, so the grandfathered corpus stays exempt by construction.
- **Ruled out:** bidirectional linking (a `plans:` field on every ADR) — the chosen consumers work
  from the plan side alone, and a back-reference would expand scope into the ADR template, the
  `awf-adr-lifecycle` skill, and every existing ADR for no consumer benefit. A generated plan index
  (an `ACTIVE.md` analog) — deferred; no chosen consumer needs it.
- **Downstream:** this ADR's invariants are backed by the single implementing plan (linking ADR-0097
  and this ADR); per ADR-0097's ordering note, the frontmatter mechanism here is sequenced ahead of
  α's lifecycle wiring. The `plans-template` and prose changes land in the `internal/catalog`
  defaults, so they re-render `examples/sundial` (which must stay zero-notes per ADR-0090) and stay
  publication-safe and generic (ADR-0001, ADR-0045, ADR-0082). The implementing plan also updates
  `AGENTS.md` in the same commit that lands the change: an `awf new plan` entry in the Commands
  section and the five new invariant bullets in the Invariants list, per the docs-travel-with-the-
  change rule.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Bidirectional plan↔ADR linking (a `plans:` field on every ADR) | The chosen consumers (link check, `awf context`) work from the plan side alone; a back-reference expands scope into the ADR template, `awf-adr-lifecycle`, and all existing ADRs for no benefit. |
| Title as a frontmatter field | `adr-template` keeps the title in its H1 and all 67 existing plans use `# Plan:`; a frontmatter title would diverge from the ADR pattern and need the H1 kept in sync. |
| Extend the dead-reference check (ADR-0020) for plan links | That check scans only awf-rendered managed files; hand-authored plans need a separate disk scan (the `adr.ParseDir` precedent). |
| Reuse `internal/adr`'s scaffold for `awf new plan` | It is coupled to sequential ADR numbering; a plan scaffold is cleaner factored into its own home. |
| A generated plan index (`ACTIVE.md` analog) | Deferred — no chosen consumer needs it; adds a generated artifact and drift surface with no current reader. |
