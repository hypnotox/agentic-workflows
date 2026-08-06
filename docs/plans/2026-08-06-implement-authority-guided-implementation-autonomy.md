---
format: plan-v2
date: 2026-08-06
adrs: [authority-guided-implementation-autonomy]
status: Proposed
---
# Plan: Implement Authority-Guided Implementation Autonomy

## Goal

Render one authority-guided implementation-autonomy boundary across every supported implementation path, keep mutable plans truthful under inline and delegated ownership, and resolve authority-determined review and verification findings without unnecessary user approval. Do not add a policy schema, classifier, deviation ledger, command, linter, workflow stage, or weaker verification path.

## Architecture summary

One shared prose partial owns the autonomy and escalation semantics and is included directly by each implementation consumer. Plan-specific templates add inline amendment and delegated report-review-settlement choreography around that common boundary. Existing rendering tests prove direct inclusion, semantic projection, coherent empty-data output, phase ordering, and plan-convention alignment. One coherent implementation transaction applies all four ADR operations because the partial, consumers, claims, lifecycle event, tests, and generated outputs form one indivisible workflow contract.

Before dispatch, the parent establishes a known clean and green baseline where `git status --short` prints no output, `./x check` reports clean, and `./x gate` passes. The phase owner receives that verified baseline rather than owning this pre-dispatch action.

Every repository path below is relative to the managed worktree `/home/hypno/Projects/agentic-workflows/.awf/worktrees/implementation-autonomy`.

## Phase 1: Render and apply authority-guided implementation autonomy

**Execution mode: subagent-driven.**
Completes: ["autonomy-policy", "truthful-plan-reconciliation", "authority-guided-review", "authority-backed", "render-green"]

### Task 1.1: Pin the shared autonomy contract before implementation
Latitude: exact
Applying: ["authority-guided-implementation-autonomy:authority-guided-autonomy", "authority-guided-implementation-autonomy:narrow-escalation-boundary", "authority-guided-implementation-autonomy:reasoned-detail-deviation", "authority-guided-implementation-autonomy:issue-resolution-before-escalation", "authority-guided-implementation-autonomy:judgment-without-policy-schema"]

In `internal/project/spine_test.go`, add `TestAuthorityGuidedImplementationAutonomy` with the proof marker `// invariant: rendering/workflow-skill-templates:authority-guided-implementation-autonomy (TestAuthorityGuidedImplementationAutonomy)`. Make the test fail before production prose changes. It must read the embedded source for each direct consumer and prove exactly one `<!-- awf:include implementation-autonomy -->` in:

- `templates/agents/implementer.md.tmpl`;
- `templates/skills/executing-direct/SKILL.md.tmpl`;
- `templates/skills/bugfix/SKILL.md.tmpl`;
- `templates/skills/tdd/SKILL.md.tmpl`;
- `templates/skills/executing-plans/SKILL.md.tmpl`;
- `templates/skills/subagent-driven-development/SKILL.md.tmpl`; and
- `templates/skills/reviewing-impl/SKILL.md.tmpl`.

Render the agent and all six skills with configured representative data and with empty optional data. Assert each expanded output is coherent, has no unresolved no-value token, and carries the shared semantics: reasoned authority-preserving correction is autonomous; ADRs, current-state claims, the approved outcome, material scope, and settled durable boundaries remain binding; discovery of a source contradiction, correctness or safety concern, review finding, blocker symptom, or failed check triggers diagnosis rather than automatic approval; and escalation is limited to authority conflict or required authority change, material outcome or scope change, a genuine unresolved design fork, inability to finish safely or correctly inside the boundary, or required verification still unreachable after reasonable remediation. Assert the obsolete rule that every effect on behavior, scope, structure, dependencies, patterns, checks, or testing strategy stops before further mutation is absent from these implementation consumers.

Extend `TestImplementerAgent` to require a completed-report deviation inventory where `None` is valid and every actual deviation names the change, rationale, governing authority, and verification. Replace its approval-requiring invalidating-source stopped assertions with the narrow authority/scope/ambiguity/safety/verification boundary while retaining the closed completed-or-stopped outcome, dirty-state inventory, actual failing-check output when applicable, and prohibition on weakening checks.

Update `TestMaintainableCodeStageCoverage` so TDD, direct execution, bug fixing, inline plan execution, and subagent-driven execution retain simplest-sufficient and structural-boundary duties but expect the new shared autonomy semantics instead of the broad material-category stop. Run `go test ./internal/project -run 'TestAuthorityGuidedImplementationAutonomy|TestImplementerAgent|TestMaintainableCodeStageCoverage'`; before implementation it fails for the missing partial/includes and old stop contract, and after Tasks 1.2-1.3 it passes.

Add the same `authority-guided-implementation-autonomy` invariant proof marker, naming each test's own unit, to `TestPhaseTransactionOwnershipAcrossWorkflowSurfaces`, `TestPlanDeviationReconciliationGuidanceStayAligned`, `TestCheckpointDigestShape`, and `TestConditionalVerifyPass`. Together these markers back the new claim's direct projection, plan ownership and ordering, checkpoint remediation boundary, and implementation-review residual routing; no single test is claimed to prove clauses it does not exercise.

### Task 1.2: Project the shared policy through implementation owners
Latitude: exact
Applying: ["authority-guided-implementation-autonomy:authority-guided-autonomy", "authority-guided-implementation-autonomy:narrow-escalation-boundary", "authority-guided-implementation-autonomy:reasoned-detail-deviation", "authority-guided-implementation-autonomy:issue-resolution-before-escalation", "authority-guided-implementation-autonomy:judgment-without-policy-schema"]

Create `templates/partials/implementation-autonomy.md` as the only full semantic statement of the common implementation boundary. Keep it target-neutral, variable-free, publication-safe, and suitable for both an interactive parent and a no-user implementation child. It must require autonomous diagnosis and reasoned correction inside existing authority and approved boundaries, state the narrow escalation conditions from the ADR, forbid weakened oracles and unrelated cleanup, and require a rationale plus verification for a non-mechanical deviation. Phrase escalation as stopping and reporting through the active workflow rather than asking an unreachable user.

Replace the duplicated broad-stop paragraphs in `templates/agents/implementer.md.tmpl`, `templates/skills/executing-direct/SKILL.md.tmpl`, `templates/skills/bugfix/SKILL.md.tmpl`, `templates/skills/tdd/SKILL.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`, and `templates/skills/subagent-driven-development/SKILL.md.tmpl` with one direct include marker per consumer. Add the same direct include once to `templates/skills/reviewing-impl/SKILL.md.tmpl`; Task 1.3 aligns its stage-specific remediation behavior. Retain each consumer's stage-specific duties: complete dispatched scope and no unrelated cleanup for the implementer; return to brainstorming for an actual new load-bearing choice in direct execution; root-cause and red-green discipline in bugfix/TDD; settled structural choices, complete phase ownership, and safe dirty-state recovery in plan execution.

In the implementer return schema, require `completed` to report `deviations: none` or enumerate each deviation with changed detail, rationale, governing authority, and verification. Reserve `stopped` for a required check that remains unresolved inside scope or a complete report showing which narrow escalation boundary was reached; keep status, completed work, remaining work, actual failure output where present, and attempts. A reasoned plan-detail deviation that reaches green is a completed outcome, never a third outcome and never an approval-requiring stop.

In `templates/skills/executing-plans/SKILL.md.tmpl`, replace the unconditional pre-edit user stop for missing phase details with authority-guided resolution: when repository authority determines a missing or stale path, check, closing subject, or local instruction, the inline parent amends the mutable plan immediately, records the reasoned deviation in Notes, and continues. A genuine unresolved design fork uses the shared escalation boundary. Run `go test ./internal/project -run 'TestAuthorityGuidedImplementationAutonomy|TestImplementerAgent|TestMaintainableCodeStageCoverage'`; it passes.

### Task 1.3: Align delegated reconciliation, review remediation, plans, and checkpoints
Latitude: exact
Applying: ["authority-guided-implementation-autonomy:truthful-plan-reconciliation", "authority-guided-implementation-autonomy:issue-resolution-before-escalation", "authority-guided-implementation-autonomy:narrow-escalation-boundary"]

In `templates/skills/subagent-driven-development/SKILL.md.tmpl`, require the completed child report to be inventoried for deviations. Supply that report verbatim to report-only phase review. After review, the parent lands one focused settlement commit containing the plan Notes reconciliation and any mechanical or reasoned review fixes before checkpointing or dispatching later execution. The parent does not rewrite the child's phase-closing commit. A no-deviation, no-finding phase needs no empty settlement commit. Preserve clean-tip validation, one delegated phase-closing commit, sequential dispatch, and explicit dirty-return recovery; user input remains only when no safe authority-preserving recovery exists.

In `internal/project/phase_transaction_ownership_test.go`, extend `TestPhaseTransactionOwnershipAcrossWorkflowSurfaces` to prove the delegated order: completed deviation report, report-only phase review, focused parent settlement commit when needed, plan reconciliation, then routine checkpoint. Prove later phase execution cannot precede reconciliation and that the child still owns exactly one phase-closing commit. Keep configured and empty-data variants.

Align plan mutability guidance in `templates/skills/writing-plans/SKILL.md.tmpl`, `templates/plans-readme/README.md.tmpl`, and `templates/plans-template/template.md.tmpl`: inline owners immediately correct stale instructions and record reasoned deviations in Notes; delegated owners may report rather than edit; the parent provides the report to phase review and reconciles the plan plus findings in a focused post-review settlement commit before checkpoint or later execution. In `internal/project/plan_detail_modes_test.go`, add `TestPlanDeviationReconciliationGuidanceStayAligned` over the default writer, README, scaffold, root rendered outputs, and both runtime writing skills. Assert the ownership distinction and ordering without freezing an entire paragraph.

In `templates/skills/reviewing-impl/SKILL.md.tmpl`, keep finding routing based on classification rather than severity. Audit errors and residual verify findings are diagnosed and resolved autonomously when authority determines a mechanical or reasoned remedy; classify and stop as `user-decision` only at the shared narrow boundary. Do not add an unbounded review loop: after the existing single verify pass, apply any authority-determined residual fix, run the gate and audit again, report the final disposition, and request user input only for a true narrow-boundary finding.

Update `templates/partials/checkpoint-routine.md` without including another partial: preserve its four-step general checkpoint protocol, but clarify that a correctness or safety concern, blocker, or failed required verification needs user attention only when it remains unresolved after the active workflow's required diagnosis and authority-guided remediation. Update the matching working-memory prose in `templates/docs/workflow.md.tmpl`. In `templates/agents-doc/AGENTS.md.tmpl`, add one concise global pointer requiring implementation agents to use their selected workflow's shared authority boundary for autonomous detail resolution; do not duplicate the partial's full classification. Do not loosen effort creation, grounded-design approval, settled-ADR approval, non-implementation workflow authority, or checkpoint persistence.

Extend `internal/project/spine_test.go` checkpoint and conditional-verify coverage to assert the narrowed unresolved condition, classification-based residual routing, single verify-pass limit, and the unchanged four-step checkpoint shape. Keep `internal/evals/chain_test.go`'s three mandatory approval boundaries green without adding another final approval stop. Run `go test ./internal/project ./internal/evals`; it passes.

### Task 1.4: Apply current authority, render outputs, and close green
Kind: batch
Latitude: exact
Applying: ["authority-guided-implementation-autonomy:authority-guided-autonomy", "authority-guided-implementation-autonomy:narrow-escalation-boundary", "authority-guided-implementation-autonomy:reasoned-detail-deviation", "authority-guided-implementation-autonomy:truthful-plan-reconciliation", "authority-guided-implementation-autonomy:issue-resolution-before-escalation", "authority-guided-implementation-autonomy:judgment-without-policy-schema"]
Paths: ["templates/partials/implementation-autonomy.md", "templates/partials/checkpoint-routine.md", "templates/agents/implementer.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/skills/executing-direct/SKILL.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "templates/skills/tdd/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/docs/workflow.md.tmpl", "internal/project/spine_test.go", "internal/project/phase_transaction_ownership_test.go", "internal/project/plan_detail_modes_test.go", "internal/evals/chain_test.go", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "changelog/CHANGELOG.md", "docs/decisions/authority-guided-implementation-autonomy.md", ".awf/awf.lock", "AGENTS.md", "docs/decisions/INDEX.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/workflow.md", "docs/plans/README.md", "docs/plans/template.md", "glob:.pi/agents/*.md", "glob:.claude/agents/*.md", "glob:.pi/skills/awf-*/SKILL.md", "glob:.claude/skills/awf-*/SKILL.md", "glob:examples/*/.awf/awf.lock", "glob:examples/*/AGENTS.md", "glob:examples/*/docs/workflow.md", "glob:examples/*/docs/plans/*.md", "glob:examples/*/.pi/agents/*.md", "glob:examples/*/.claude/agents/*.md", "glob:examples/*/.pi/skills/*/SKILL.md", "glob:examples/*/.claude/skills/*/SKILL.md"]
Representative: `templates/partials/implementation-autonomy.md` expands once into the root Pi and Claude implementer and execution/review skills, while the plan convention templates render the inline and delegated reconciliation distinction into their root documentation outputs.
Edge: Empty optional template data renders the same authority boundary without an unresolved token; checkpoint consumers outside implementation retain their general authority semantics; an implementer with a reasoned green deviation returns completed while a persistently failing required check or genuine authority conflict returns stopped.
Post-check: After `./x render`, run `git diff --check`; run `rg -n 'If a newly discovered need affects behavior, scope, structure, dependencies, patterns, checks, or testing strategy|complete approval-requiring invalidating-source report' templates/agents/implementer.md.tmpl templates/skills/executing-direct/SKILL.md.tmpl templates/skills/bugfix/SKILL.md.tmpl templates/skills/tdd/SKILL.md.tmpl templates/skills/executing-plans/SKILL.md.tmpl templates/skills/subagent-driven-development/SKILL.md.tmpl .pi/agents/implementer.md .claude/agents/implementer.md .pi/skills/awf-{executing-direct,bugfix,tdd,executing-plans,subagent-driven-development}/SKILL.md .claude/skills/awf-{executing-direct,bugfix,tdd,executing-plans,subagent-driven-development}/SKILL.md` and require no matches with successful file traversal; inspect `git diff --name-only` and confirm every changed generated output is attributable to a named source or current-state mutation and every changed path falls within the declared Paths population.

In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, add `invariant: authority-guided-implementation-autonomy` with `Origin: ADR-authority-guided-implementation-autonomy`, `Backing: test`, and the five exact proof markers assigned in Tasks 1.1 and 1.3. The claim owns direct single-partial projection across all named implementation consumers; authority/outcome/material-scope/durable-boundary preservation; autonomous reasoned correction and issue remediation; narrow escalation; inline Notes amendment; delegated deviation reporting, review input, and focused post-review parent reconciliation; classification-based implementation-review remediation; no weakened oracle; and coherent empty-data rendering.

Update these existing claims while preserving Origin and prior Revised-by order, appending `ADR-authority-guided-implementation-autonomy`, and retaining `Backing: test`:

- `mandatory-approval-boundaries`: retain only the three mandatory approval boundaries and the proportionate grounded-design contract; remove the broad implementation-category stop because the new autonomy claim now owns the implementation boundary.
- `maintainable-code-subagent-contract`: permit reasoned authority-preserving detail deviations inside complete scope, require their structured completed report, and retain no replanning, material broadening, unrelated cleanup, or memory editing.
- `implementer-role-contract`: require the completed deviation inventory and reserve stopped for persistently unreachable verification or a narrow-boundary report while retaining the closed two-outcome schema and complete dirty-state inventory.

Update `changelog/CHANGELOG.md` under Unreleased Features with one concise adopter-facing entry: implementation paths now resolve authority-preserving reasoned deviations and review findings autonomously, inline owners amend mutable plans, delegated owners report deviations for post-review parent reconciliation, and material authority/outcome/scope or persistent safety/verification boundaries still escalate.

Transition `docs/decisions/authority-guided-implementation-autonomy.md` from Proposed to Implementing in the same transaction as all four claim mutations. Change frontmatter status, append an Implementing event dated on execution day with the current content digest, then append one Applied event listing exactly the add and three update operations declared by the ADR. Obtain the digest with the governed placeholder workflow: insert 64 zeros, run `./x check`, copy the reported computed digest exactly, replace the placeholder, and rerun until clean; do not precompute or guess it.

Run `./x render`. Inspect representative root Pi and Claude implementer, execution, implementation-review, agent-guide, workflow, plan README, and plan template outputs for contradictory old/new fragments, concept-preserving paraphrase, intentional placeholder syntax, duplicate policy text, unresolved no-value tokens, and project-specific leakage. Confirm literals such as `<slug>` and `<completed phase>` remain intentional generic placeholders rather than unresolved template values. Inspect any active example outputs changed by rendering with the same semantic lens.

Run `go test ./internal/project ./internal/evals`, `./x check`, and `./x gate`; each reaches a clean/pass terminal state.

### Phase close

Return every implementation deviation in the completed child report and do not edit this plan. Leave plan frontmatter `status: Proposed`; the parent gives the report to phase review, then owns any focused Notes-and-findings settlement commit, while terminal implementation review owns the later plan and ADR `Implemented` flips. Stage the complete child transaction explicitly, including the new partial, consumer templates, tests, current-state part, ADR lifecycle changes, changelog, lock, and every rendered output, but excluding this parent-owned plan. Run `./awf check staged` and `./x gate`; both pass. Create the one phase-closing commit:

```commit
feat(rendering): add authority-guided implementation autonomy
```

## Definition of done

- `dod: autonomy-policy` Every supported implementation owner renders the same direct-inclusion authority boundary and autonomously completes reasoned deviations that preserve authority, approved outcome, material scope, settled durable boundaries, and verification.
- `dod: truthful-plan-reconciliation` Inline owners immediately amend plan instructions and Notes; delegated owners report deviations to phase review and the parent reconciles them with findings in a focused settlement commit before checkpoint or later execution.
- `dod: authority-guided-review` Implementation review routes findings by classification rather than severity and autonomously remediates authority-determined mechanical and reasoned issues without adding an unbounded review loop.
- `dod: authority-backed` The ADR is Implementing with all four operations Applied atomically, and the new and revised current-state claims are backed by focused rendering tests.
- `dod: render-green` Root and active example outputs are semantically coherent, generated without drift or unresolved tokens, and the focused tests, project check, and full gate pass.

## Notes

Record inline deviations immediately. For a delegated phase, preserve the child report as phase-review input and reconcile reported deviations plus review findings in one focused parent settlement commit before checkpointing or later execution. The settled design prohibits a policy schema, classifier, deviation ledger, command, linter, new workflow stage, weakened oracle, unrelated cleanup, or child-owned working memory.

Phase-review settlement: the child reported no implementation deviations. Report-only phase review found five incomplete invariant proofs and one classification-order defect in implementation review. The parent expanded configured and empty-data policy coverage, delegated ordering, plan-surface alignment, checkpoint remediation, and residual-review assertions, then corrected implementation review to diagnose before final classification and stop on findings that remain `user-decision`.

Terminal-review settlement: implementation review found three remaining proof gaps, one structural rendering defect, and one under-specific settlement subject. The parent strengthened the invariant proofs and replaced the shared partial's heading with a non-structural label so direct inclusion preserves consumer sections and numbered procedures. The single verify pass found one residual stopped-schema proof ambiguity, resolved mechanically by scoping its assertions to the return schema. The earlier settlement commit remains immutable under the no-amend rule; its body already describes the production routing correction. Both audits reported no errors, the repo-local audit stayed clean, and the gate passed after each remediation. No implementation deviation changed the approved outcome, scope, authority, or verification boundary. Integration numbered the decision as ADR-0240 and preserved the reviewed public shape.
