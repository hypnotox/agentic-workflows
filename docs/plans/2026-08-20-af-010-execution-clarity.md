---
format: plan-v2
date: 2026-08-20
adrs: []
status: Proposed
---
# Plan: AF-010 Execution Clarity

## Goal

Rewrite only the residual dense operative prose identified after AF-006 through AF-009, AF-011, and AF-013 so rules, flexible implementation details, bounded stop conditions, and required evidence are independently readable. Preserve every settled authority, safety, compatibility, ownership, review, and verification semantic; do not consolidate AF-012 concepts, add a workflow layer, or sweep the corpus.

## Architecture summary

Keep each semantic rule in its existing canonical template or partial. Restructure the grounded residue in the shared review-remediation partial, the default workflow chain, and the writing-plans, executing-plans, and reviewing-impl skill templates. Leave the already clear implementation-autonomy partial and agents-md standard unchanged. Update clause-sensitive contract tests without weakening their protected outcomes, render all consumers, and inspect the generated Pi, Claude, Core, and Full boundaries for equivalent meaning. No ADR is needed because this plan changes presentation, not a durable decision or active project rule.

The owner-approved boundary is fixed: consume the approved AF-010 topology without reopening it; rewrite only grounded residual contradictions and dense passages; preserve settled semantics; add no profile, rigor mode, depth knob, router, plan format, lifecycle state, or principle-restating skill. Stop if implementation would move canonical ownership, change a stop or approval boundary, weaken required evidence, affect safety or compatibility, or expand into AF-012 or AF-014B. After implementation and assurance are integration-ready, leave this plan Proposed and stop without integrating, closing deferred artifacts, removing topology, finishing the effort, or starting a later issue.

## Phase 1: Restructure the grounded operative residue

**Execution mode: inline.**

Completes: ["clear-operative-rules", "preserved-contract", "rendered-equivalence"]

### Task 1.1: Establish clause-preserving clarity expectations
Paths: ["internal/project/spine_test.go", "internal/project/plan_detail_modes_test.go", "internal/project/phase_transaction_ownership_test.go", "internal/evals/independent_workflow_escalation_test.go"]

Revise the focused contract tests that pin the affected passages. Treat preserved semantic-clause and canonical-consumer assertions as authority checks. Add only the least restrictive state checks needed to prove independently readable separation, without a byte, sentence, vocabulary-count, corpus-wide style gate, or choreography-only wording constraint. Keep existing protection for trigger independence, review classification, one bounded verify pass, plan-v2 task semantics, inline phase ownership, helper confinement, evidence freshness, and required verification.

### Task 1.2: Rewrite only the grounded source passages
Paths: ["templates/partials/review-remediation-autonomy.md", "templates/docs/workflow.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl"]

Restructure each grounded candidate in its existing canonical owner. Give each independent rule a short readable unit; visibly separate flexible implementation details from protected constraints; state bounded stop conditions without making a brainstorming checkpoint read like an authority-conflict stop; and put required evidence beside the action it proves. Prefer the established core vocabulary where it is accurate, while retaining distinct technical terms such as execution mode, status, severity, rank, and runtime where they drive behavior. Use canonical includes and links instead of restating shared protocol.

Preserve the review spine's classification ownership and autonomous remediation, the default workflow's independent triggers and material-decision boundary, all plan-v2 grammar and helper confinement, parent ownership of inline transactions, exact phase-review freshness invalidators, report-only assurance, exact review ranges, audit coverage, and the single conditional verify pass. Do not edit `templates/partials/implementation-autonomy.md` or `templates/docs/agents-md-standard.md.tmpl`; the grounded inventory found them already clear and semantically distinct.

### Task 1.3: Render and inspect every affected projection
Kind: batch
Paths: [".awf/awf.lock", "pathspec:.pi/skills/awf-*/SKILL.md", "pathspec:.claude/skills/awf-*/SKILL.md", "pathspec:.pi/agents/*.md", "pathspec:.claude/agents/*.md", "docs/workflow.md"]
Representative: Compare the rendered Pi and Claude writing-plans, executing-plans, and reviewing-impl skills at the rewritten boundaries.
Edge: Extend `TestAuthorityGuidedReviewRemediation` with explicit Core and Full render variants, then use them to produce and assert both review-remediation branches; inspect the named Pi and Claude skill boundaries from those projections. Confirm the repository workflow override remains authoritative when the default template chain does not render locally.
Post-check: Run `./x render`, `./x check`, and `go test ./internal/project ./internal/evals -run 'TestAuthorityGuidedReviewRemediation|TestPlanTaskDetailModesStayAligned|TestPlanningVerificationGuidanceStayAligned|TestPhaseTransactionOwnershipAcrossWorkflowSurfaces|TestFreshPhaseAssuranceReuseContract|TestIndependentWorkflowEscalation'`. Treat canonical consumer and preserved-clause assertions as authority checks, and render/drift plus independently readable separation as state checks. Inspect every manifest-proven affected consumer, expanding the resolved affected population before settlement when the manifest identifies one outside the initial Paths. Record the exact Core and Full, Pi and Claude output boundaries inspected; whether existing flexible details, stop conditions, and required evidence are visibly separated where applicable; whether configured and empty-data rendering remain token-free; and whether any contradictory fragment, lost nuance, duplicate semantic home, or unintended generated path remains. The expected terminal set is no unexplained drift or semantic discrepancy.

### Phase close

Land the semantics-preserving AF-010 source, contract-test, and generated-output transaction.

```commit
docs(rendering): clarify operative execution rules
```

## Definition of done

- `dod: clear-operative-rules` Every changed operative rule is independently comprehensible; existing flexible details, bounded stop conditions, and required evidence are visibly separated or adjacent where the preserved semantics contain them.
- `dod: preserved-contract` Focused tests and semantic review prove that no authority, safety, compatibility, ownership, lifecycle, review, or verification nuance changed and no AF-012 or AF-014B work entered scope.
- `dod: rendered-equivalence` All affected Pi, Claude, Core, and Full projections render cleanly, preserve equivalent applicable semantics, contain no unresolved token or contradictory parallel rule, and pass the full repository gate.

## Notes

The grounded inventory classified `templates/partials/implementation-autonomy.md` and `templates/docs/agents-md-standard.md.tmpl` as no-change surfaces because their operative units are already separated and their distinct terminology drives behavior. No ADR is required unless execution exposes a change to canonical ownership, an active project rule, a material stop or approval boundary, required verification strength, safety, compatibility, or material scope.

Plan review disposition: applied the reasoned recommendation to name the existing profile render fixture because reproducible Core and Full evidence is necessary for semantic review. Applied the reasoned scope correction so the four-part presentation is required only where those semantic parts exist, avoiding invented categories while preserving the audit acceptance boundary. Mechanical corrections restored the integration-ready stop, classified checks, removed generic phase-owned gate protocol, and made the affected generated population exhaustive.

Implementation evidence: `TestAuthorityGuidedReviewRemediation` renders explicit Core and Full configured and empty-data variants. Focused inspection covered the rule, flexible-detail, stop, and evidence boundaries in Pi and Claude reviewing-adr, reviewing-plan, and reviewing-impl; the task, flexible-detail, stop, and scope-evidence boundaries in both writing-plans projections; and the parent rule, helper/runtime detail, required evidence, and freshness boundaries in both executing-plans projections. All preserved authority and verification clauses remained present, both runtimes retained equivalent applicable meaning, empty-data renders stayed token-free, the repository workflow override remained unchanged, and no contradictory fragment, duplicate semantic home, unintended generated path, or lost nuance was found. `changelog/CHANGELOG.md` was added as the authority-determined documentation-currency path omitted from the initial route.

Phase review disposition: restored five mechanically omitted settled clauses for newly warranted support activation, categorical approval evidence, the canonical review user-decision predicate, whole-new-file task ownership, and uncertain-hook verification. These corrections restore the approved semantics rather than create a user decision, so the reviewer's `user-decision` classification was rejected under the authority-guided remediation boundary. Temporary mutations of each restored clause made its focused contract test fail for the intended missing-clause reason, then the sources were restored; no mutation survived.
