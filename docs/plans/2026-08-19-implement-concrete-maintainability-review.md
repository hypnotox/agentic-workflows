---
format: plan-v2
date: 2026-08-19
adrs: [make-review-enforce-concrete-maintainability-risks]
status: Proposed
---
# Plan: Implement Concrete Maintainability Review

## Goal

Make implementation and plan review act only on concrete maintainability risk while preserving the
existing six-field finding model, classification autonomy, material-decision routing, report-only
reviewers, and one verify pass. Do not change ADR review or introduce AF-013 severity routing.

## Architecture summary

Add one heading-free shared partial as the operative semantic home for maintainability-review
admissibility and evidence, while `docs/maintainable-code-design.md` remains the doctrine owner.
Project the partial into code review, plan review, and their dispatching skills. An actionable
maintainability finding maps its affected location to `location`, its semantic owner and concrete risk
to `issue`, its smallest clean remediation to `suggested_fix`, and remediation ownership to the
existing `classification`; a risk-free aesthetic preference stays out of the actionable digest and is
rejected defensively by a dispatcher rather than acquiring a new disposition. Competing clean local
options remain delegated detail, while a new material choice returns to brainstorming independently
of severity. One subagent-driven phase establishes the deterministic oracles, lands the contract and
consumers, applies all three ADR claim operations atomically, renders Pi and Claude outputs, and
records adopter-visible behavior. ADR review and AF-013 remain outside scope; no enabling production
refactor or residual issue-local debt is expected.

**Plan flexibility.**

The protected-contract rule in the workflow document governs what a plan may not change. The plan
records the best known route at authoring time, not a binding implementation choreography. A
commit-capable owner may merge, split, reorder, add, remove, or replace recorded route detail while
the protected contract holds. A path omitted from the plan is not alone a reason to stop, and a stale
listed path need not be touched. Reapproval is required only when the protected contract would change
or an unresolved material decision appears.

Reconcile a Proposed plan only when another phase or reviewer could rely on stale material
instructions. Inconsequential and independently local edits require no deviation record. A delegated
owner reports material cross-owner revisions for parent reconciliation. A helper remains confined to
its assigned paths and gains no scope, commit, review, checkpoint, handoff, or outcome authority from
route flexibility.

## Phase 1: Land the concrete-risk review contract

**Execution mode: subagent-driven.**

Completes: ["concrete-risk-review-operative", "concrete-risk-review-proven"]

### Task 1.1: Establish contract and behavioral regression oracles
Kind: batch
Applying: ["make-review-enforce-concrete-maintainability-risks:deterministic-contract-proof"]
Paths: ["internal/project/spine_test.go", "internal/project/target_test.go", "internal/evals/chain_test.go", "internal/evals/concrete_maintainability_review_test.go"]
Representative: "A second independently changeable policy owner creates future divergence, so review emits an actionable finding whose issue names that owner and risk and whose suggested fix names the smallest clean consolidation."
Edge: "A reviewer prefers a different local helper shape but cannot name concrete harm, so no actionable finding enters the digest; two clean local remedies with the same protected outcome remain delegated detail."
Post-check: "Before operative templates change, `go test ./internal/project ./internal/evals -run 'TestConcreteMaintainabilityReview|TestMaintainableCodeReviewLenses|TestReviewingSkillAgentContracts'` exits nonzero because the shared contract, consumer projection, evidence mapping, dispatcher defense, and outcome scenarios are absent; the failure names those contract gaps rather than an unrelated compile or environment error."

Add a marker-backed project test that reads the new shared partial, rejects headings and duplicate
authored policy, requires exactly one include in the code reviewer, plan reviewer, reviewing-impl, and
reviewing-plan templates, and proves configured plus empty-data rendering without unresolved tokens
or `<no value>`. Extend the existing review-lens and target parity assertions where their current
invariant markers own the changed contract. Prove equivalent applicable semantics across Core and
Full and Pi and Claude without pulling Full-only plan machinery into Core.

Add deterministic outcome evaluations for dual ownership, duplicated policy, dependency inversion,
representation leakage, wrong-model workarounds, unbounded debt, and reduced verification strength.
Each accepted finding must map location, semantic owner plus concrete risk, smallest clean remediation,
and classification through the unchanged schema. Add negative and routing scenarios proving an
aesthetic or pattern preference without concrete risk does not demand remediation, competing clean
local options remain autonomous, and a protected-contract change returns to the material-decision
boundary. Mutation-check the governing clauses so shared labels cannot masquerade as scenario proof.

### Task 1.2: Project the shared threshold through review producers and dispatchers
Kind: batch
Applying: ["make-review-enforce-concrete-maintainability-risks:concrete-risk-admissibility", "make-review-enforce-concrete-maintainability-risks:finding-evidence-mapping", "make-review-enforce-concrete-maintainability-risks:risk-grounded-remediation", "make-review-enforce-concrete-maintainability-risks:single-operative-contract", "make-review-enforce-concrete-maintainability-risks:preserve-review-boundaries"]
Paths: ["templates/partials/review-maintainability-risk.md", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/agents/code-reviewer.md.tmpl", "templates/agents/plan-reviewer.md.tmpl"]
Representative: "The reviewer admits a duplicated-policy finding only after naming the semantic owner, divergence risk, affected location, and smallest consolidation; the dispatcher validates that evidence and applies the classified correction."
Edge: "The concern states only that another pattern is cleaner, so the reviewer omits it from the actionable digest and the dispatcher defense refuses to turn it into required remediation."
Post-check: "`go test ./internal/project -run 'TestConcreteMaintainabilityReview|TestMaintainableCodeReviewLenses'` exits zero; the contract test reports one authored operative home, exactly four approved consumers, coherent configured and empty-data rendering, unchanged six-field schema and informational severity, and no ADR-review or severity-routing consumer."

Create the heading-free shared partial with the concrete-risk admission threshold, explicit evidence
mapping, semantic-owner definition, risk examples, risk-free preference exclusion, dispatcher
defense, autonomous clean-option rule, and independent material-decision route. Keep it operative and
point to the maintainable-code-design guide rather than restating that doctrine.

Include it once in each approved reviewer and dispatching skill. Replace the two reviewer clauses that
currently disclaim AF-005 blocking policy with the shared threshold while retaining their
clean-integration lenses. Make the dispatchers validate maintainability findings before classification
routing, reject a risk-free preference as a reviewer-contract violation rather than a new severity or
classification disposition, and route a genuinely new material choice through brainstorming
independently of severity. Preserve the shared review spine, report-only boundary, consensus handling,
mechanical/reasoned autonomy, user-decision authority test, and exactly one verify pass. Do not change
ADR reviewer or reviewing-adr templates.

### Task 1.3: Apply authority, render projections, and record adopter behavior
Kind: batch
Applying: ["make-review-enforce-concrete-maintainability-risks:concrete-risk-admissibility", "make-review-enforce-concrete-maintainability-risks:finding-evidence-mapping", "make-review-enforce-concrete-maintainability-risks:risk-grounded-remediation", "make-review-enforce-concrete-maintainability-risks:single-operative-contract", "make-review-enforce-concrete-maintainability-risks:preserve-review-boundaries", "make-review-enforce-concrete-maintainability-risks:deterministic-contract-proof"]
Paths: ["docs/decisions/make-review-enforce-concrete-maintainability-risks.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/topics/rendering/workflow-skill-templates.md", "changelog/CHANGELOG.md", ".awf/awf.lock", ".pi/skills/awf-reviewing-impl/SKILL.md", ".pi/skills/awf-reviewing-plan/SKILL.md", ".pi/agents/code-reviewer.md", ".pi/agents/plan-reviewer.md", ".claude/skills/awf-reviewing-impl/SKILL.md", ".claude/skills/awf-reviewing-plan/SKILL.md", ".claude/agents/code-reviewer.md", ".claude/agents/plan-reviewer.md"]
Post-check: "After `./x render`, `./x check` exits zero; `./awf context --show pending docs/decisions/make-review-enforce-concrete-maintainability-risks.md` reports no Remaining operation from the phase batch; focused project and eval tests exit zero; generated Core and Full and Pi and Claude projections carry equivalent applicable semantics without `<no value>`; ADR review remains unchanged; and focused meaning review finds no parallel risk threshold, actionable aesthetic preference, or AF-013 severity routing."

Use `awf-adr-lifecycle` to transition the reviewed ADR to Implementing and append one Applied batch
containing the add of `concrete-maintainability-review` and the updates of
`maintainable-code-review-lenses` and `authority-guided-review-remediation`. Author the new invariant
claim with test backing and precise Origin, revise the two existing claims with final reviewer and
dispatcher semantics, and keep ADR review explicitly outside the reviewer-lens claim's new threshold.

Add an Unreleased feature entry. Run `./x render` so the decision index, current-state projection,
lock, Pi and Claude skills, and Pi and Claude agents move with authored sources. Inspect generated
code and plan reviewers plus both dispatching skills at their applicable Core and Full output
boundaries; record concept-preserving field mapping and routing, intentional profile differences,
absence of contradictory fragments, and absence of AF-013 severity routing in phase evidence. Run
focused tests, `./awf check staged`, and the full gate before phase close.

### Phase close

Land regression oracles, the shared operative threshold, approved consumers, all three atomic claim
mutations, generated projections, ADR application history, and changelog as one independently green
transaction.

```commit
feat(rendering): enforce concrete-risk review (applies ADR batch)
```

## Definition of done

- `dod: concrete-risk-review-operative` Implementation and plan reviewers admit only maintainability findings that name semantic owner, affected location, concrete risk, smallest clean remediation, and classification; their dispatchers reject risk-free preferences, preserve clean local autonomy, and route material boundary changes through brainstorming without changing severity policy or ADR review.
- `dod: concrete-risk-review-proven` Single-home, empty-data, schema, generated-output, cross-profile, cross-runtime, and outcome-oriented scenarios prove concrete risks block, aesthetic preference does not demand remediation, competing clean options remain delegated, and protected-contract changes retain the material-decision boundary.

## Notes

Apply the plan-flexibility rule above when recording deviations. Delegated owners report material
cross-owner revisions rather than editing the plan; the parent supplies the report to phase review
and reconciles required plan changes with findings in one focused post-review settlement commit before
checkpointing or later execution. After implementation assurance settles, `awf-effort-workflow`
reconciles those records, appends only the ADR's Implemented event, changes this plan to
`status: Implemented`, renders the decision index and lock, and commits that lifecycle-only
transaction before managed-topology removal. The AF-005 completion report follows integration and
terminal artifact closure so its recorded range and verification evidence are final.
