---
format: plan-v2
date: 2026-08-19
adrs: [make-plans-mutable-execution-guidance]
status: Proposed
---
# Plan: Implement Mutable Plan Execution Guidance

## Goal

Make every plan authoring, review, and execution path bind ADR-0286's protected contract while
allowing the implementation owner to revise non-load-bearing route metadata. Preserve typed plan
projection, lifecycle, outcome ownership, confinement, green transactions, and verification; do not
begin AF-004 clean-integration work or AF-005 review-severity reform.

## Architecture summary

Add one `plan-flexibility` partial as the semantic owner of plan-specific consequences and have it
point to the rendered workflow's protected-contract section without embedding a concrete ADR
citation or restating the doctrine. Render that partial into Full plan documentation, authoring,
review, execution, and agent surfaces. Core remains plan-free and continues to receive route
autonomy only through its existing protected-contract and implementation-autonomy surfaces. Keep
route-specific transaction and helper protocol in its existing owners. Align the parser and the
project-specific plan-reviewer focus with the authored contract: optional examples remain parseable
but are not required, `Latitude: exact` stays optional, and ordering is required only for an actual
dependency. One inline transaction adds behavioral and mutation-resistant contract tests, updates
the six declared claims with the ADR's Applied batch, renders both runtimes and their applicable
governance footprints, and records the adopter-visible change.

## Phase 1: Land the mutable-plan contract

**Execution mode: inline.**

Completes: ["plan-route-flexible", "plan-properties-preserved", "plan-contract-proven"]

### Task 1.1: Establish behavioral and parser regression oracles
Kind: batch
Applying: ["make-plans-mutable-execution-guidance:plan-flexibility-proof"]
Paths: ["internal/plan/structure_test.go", "internal/project/plan_detail_modes_test.go", "internal/project/phase_transaction_ownership_test.go", "internal/project/spine_test.go", "internal/evals/plan_flexibility_test.go"]
Representative: "A phase owner merges two planned route groupings, adds an unlisted necessary path, omits a stale listed path, and substitutes an equivalent local mechanism while preserving linked authority and Definition of Done outcomes."
Edge: "A proposed revision changes compatibility, safety, material scope, verification strength, or a linked Decision, so the owner stops for the protected-contract change rather than treating it as plan drift."
Post-check: "The focused test run exits nonzero at the pre-implementation snapshot: `TestPlanV2BatchOptionalExamples` rejects the currently mandatory Representative/Edge relationship, `TestPlanFlexibilityContract` reports the absent shared rule and contradictory consumers, and `TestPlanFlexibilityScenarios` reports the old route-binding outcomes. The failure output names those contract gaps rather than an unrelated compile, fixture, or environment error."

Before changing operative guidance, add focused tests and observe the named failures against the
current contract. In `internal/plan/structure_test.go`, add `TestPlanV2BatchOptionalExamples` to prove
a batch with Paths and Post-check but no Representative or Edge parses, while existing invalid-field,
missing-Paths, and missing-Post-check cases remain rejected. In
`internal/project/plan_detail_modes_test.go`, add
`// invariant: rendering/workflow-skill-templates:plan-flexibility (TestPlanFlexibilityContract)` on
the named test that owns the explicit consumer manifest, single authored home, projection, and
empty-data contract. Use semantic inversion mutations, not phrase absence alone, to prove route
revision, cross-owner reconciliation, actual-dependency ordering, and protected-contract escalation.

Extend `internal/project/phase_transaction_ownership_test.go` and `internal/project/spine_test.go`
only where their existing invariant markers own the affected phase, implementer, and review
contracts. Add `internal/evals/plan_flexibility_test.go` in the existing deterministic eval harness to
cover Core and Full and Pi and Claude without a live model. The scenarios distinguish permitted
merge, split, reorder, path correction, helper reassignment, and equivalent-mechanism changes from
forbidden protected-contract changes. Retain the existing checks for typed Decision references,
Advances and Completes, phase review freshness, helper confinement, and green transaction closure.

### Task 1.2: Project the single plan-flexibility rule through plan surfaces
Kind: batch
Applying: ["make-plans-mutable-execution-guidance:plan-binds-protected-contract", "make-plans-mutable-execution-guidance:route-revision-without-reapproval", "make-plans-mutable-execution-guidance:material-plan-reconciliation"]
Paths: ["templates/partials/plan-flexibility.md", "templates/partials/implementation-autonomy.md", "templates/docs/workflow.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/partials/phase-loop-continuation.md", "templates/agents/plan-reviewer.md.tmpl", "templates/agents/implementer.md.tmpl", "templates/agents/code-reviewer.md.tmpl"]
Representative: "The shared rule points to the rendered workflow's protected-contract section, treats the recorded phase and task route as revisable, and requires Notes reconciliation only when another phase or reviewer can rely on stale material instructions."
Edge: "A helper remains confined to assigned paths and cannot convert route flexibility into scope, commit, review, checkpoint, or outcome authority; a protected-contract change still returns through brainstorming."
Post-check: "`go test ./internal/project -run 'TestPlanFlexibilityContract|TestPhaseTransactionOwnershipAcrossWorkflowSurfaces|TestAuthorityGuidedImplementationAutonomy'` exits zero. `TestPlanFlexibilityContract` walks its explicit Full consumer manifest and requires exactly one include or narrow reference per entry, one authored definition, no concrete ADR token under the adopter template, and no route-binding inversion; it separately asserts that Core-only workflow surfaces contain no ADR or plan-governance prose and that configured and empty-data rendering emit no unresolved token."

Author `templates/partials/plan-flexibility.md` as the sole plan-specific rule. It references the
protected-contract doctrine in the rendered workflow rather than copying either defining clause. It
states the plan owner's route-revision authority, the protected-contract stop, the cross-owner
material-reconciliation threshold, and the distinction between commit-capable owners and confined
helpers.

Replace independently worded plan-authority clauses in the Full workflow, plans README and
scaffold, writing and reviewing skills, inline and delegated execution skills, phase-loop
continuation, and plan, implementer, and code-reviewer agents with the shared source or a narrow
reference where a full projection would be disproportionate. Preserve each surface's existing
ownership: parser vocabulary stays in plan authoring, phase transaction and recovery protocol stays
in execution, report-only classification stays in review, and helper confinement stays in the
implementer contract. In `implementation-autonomy`, retain reporting for a reasoned change another
owner can rely on but remove the rule that every added path is automatically a deviation. In the
Full code reviewer, replace literal plan-phase commit matching with protected-contract and coherent
transaction review; keep Core's coherent-boundary rule unchanged. Remove the sentence that requires
an executor not to drift from the plan. A Proposed plan is amended only before another owner could
rely on stale material route instructions; local inconsequential edits need no Notes entry.

### Task 1.3: Align structural enforcement and project review focus
Applying: ["make-plans-mutable-execution-guidance:structure-must-protect-a-property"]
Paths: ["internal/plan/structure.go", "internal/plan/structure_test.go", ".awf/agents/plan-reviewer.yaml"]
Post-check: "`go test ./internal/plan` exits zero; a batch lacking Representative and Edge but carrying Paths and Post-check parses; batches lacking Paths or Post-check still fail; unknown, duplicate, malformed, noncontiguous, or misplaced fields still fail. Rendered project plan-reviewer agents no longer demand universal `Latitude: exact` or dependency ordering without an actual protected-property dependency."

Change `internal/plan.validateTask` so `Kind: batch` continues to require Paths and Post-check but no
longer requires Representative or Edge. Preserve both fields as optional batch-only values and keep
their rejection on non-batch tasks. Do not relax phase/task identity, typed Applying and Context,
Advances and Completes, execution-mode projection, spike separation, ambiguous-scope Paths,
deterministic post-checks, helper confinement, lifecycle, or phase-close validation.

Update `.awf/agents/plan-reviewer.yaml` at its authored focus source. Remove the requirement that every
qualifying task carry `Latitude: exact`; require ordering evidence only where an actual dependency
constrains authority, outcome ownership, scope, safety, compatibility, lifecycle, or verification.
Keep its concrete checks for schema legality, executable terminal state, pair-atomic ADR batches,
generated-source closure, and gateable phase boundaries.

### Task 1.4: Apply authority, render outputs, and record the change
Applying: ["make-plans-mutable-execution-guidance:plan-binds-protected-contract", "make-plans-mutable-execution-guidance:route-revision-without-reapproval", "make-plans-mutable-execution-guidance:material-plan-reconciliation", "make-plans-mutable-execution-guidance:structure-must-protect-a-property", "make-plans-mutable-execution-guidance:plan-flexibility-proof"]
Paths: ["docs/decisions/make-plans-mutable-execution-guidance.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "changelog/CHANGELOG.md"]
Post-check: "After `./x render`, `./x check` exits zero; `docs/decisions/INDEX.md` shows the pending ADR as Implementing; `./awf context --show pending docs/decisions/make-plans-mutable-execution-guidance.md` reports no Remaining operation from the phase batch; `go test ./internal/plan ./internal/project ./internal/evals` exits zero; Full plan outputs for Pi and Claude carry the rule without `<no value>` or a concrete ADR token, while Core-only workflow outputs remain free of ADR and plan-governance prose."

Use `awf-adr-lifecycle` to transition the pending ADR from Proposed to Implementing and append one
Applied batch containing all six declared operations. In the same transaction, add the tested
`plan-flexibility` invariant and update `phase-transaction-ownership`, `plan-task-detail-modes`,
`maintainable-code-subagent-contract`, `implementer-role-contract`, and
`maintainable-code-review-lenses` with their preserved provenance and this ADR in Revised-by.
Describe the final behavior rather than the rollout.

Run `./x render` so `.awf/awf.lock`, `docs/decisions/INDEX.md`, applicable runtime skill and agent
trees, the Full workflow and plans documentation, plan scaffold, and topic output move with their
authored sources. Preserve the Core footprint's absence of ADR and plan governance. Add an
Unreleased changelog feature entry naming plan route mutability and the protected-contract stop. Perform a
focused meaning review of the generated workflow, plan README and scaffold, writing/reviewing skills,
inline/delegated execution skills, and both reviewer-agent outputs: record that they preserve the
same concepts, carry no contradictory route-binding fragment, and intentionally retain runtime- and
profile-specific protocol only in its existing owner. Run the focused test packages, `./awf check
staged`, and the full project gate before the phase close.

### Phase close

Land the canonical rule, parser alignment, all claim mutations, generated outputs, tests, and
changelog as one independently green application transaction. After the closing commit exists, run
`./x audit-local 005d1ae8d..HEAD`; require a clean result and include it in the phase-review evidence.

```commit
feat(plans): make execution routes mutable (applies ADR batch)
```

## Definition of done

- `dod: plan-route-flexible` Every plan authoring, review, and execution path permits a commit-capable owner to revise non-load-bearing route metadata and requires reapproval only for a protected-contract change.
- `dod: plan-properties-preserved` Typed authority, Definition of Done ownership, scope confinement, lifecycle, deterministic verification, helper boundaries, independently green transactions, and phase review freshness remain enforced.
- `dod: plan-contract-proven` Parser, template-contract, mutation, and deterministic cross-footprint and cross-runtime scenarios prove permitted route changes, forbidden protected changes, material Proposed-plan reconciliation, one authored rule, optional batch examples, and coherent empty-data rendering.

## Notes

The initial draft follows the reviewed pending ADR. Record material execution deviations and review
findings here only when another phase, reviewer, or terminal assurance can rely on them.

- Plan review clarified the output partition: the shared plan-flexibility rule governs Full plan
  consumers, while Core remains plan-free and retains route autonomy through its existing
  protected-contract and implementation-autonomy surfaces. This preserves the approved
  cross-footprint semantics without leaking Full governance into Core.
- During rendering, `.awf/parts/workflow/chain.md` proved to override the generic workflow chain,
  while convention-part bodies do not expand include directives. The phase owner therefore moved
  the shared include to the workflow's non-overridden Principles section instead of duplicating its
  prose in the project override. This source correction preserves the approved single-rule design
  and verifies the rendered workflow boundary another reviewer can rely on.
- Phase review found residual universal ordering language, one delegated-execution instruction that
  still bound recorded structural route detail, and proof gaps in the revised invariant claims. The
  parent treated these as authority-preserving reasoned corrections under the approved design:
  ordering now binds only a named protected property across template and catalog-backed reviewer
  surfaces, delegated execution preserves settled durable choices, and marker-backed tests cover
  every shared clause, Core autonomy, route regrouping with landed-scope preservation, and the
  revised plan and code review lenses.
