---
status: Implemented
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [plan-taxonomy, placeholder-degradation]
related: [97, 98]
domains: [rendering]
---
# ADR-0108: Trim the plan header taxonomy to three fields

## Context

ADR-0097 Decision 1 fixed the plan narrative header at **exactly four fields** (Goal,
Architecture summary, **Tech stack**, File structure) and ADR-0098 Decision 2 embodied that
taxonomy in the rendered `plans-template` singleton, backed by the `plans-template-taxonomy`
invariant (declared by ADR-0098, proven in `internal/project/golden_test.go`). In practice the
`Tech stack` field earns nothing on a plan written *inside an existing project*:

- **Language version** is a project constant (`go.mod` / the agent guide already pin it); restating
  it on every plan is boilerplate.
- **Key packages / files touched** duplicates the **File structure** field (`created` / `modified` /
  `deleted`) and the **Architecture summary**.
- **The gate command** is a fixed project invariant, not a per-plan decision.
- A genuinely *new* dependency a plan pulls in is a load-bearing choice that belongs in an ADR, not a
  plan header field.

A second, smaller redundancy sits above the header: the standalone `positioning` one-liner ("One-line
statement of what this plan implements") asks nearly the same question as **Goal** ("What this plan
achieves").

Separately, the plans template is the one command-bearing template that hard-codes the literal English
"the gate" rather than interpolating the adopter's configured command via
`{{ with .vars.gateCmd }}...{{ else }}... {{ end }}` the way `writing-plans`, `executing-plans`,
`subagent-driven-development`, and `workflow.md` do, so the published template never surfaces an
adopter's real verify command, and awf's own render says "the gate" instead of `./x gate`.

The taxonomy is a settled, ADR-codified convention (ADR-0097/0098, both `Implemented`), so trimming it
is load-bearing and recorded here as a partial-item supersession rather than an in-place body edit.

## Decision

1. **The plan narrative header is three fields, not four.** Partially superseding **ADR-0097
   Decision 1** and **ADR-0098 Decision 2**, the canonical header is, in order: **Goal**;
   **Architecture summary** (execution shape, not rationale); **File structure**
   (`created` / `modified` / `deleted`). The **Tech stack** field is removed. Everything else in
   ADR-0097 Decision 1 (the frontmatter block, the `# Plan:` title, the phases, the optional
   Verification and Notes tails, "no other top-level sections") stands unchanged. The content
   `Tech stack` used to hold (language version, packages touched, gate command) is **dropped as
   boilerplate**, not relocated: it is already project-constant or covered by File structure /
   Architecture summary, and a new dependency belongs in an ADR.

2. **`Goal` absorbs the positioning one-liner.** The separate `positioning` section is removed; its
   job (a one-line statement of what the plan implements, the instruction to link driving ADRs in
   `adrs:` and inline, and **one line of explicit non-goals (what the plan does not do)**) folds into
   the `Goal` section. This drops the template's `positioning` `awf:section`, so the `plans-template`
   catalog `Sections` list loses its `positioning` entry (the `adr-singleton-section-parity` invariant
   (which covers Mandatory singletons like `plans-template`; `docs-section-parity` skips them) keeps
   the singleton's template markers and catalog `Sections` list in lockstep).

3. **The plans template interpolates the configured gate command.** The `plans-template` source uses
   the standard `{{ with .vars.gateCmd }}` `{{ . }}` `{{ else }}the project's gate{{ end }}` form for
   every command-bearing gate reference, degrading to generic prose when `gateCmd` is unset, matching
   the rest of the standard and satisfying the publication-safe-templates invariant. This
   publication-safety fix co-ships with Decisions 1-2 rather than standing as its own ADR because it
   edits the same `plans-template` source and rides the same render fan-out sweep; splitting it would
   fragment one file's change across two records.

## Invariants

This ADR declares **no new invariant slug**. It narrows the existing `plans-template-taxonomy`
invariant (declared by ADR-0098, one declaring ADR per slug), whose backing test
(`internal/project/golden_test.go`) drops its `## Tech stack` assertion and continues to prove the
now-three-field header. The `adr-singleton-section-parity` invariant continues to hold once the
template's `positioning` section and the catalog `Sections` entry are removed together.

- The rendered `plans-template` carries exactly the three header fields Goal / Architecture summary /
  File structure (no `## Tech stack` heading and no standalone positioning section), plus the
  frontmatter block, `# Plan:` title, a phase, and the optional Verification/Notes tails.
- The rendered `plans-template`, given a non-empty `gateCmd`, surfaces that interpolated command and
  carries no hard-coded "the gate" literal, asserted positively by the `plans-template-taxonomy`
  golden test against its own fixture's configured `gateCmd` (the absence of a plain-prose literal is
  not otherwise mechanically checkable), while `adr-singleton-section-parity` renders the singleton
  under empty vars to reject any `<no value>` leak from a bare interpolation.

## Consequences

- **Leaner plans.** Authors stop filling a boilerplate field and stop restating what File structure
  already carries; the header reads Goal → Architecture summary → File structure.
- **Non-goals get a home.** Scope boundaries move from the easily-skipped optional Notes tail into
  Goal, without adding a top-level section (ADR-0097's "no other top-level sections" holds).
- **Publication parity.** Adopters' rendered plans template now surfaces their own verify command.
- **Accepted loss.** A plan read cold and standalone (e.g. a published adopter plan) no longer states
  its language / stack at a glance; File structure gives paths, not stack orientation. Accepted
  because the stack is a project constant already recorded elsewhere (`go.mod`, the agent guide) and a
  plan is always authored and read within its project.
- **Fan-out to sweep in one change** (the recurring under-enumeration pitfall): the template source
  and its root + `examples/sundial` renders; the catalog `Sections` list; the
  `plans-template-taxonomy` backing test; the two prose surfaces that enumerate "the four fields"
  (`writing-plans` SKILL, plans README); the two sidecar prose overrides that name the four-field
  header (`plan-reviewer.yaml` section-taxonomy lens, and the `agents-doc.yaml` invariant bullet,
  whose `ref:` gains ADR-0108 alongside the text edit); a `CHANGELOG.md` `[Unreleased]` entry; and
  `./x sync` regenerating every target render, `docs/decisions/ACTIVE.md` (on the eventual status
  flip), and both lockfiles.
- **Predecessor back-pointers.** Per the partial-supersession convention (and the twice-recurred
  forward-pointer pitfall), ADR-0097 and ADR-0098 each gain `108` in their `related:` frontmatter in
  the same change; their status stays `Implemented` and their bodies are untouched.
- **Grandfathered plans untouched.** Existing `docs/plans/*.md` carrying a `## Tech stack` heading are
  never re-validated and are left as historical record.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `Tech stack`, repurpose to a "new dependencies" field | A new dependency is load-bearing and belongs in an ADR, not a plan header; the field would be empty on nearly every plan. |
| Fold the one useful `Tech stack` sentence (the verify command) into Architecture summary | The gate command is already a project constant surfaced by `gateCmd` interpolation in Decision 3; no residue needs a home. |
| Add a dedicated `## Non-goals` optional section | Adds a top-level section against ADR-0097's minimalism; a one-line non-goals prompt inside Goal is enough. |
| In-place edit of ADR-0097/0098 Decision items | Violates the append-only-ADR invariant; the change is recorded as a partial-item supersession instead. |
