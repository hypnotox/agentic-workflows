---
format: plan-v2
date: 2026-08-04
adrs: [proportional-simplicity-boundaries]
status: Proposed
---
# Plan: Implement Proportional Simplicity Boundaries

## Goal

Render a proportional-simplicity boundary that settles material design choices before implementation, preserves mechanical autonomy, and rejects speculative machinery across planning, execution, and review. Do not add a new enforcement mechanism, configuration surface, or workflow stage.

## Architecture summary

One canonical rule remains in the maintainable-code guide and projects a concise global instruction through the agent guide. Existing workflow, implementation, and reviewer templates receive stage-local obligations; existing rendering tests back the changed current-state claims. One coherent implementation transaction applies all five ADR operations because the template prose, proof assertions, rendered outputs, and claims describe one indivisible workflow contract.

## Phase 1: Render and apply the simplicity boundary

**Execution mode: subagent-driven.**
Completes: ["simplicity-guidance", "approval-boundary", "review-restraint", "authority-backed", "render-green"]

### Task 1.1: Establish the canonical rule and proportionate plan selection
Applying: ["proportional-simplicity-boundaries:simplest-sufficient-default", "proportional-simplicity-boundaries:proportionate-planning", "proportional-simplicity-boundaries:judgment-not-mechanism"]

Before dispatch, confirm the phase starts clean and green: `git status --short` prints no output, `./x check` reports clean, and `./x gate` passes.

Update `templates/docs/maintainable-code-design.md.tmpl` so Decision posture names the simplest sufficient solution as the default and permits added abstraction, indirection, validation, test machinery, tooling, cleanup, or process only for requested behavior, a reproduced defect, an existing documented contract, or a clearly applicable project invariant. State that generic robustness, hypothetical future use, and the mere possibility of doing more are insufficient. Keep the guidance language-agnostic and direct; add no section, template variable, conditional, schema, or reusable prose mechanism.

Update the Workflow section of `templates/agents-doc/AGENTS.md.tmpl` with one concise global obligation that points to `{{ .layout.maintainableCodeDesign }}` for the full rule and requires preservation of the user-approved material design boundary. Do not duplicate the full rationale.

Update the planning paragraph in `templates/docs/workflow.md.tmpl` and When to invoke in `templates/skills/writing-plans/SKILL.md.tmpl`: a plan is warranted when sequencing, coordination, or resumability materially helps; remove "When in doubt, write the plan." State that a plan records and operationalizes approved choices rather than inventing speculative structure, checks, or work. Keep the established complex examples only where they satisfy this criterion.

### Task 1.2: Enforce the approved boundary at design and implementation stages
Applying: ["proportional-simplicity-boundaries:pre-implementation-simplicity-contract", "proportional-simplicity-boundaries:material-deviation-approval", "proportional-simplicity-boundaries:stage-local-enforcement", "proportional-simplicity-boundaries:judgment-not-mechanism"]

Update `templates/skills/brainstorming/SKILL.md.tmpl` so its scaled design sections settle a proportionate simplicity contract before approval: scope and exclusions, structural approach and dependencies, patterns or abstractions, and checks and testing strategy. A straightforward change may settle these in a few sentences; do not add a separate stage or fixed form. The final approved design becomes the implementation boundary.

Add concise, context-specific preservation language to `templates/skills/executing-direct/SKILL.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`, `templates/skills/subagent-driven-development/SKILL.md.tmpl`, and `templates/skills/bugfix/SKILL.md.tmpl`. A newly discovered need affecting behavior, scope, structure, dependencies, patterns, checks, or testing strategy stops further mutation and returns to the user with the changed fact, why the approved approach no longer fits, affected approved categories, and simplest viable options. Equivalent mechanical choices remain autonomous. Preserve each skill's existing ownership and escalation structure rather than introducing a shared partial.

Update `templates/skills/tdd/SKILL.md.tmpl` so tests, checks, seams, and harness work are grounded only in changed behavior, a demonstrated regression, an existing contract, or an applicable invariant. Reject speculative test or policy machinery while retaining the mandatory red-green discipline and project gate.

Update `templates/agents/implementer.md.tmpl` to preserve the same material boundary in a no-user child. When an invalidating source fact requires user approval, the implementer stops before the affected mutation and reports the fact, affected categories, and simplest viable options. Adjust the closed stopped outcome so it accepts either a named failing check with actual output or an approval-requiring invalidating source fact; retain status, completed work, remaining work, and attempts in both cases. Do not create a third outcome or require a fabricated failing check.

### Task 1.3: Add proportionate review restraint and focused semantic proofs
Applying: ["proportional-simplicity-boundaries:stage-local-enforcement", "proportional-simplicity-boundaries:judgment-not-mechanism"]

Update the universal maintainable-design lens in `templates/agents/plan-reviewer.md.tmpl` to flag unapproved or unjustified abstraction, indirection, validation, test machinery, tooling, cleanup, or process. It must not demand additions merely because more structure, testing, cleanup, or validation is imaginable. Apply the equivalent rule to `templates/agents/code-reviewer.md.tmpl`, preserving report-only review and existing correctness obligations.

Extend existing tests rather than adding a framework or prose-wide scanner:

- In `internal/project/docs_sections_test.go`, extend `TestMaintainableCodeDesignGuide` to assert the canonical simplest-sufficient rule and its four grounds without freezing full sentences.
- In `internal/project/spine_test.go`, add the `rendering/guide-and-doc-templates:maintainable-code-design-guide (TestAgentsDocGuide)` proof marker and extend `TestAgentsDocGuide` to assert the concise global projection.
- Add the `rendering/workflow-skill-templates:mandatory-approval-boundaries (TestMaintainableCodeStageCoverage)` proof marker to `TestMaintainableCodeStageCoverage`; extend that test's per-skill expectations for the approved contract, plan-selection rule, all four implementation paths, TDD restraint, and the material-deviation categories. Assert equivalent mechanical autonomy where implementation paths render it.
- Extend `TestImplementerAgent` for the approval-requiring stopped shape and confirm no failing check is required for that alternative.
- Extend `TestMaintainableCodeReviewLenses` for both reviewers' speculative-demand restraint.

Assertions pin essential semantics and the complete named surface set, not exact paragraphs or formatting. Do not add source-substring checks, a new linter, or a standalone test helper for this change.

### Task 1.4: Apply authority, render adopters, and close green
Latitude: exact
Applying: ["proportional-simplicity-boundaries:simplest-sufficient-default", "proportional-simplicity-boundaries:pre-implementation-simplicity-contract", "proportional-simplicity-boundaries:material-deviation-approval", "proportional-simplicity-boundaries:proportionate-planning", "proportional-simplicity-boundaries:stage-local-enforcement", "proportional-simplicity-boundaries:judgment-not-mechanism"]

In `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, update `maintainable-code-design-guide` to own the canonical simplest-sufficient semantics, its valid grounds, the concise agent-guide projection, proportionate workflow plan selection, and coherent adopter-neutral rendering. Preserve Origin, append `ADR-proportional-simplicity-boundaries` to Revised-by, and retain `Backing: test`.

In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, update these claims while preserving Origin and prior Revised-by order, appending `ADR-proportional-simplicity-boundaries`, and retaining `Backing: test`:

- `mandatory-approval-boundaries`: add the proportionate brainstorming contract and renewed user approval for deviations in behavior, scope, structure, dependencies, patterns, checks, or testing strategy, while equivalent mechanical choices remain autonomous.
- `maintainable-code-stage-coverage`: add stage-local simplest-sufficient obligations across brainstorming, plan writing, TDD, direct execution, inline plan execution, subagent-driven execution, and bug fixing; plans record approved choices instead of expanding them.
- `implementer-role-contract`: allow the stopped outcome to report either a failing check or an approval-requiring invalidating source fact, while retaining the shared inventory and closed two-outcome contract.
- `maintainable-code-review-lenses`: require plan and code review to flag unapproved or unjustified machinery and forbid speculative demands merely because more work is imaginable.

Update `changelog/CHANGELOG.md` under Unreleased Features with the adopter-facing proportional-simplicity behavior, including the pre-implementation approval boundary, material-deviation stop, proportionate planning, and reviewer restraint. Keep it one concise entry.

Transition `docs/decisions/proportional-simplicity-boundaries.md` from Proposed to Implementing in this same transaction: change frontmatter status, append an Implementing event dated on execution day with the current content digest, then append one Applied event listing exactly the five declared update operations. Obtain the digest by temporarily using 64 zeros, run `./x check`, copy the reported computed digest exactly, replace the zeros, and rerun until clean; never precompute or guess it.

Run `./x render`. It must regenerate root outputs, both runtime targets, the Sundial adopter, `.awf/awf.lock`, current-state topic docs, and `docs/decisions/INDEX.md` from their authored sources. Inspect the rendered diff to confirm every changed output is attributable to the named templates or topic parts and contains no unresolved token, no value token, duplicated paragraph, or project-specific leakage. Run `go test ./internal/project`, `./x check`, and `./x gate`; each reaches a clean/pass terminal state.

### Phase close

Stage the complete transaction explicitly, including templates, tests, topic parts, ADR lifecycle files, changelog, lock, and every rendered output. Run `awf check staged` and `./x gate`; both pass. Create the single phase-closing commit:

```commit
feat(rendering): add proportional simplicity guardrails
```

## Definition of done

- `dod: simplicity-guidance` The mandatory maintainable-code guide and agent guide render the simplest-sufficient default and its grounded justification boundary without new policy machinery.
- `dod: approval-boundary` Brainstorming settles a proportionate contract, and every implementation path stops for material deviations while preserving mechanical autonomy.
- `dod: review-restraint` Plan and code review reject speculative additions without weakening correctness, contract, invariant, or regression obligations.
- `dod: authority-backed` All five ADR claim updates are Applied atomically and backed by focused existing tests that exercise their added clauses.
- `dod: render-green` Root, Pi, Claude, and Sundial outputs are regenerated, drift-free, and pass the full project gate.

## Notes

Record deviations, review findings, and any exact rendered-output surprise before the plan freezes. The settled design prohibits a new simplicity checker, schema, shared enforcement engine, or test framework.
