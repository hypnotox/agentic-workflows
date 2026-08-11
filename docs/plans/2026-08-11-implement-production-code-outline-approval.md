---
format: plan-v2
date: 2026-08-11
adrs: [user-approved-production-code-implementation-outlines]
status: Implemented
---
# Plan: Implement Production-Code Outline Approval

## Goal

Require one proportionate, user-approved implementation outline before every hand-authored production-code change, then keep ADRs, plans, implementation, and review autonomous inside that boundary. Preserve the ADR's nonproduction exclusions and add no runtime classifier, approval artifact, or second workflow owner.

## Architecture summary

Implement the linked ADR in two independently green subagent-driven transactions. First make brainstorming the sole outline owner, project one variable-free approval-evidence boundary into every production-code entry path, preserve the named nonproduction exclusions, and apply the independent-intake claim. Then remove settled-ADR approval as a routine stop, route later material choices back through the pre-artifact outline boundary, update the remaining approval and review claims, and render all targets. Tests pin behavior before each source change; templates remain the authoring authority, generated Pi and Claude outputs are inspected rather than hand-edited, and each current-state claim mutation travels with its matching ADR lifecycle event.

## Phase 1: Require approved outlines at production-code intake

**Execution mode: subagent-driven.**

Completes: ["outline-intake", "approval-evidence"]

### Task 1.1: Pin the outline boundary and evidence forms before source changes
Applying: ["user-approved-production-code-implementation-outlines:approve-production-code-outline", "user-approved-production-code-implementation-outlines:preserve-nonproduction-autonomy", "user-approved-production-code-implementation-outlines:keep-brainstorming-as-single-owner", "user-approved-production-code-implementation-outlines:retain-outline-approval-evidence"]
Paths: ["internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go"]

Start from the accepted ADR, the approved effort Decision-log evidence, and the clean phase baseline. Extend the catalog-rendered contract tests first. Prove that brainstorming owns both concise outlines and fuller material-choice design; hand-authored production changes, mechanical production refactors, and tests preparing a production change cannot mutate before explicit outline approval; documentation-only, test-only maintenance, generated-output-only, and non-code mechanical work remain autonomous unless another independent trigger fires. Prove all accepted evidence forms: retained conversation, effort Decision-log evidence, and an explicit request to execute a named plan whose Architecture summary supplies the outline. Pin direct, TDD, bugfix, inline-plan, delegated-plan, and implementer paths, including the rule that a delegated owner receives the parent's approved boundary and never recreates the approval interaction. Preserve independent continuity, grounding, ADR, plan, and review triggers. Run the focused tests before production edits and record the expected red failures caused by absent outline semantics.

### Task 1.2: Broaden brainstorming and project the intake boundary
Kind: batch
Applying: ["user-approved-production-code-implementation-outlines:approve-production-code-outline", "user-approved-production-code-implementation-outlines:preserve-nonproduction-autonomy", "user-approved-production-code-implementation-outlines:keep-brainstorming-as-single-owner", "user-approved-production-code-implementation-outlines:retain-outline-approval-evidence"]
Paths: ["templates/partials/production-code-outline-approval.md", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/executing-direct/SKILL.md.tmpl", "templates/skills/tdd/SKILL.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/agents/implementer.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/docs/workflow.md.tmpl", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/user-approved-production-code-implementation-outlines.md", "internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go", ".awf/awf.lock", "AGENTS.md", "docs/decisions/INDEX.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/workflow.md", "docs/plans/README.md", "docs/plans/template.md", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md", "glob:.pi/agents/*.md", "glob:.claude/agents/*.md", "glob:examples/*/AGENTS.md", "glob:examples/*/.pi/skills/*/SKILL.md", "glob:examples/*/.claude/skills/*/SKILL.md", "glob:examples/*/.pi/agents/*.md", "glob:examples/*/.claude/agents/*.md", "glob:examples/*/docs/workflow.md", "glob:examples/*/docs/plans/*.md", "glob:examples/*/.awf/awf.lock"]
Representative: `templates/skills/brainstorming/SKILL.md.tmpl` remains the sole workflow owner, while `templates/partials/production-code-outline-approval.md` is only the variable-free projection seam consumed at production mutation boundaries.
Edge: The partial must distinguish a preparatory production test from autonomous test-only maintenance, accept a named plan only when the user's explicit execution request and its Architecture summary are both present, and tell delegated owners to consume the supplied boundary rather than seek user access.
Post-check: Run the focused independent-escalation and maintainable-stage tests; inspect rendered brainstorming, direct, TDD, bugfix, plan-writing, plan-execution, delegated-execution, proposing-ADR, and implementer outputs for both enabled targets; require the four exclusions and three evidence forms to remain coherent with empty optional data; then run `./x render && ./x check` and verify no generated drift remains.

Create the shared projection seam without making it a second workflow owner. Broaden brainstorming's invocation and final presentation so even a straightforward production change receives a proportionate outline, and make approval close the implementation boundary before preparatory tests, ADRs, or plans. Each production-code entry path consumes or routes to that approved boundary before mutation. Keep effort creation independent and optional; effort-backed evidence comes from user-provenance Decision-log entries, effort-free evidence may remain in conversation, a named plan requires an explicit request and its Architecture summary, and delegated owners inherit evidence from the parent brief. Update plan guidance so Architecture summaries are complete enough to serve that accepted reuse path. Update `rendering/workflow-skill-templates:independent-workflow-escalation`, preserve its existing proof marker, append this ADR to `Revised-by`, and transition the ADR from Accepted to Implementing with an Applied event for only that operation. Render and inspect every affected publication while leaving the existing settled-ADR approval stop intact until Phase 2.

### Phase close

Close the production intake, evidence projection, independent claim, ADR lifecycle event, tests, and generated outputs as one green transaction.

```commit
feat(rendering): require outline approval (applies ADR batch)
```

## Phase 2: Make the outline the sole routine design checkpoint

**Execution mode: subagent-driven.**

Completes: ["downstream-autonomy", "publication-contract"]

### Task 2.1: Pin autonomous ADR and downstream review behavior
Applying: ["user-approved-production-code-implementation-outlines:approve-before-artifacts-once", "user-approved-production-code-implementation-outlines:approve-production-code-outline", "user-approved-production-code-implementation-outlines:retain-outline-approval-evidence"]
Paths: ["internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go"]

Start from the green Phase 1 transaction and its rendered evidence boundary. Rewrite the approval-boundary expectations before source edits. Preserve effort first-creation confirmation and the pre-artifact outline approval stop, but require ADR review to proceed directly after settlement without a routine user approval. Prove that proposing ADR, writing and reviewing plans, implementation, and implementation review remain autonomous inside the approved boundary. Route a new material decision or changed approved boundary back through brainstorming; after diagnosis, report an unresolved blocker or safety or correctness concern through the active workflow. Keep review finding classification, consent adherence, surplus-removal disclosure, and one verify pass unchanged. Run the focused mandatory-approval and authority-guided-review tests and record the expected red failures from the surviving settled-ADR stop.

### Task 2.2: Remove downstream approval and apply the remaining claims
Kind: batch
Applying: ["user-approved-production-code-implementation-outlines:approve-before-artifacts-once", "user-approved-production-code-implementation-outlines:keep-brainstorming-as-single-owner", "user-approved-production-code-implementation-outlines:retain-outline-approval-evidence"]
Paths: ["templates/partials/checkpoint-approval.md", "templates/partials/review-remediation-autonomy.md", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/docs/workflow.md.tmpl", ".awf/docs/glossary.yaml", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/user-approved-production-code-implementation-outlines.md", "changelog/CHANGELOG.md", "internal/evals/independent_workflow_escalation_test.go", "internal/project/spine_test.go", ".awf/awf.lock", "docs/decisions/INDEX.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/workflow.md", "docs/glossary.md", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md", "glob:.pi/agents/*.md", "glob:.claude/agents/*.md", "glob:examples/*/.pi/skills/*/SKILL.md", "glob:examples/*/.claude/skills/*/SKILL.md", "glob:examples/*/.pi/agents/*.md", "glob:examples/*/.claude/agents/*.md", "glob:examples/*/docs/workflow.md", "glob:examples/*/.awf/awf.lock"]
Representative: `templates/skills/reviewing-adr/SKILL.md.tmpl` continues directly from settled review to linked-plan handling, and `templates/partials/checkpoint-approval.md` becomes specific to the pre-artifact outline approval still included by brainstorming.
Edge: Removing the routine ADR stop must not remove consent-adherence review, disclosure of removed surplus commitments and semantics-preserving refinements, user-decision routing for changed accepted semantics, or effort checkpoint persistence when a genuine new decision returns through brainstorming.
Post-check: Run the focused mandatory-approval, independent-escalation, review-remediation, reviewing-ADR, and empty-render tests; inspect rendered brainstorming, proposing-ADR, reviewing-ADR, reviewing-plan, and reviewing-impl outputs for both targets as one coherent chain; search tracked source and generated output for obsolete claims that settled ADR review always stops for approval, with a checked success sentinel and only lifecycle history allowed as residue; then run `./x render && ./x check` and require a clean result.

Remove the settled-ADR mandatory approval include and route settled review directly to deterministic linked-plan review or the independently selected next implementation path. Narrow the approval partial and workflow/glossary narrative to effort creation plus the single pre-artifact outline checkpoint. Refine shared review remediation so a new load-bearing decision returns through brainstorming before ADR mutation, while mechanical and authority-preserving reasoned findings remain autonomous. Update `rendering/workflow-skill-templates:mandatory-approval-boundaries` and `rendering/workflow-skill-templates:authority-guided-review-remediation`, preserve their proof markers and provenance order, and append this ADR to each `Revised-by`. Append Applied events for those two distinct claim operations while leaving the ADR Implementing for deferred terminal closure. Add one concise Unreleased changelog entry, render all enabled targets, and record the focused semantic inspection boundaries and result in completion evidence.

### Phase close

Close the single-checkpoint semantics, autonomous review chain, two claim operations, ADR lifecycle events, changelog, tests, and generated outputs as one green transaction.

```commit
feat(rendering): make outline approval the sole checkpoint
```

## Definition of done

- `dod: outline-intake` Every rendered production-code path requires a proportionate approved outline before hand-authored production mutation, including mechanical production refactors and preparatory tests, while the four named nonproduction categories remain autonomous.
- `dod: approval-evidence` Conversation, effort Decision-log evidence, and an explicit named-plan execution request with its Architecture summary are accepted without requiring an effort, and delegated owners consume the parent-supplied boundary without another approval interaction.
- `dod: downstream-autonomy` ADR and plan authoring and review, implementation, and implementation review continue autonomously inside the approved outline; genuine new material decisions or boundary changes return through brainstorming, and settled ADR review is not a routine stop.
- `dod: publication-contract` The three declared claim operations are Applied with matching current-state prose and proof markers, both enabled targets render coherent token-free guidance, the Unreleased changelog documents the behavior, and `./x check` plus the project gate pass.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners may report rather than edit; the parent supplies the report to phase review and reconciles it with findings in one focused post-review settlement commit before checkpointing or later execution. Record deviations, review findings and dispositions, semantic rendering evidence, and any renewed assurance here.

Plan review settlement: one reasoned finding identified authority and verification categories as surplus return conditions and incorrectly routed all persistent concerns through brainstorming. Task 2.1 now reserves brainstorming for a new material decision or boundary change and leaves unresolved blocker and safety or correctness reporting with the active workflow. Mechanical corrections restored exact execution-mode grammar, shortened both closing subjects, and added the slim AGENTS source and generated fan-out to Phase 1.

Phase 1 deviation: the owner added `.awf/parts/workflow/chain.md` because awf's local generated workflow output requires its configured override source rather than a direct standard-template-only edit. The rendered-source ownership rule governed the deviation; render, staged check, and semantic inspection verified it.

Phase 1 review settlement: four mechanical findings were applied in this focused parent transaction. The brainstorming skill description now advertises the mandatory production-code trigger; the shared projection distinguishes interactive routing from a delegated owner's stop-and-report behavior when evidence is missing; the published exclusion sentence is capitalized; and the Unreleased changelog records Phase 1's adopter-facing behavior. Focused tests, render/check, staged check, and the gate verify the settlement.

Phase 2 review settlement: review correctly identified that the three review workflows still bypassed brainstorming for `user-decision` findings, but misclassified the fix as a new user decision. The accepted sole-owner Decision already requires new material decisions and boundary changes to return through brainstorming, so the parent applied that routing as an authority-preserving reasoned correction. Necessary omitted paths `templates/skills/reviewing-plan/SKILL.md.tmpl` and `templates/skills/reviewing-impl/SKILL.md.tmpl` complete the three-workflow projection. Mechanical corrections strengthen the authority-guided-remediation proof against the obsolete direct route and exercise the pre-artifact boundary plus both linked-plan and direct-implementation ADR hand-offs. Because the settlement is reasoned, renewed phase review is required before checkpointing.

Phase 2 renewed-review settlement: two reasoned findings exposed remaining direct user routes outside the sole brainstorming owner. Plan writing now resolves authority-determined local file and phase shape autonomously but routes a new material decision or changed approved boundary through brainstorming. The routine checkpoint now separates that material-choice routing from active-workflow reporting of blockers, failed verification, and safety or correctness concerns that remain unresolved after diagnosis and remediation. Focused source and both-target proofs require each replacement route and reject the three obsolete direct-route phrases. These authority-preserving corrections add `templates/skills/writing-plans/SKILL.md.tmpl` and `templates/partials/checkpoint-routine.md` as necessary omitted paths, render their complete fan-out, and require another renewed combined Phase 2 review.

Phase 2 combined-review settlement: the fresh combined review found four mechanical residuals. Bugfix and TDD now route materially larger-work disposition through brainstorming, with an active-workflow design-discussion fallback when the native skill is unavailable. The workflow document mirrors the checkpoint split between brainstorming-owned material decisions or boundary changes and active-workflow reporting of unresolved blockers, failed verification, and safety or correctness concerns. Source, empty-skill fallback, both-target, workflow-document, positive-route, and obsolete-route assertions cover all corrected surfaces. These necessary omitted paths complete the approved downstream-autonomy projection; because every residual correction is mechanical, the conditional verify-pass rule requires no further same-phase review.

Terminal assurance settlement: full-range review found two mechanical delegated-dispatch proof gaps. Commit-capable phase-owner briefs and commit-disabled helper briefs now explicitly identify the parent-supplied approved boundary, while effort identity remains conditional on an existing effort. Source, empty-skill, Pi, and Claude assertions fail if either dispatch clause is removed; temporary falsification confirmed both negative cases. The fixes preserve the implementer's single evidence-consumption contract without duplicating the approval interaction. Mechanical-only terminal fixes require no verify pass; the complete final range remains subject to the gate and both audits.
