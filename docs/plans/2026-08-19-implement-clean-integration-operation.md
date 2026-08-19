---
format: plan-v2
date: 2026-08-19
adrs: [make-clean-integration-operative]
status: Proposed
---
# Plan: Implement Clean Integration Operation

## Goal

Make every primary design, planning, implementation, and review path apply the one proportional
clean-integration contract from ADR-make-clean-integration-operative. Preserve the canonical
design guide, protected-contract authority, stage-local protocol, YAGNI, and AF-005's separate
review-severity scope.

## Architecture summary

Add one heading-free `clean-integration` partial as the operative semantic home and keep
`maintainable-code-design.md` as the doctrine owner. Project the partial directly into the approved
skill and agent consumers so coverage stays explicit; replace only fragmented operative prompts and
retain each consumer's authority, execution, review, and confinement protocol. Keep the agent-guide
summary slim and clarify the guide/operative-rule relationship without copying the rule into either
surface. One inline phase first establishes contract and behavioral oracles, then lands the shared
partial and consumers, applies the ADR's claim batch, renders both governance footprints for Pi and
Claude, and records the adopter-visible change. The approved boundary includes brainstorming and
plan writing as always-aware consumers, bounded necessary refactoring, practical obsolete-path
retirement, applicable verification surfaces, explicit residual debt, and review lenses that do not
adopt AF-005 severity policy.

## Phase 1: Land the clean-integration contract

**Execution mode: inline.**

Completes: ["clean-integration-operative", "clean-integration-proven"]

### Task 1.1: Establish contract and behavioral regression oracles
Kind: batch
Applying: ["make-clean-integration-operative:operative-consumer-and-proof"]
Paths: ["internal/project/spine_test.go", "internal/project/target_test.go", "internal/project/docs_sections_test.go", "internal/project/render_tree_test.go", "internal/evals/clean_integration_test.go"]
Representative: "A change would duplicate policy across commands, so design and execution place the policy in one owner before adding behavior and review detects any parallel obsolete route."
Edge: "The existing owner already carries the behavior cleanly, so the rule requires no refactor and rejects an attractive unrelated cleanup or test-only production seam."
Post-check: "Before operative templates change, `go test ./internal/project ./internal/evals -run 'TestCleanIntegration|TestMaintainableCodeReviewLenses|TestMaintainableCodeStageCoverage|TestMaintainableCodeSubagentContract'` exits nonzero because the shared home, explicit consumer manifest, retirement and residual-debt lenses, and cross-footprint/runtime scenarios are absent; the failure names those contract gaps rather than an unrelated compile or environment error."

Add a marker-backed `TestCleanIntegrationContract` near the existing maintainable-code spine tests.
It reads `templates/partials/clean-integration.md`, rejects headings and duplicate authored doctrine,
walks the approved explicit consumer manifest, requires one include per applicable consumer, and
renders configured and empty-data variants without unresolved tokens. Extend the existing stage,
subagent, implementer, guide, and reviewer tests only where their current invariant markers own the
changed contract; preserve Core's absence of Full-only plan and ADR protocol while proving that every
applicable Core consumer receives equivalent clean-integration semantics.

Add `internal/evals/clean_integration_test.go` in the deterministic eval harness. Cover Core and Full
and Pi and Claude with outcome-oriented scenarios for duplicated policy, adapter representation
leakage, an existing clean owner requiring no refactor, unrelated cleanup exclusion, obsolete-path
removal or explicit migration, residual debt, and rejection of test-shaped production design. The
scenarios require proportional answers rather than long checklist output and distinguish a bounded
enabling refactor inside scope from a protected-contract change that returns to the material-decision
boundary.

### Task 1.2: Project the shared operative rule through approved consumers
Kind: batch
Applying: ["make-clean-integration-operative:canonical-guide-operative-rule", "make-clean-integration-operative:clean-integration-questions", "make-clean-integration-operative:bounded-refactor-inside-scope", "make-clean-integration-operative:retirement-without-speculation", "make-clean-integration-operative:operative-consumer-and-proof"]
Paths: ["templates/partials/clean-integration.md", "templates/docs/maintainable-code-design.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/executing-direct/SKILL.md.tmpl", "templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/bugfix/SKILL.md.tmpl", "templates/skills/tdd/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/agents/implementer.md.tmpl", "templates/agents/plan-reviewer.md.tmpl", "templates/agents/code-reviewer.md.tmpl"]
Representative: "The partial points to the maintainable-code guide, asks proportionate owner, integration, refactor, retirement, verification, and debt questions, and a simple change answers them without ceremonial output."
Edge: "A bounded refactor would create a durable choice, raise risk, change external behavior, or expand the requested outcome, so the consumer returns to the material-decision boundary rather than silently broadening scope."
Post-check: "`go test ./internal/project -run 'TestCleanIntegrationContract|TestMaintainableCodeReviewLenses|TestMaintainableCodeStageCoverage|TestMaintainableCodeSubagentContract'` exits zero; the contract test reports exactly one authored operative home, every applicable approved consumer includes it once, configured and empty-data renders contain no unresolved token, and no retired parallel operative clause remains."

Create the shared partial as the sole operative rule. It points to the design guide without restating
its doctrine, requires proportional treatment, places necessary bounded enabling work inside scope,
and states the separate-decision boundary. It names practical obsolete-route retirement, applicable
verification surfaces, residual debt, YAGNI, and the test-shaped-design prohibition. Keep the text
heading-free so each consumer retains its own document structure.

Include the partial directly beside the existing maintainability obligation in brainstorming, plan
writing, direct execution, plan execution, subagent-driven development, bugfix, TDD, plan review,
implementation review, implementer, plan-reviewer, and code-reviewer templates. Retire only parallel
operative question lists; preserve brainstorming approval, plan structure, transaction ownership,
helper confinement, root-cause and test-first protocol, implementation autonomy, review remediation,
and report-only behavior. The reviewer consumers add explicit one-home, obsolete-path,
dependency-direction, representation-boundary, and residual-debt lenses without stating whether a
finding blocks. Add only a concise clean-integration imperative and guide reference to the agent
guide. Clarify the design guide's doctrine ownership and proportional workflow application without
embedding the operative questions there.

### Task 1.3: Apply authority, render outputs, and record adopter behavior
Kind: batch
Applying: ["make-clean-integration-operative:canonical-guide-operative-rule", "make-clean-integration-operative:clean-integration-questions", "make-clean-integration-operative:bounded-refactor-inside-scope", "make-clean-integration-operative:retirement-without-speculation", "make-clean-integration-operative:operative-consumer-and-proof"]
Paths: ["docs/decisions/make-clean-integration-operative.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", "changelog/CHANGELOG.md", ".awf/awf.lock", "AGENTS.md", "docs/maintainable-code-design.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/topics/rendering/guide-and-doc-templates.md", ".pi/skills/awf-brainstorming/SKILL.md", ".pi/skills/awf-writing-plans/SKILL.md", ".pi/skills/awf-executing-direct/SKILL.md", ".pi/skills/awf-executing-plans/SKILL.md", ".pi/skills/awf-subagent-driven-development/SKILL.md", ".pi/skills/awf-bugfix/SKILL.md", ".pi/skills/awf-tdd/SKILL.md", ".pi/skills/awf-reviewing-plan/SKILL.md", ".pi/skills/awf-reviewing-impl/SKILL.md", ".claude/skills/awf-brainstorming/SKILL.md", ".claude/skills/awf-writing-plans/SKILL.md", ".claude/skills/awf-executing-direct/SKILL.md", ".claude/skills/awf-executing-plans/SKILL.md", ".claude/skills/awf-subagent-driven-development/SKILL.md", ".claude/skills/awf-bugfix/SKILL.md", ".claude/skills/awf-tdd/SKILL.md", ".claude/skills/awf-reviewing-plan/SKILL.md", ".claude/skills/awf-reviewing-impl/SKILL.md", ".pi/agents/implementer.md", ".pi/agents/plan-reviewer.md", ".pi/agents/code-reviewer.md", ".claude/agents/implementer.md", ".claude/agents/plan-reviewer.md", ".claude/agents/code-reviewer.md"]
Post-check: "After `./x render`, `./x check` exits zero; `./awf context --show pending docs/decisions/make-clean-integration-operative.md` reports no Remaining operation from the phase batch; focused project and eval tests exit zero; generated Core and Full and Pi and Claude consumers carry equivalent applicable semantics without `<no value>`; and a focused meaning review finds no contradictory parallel operative rule or AF-005 blocking policy."

Use `awf-adr-lifecycle` to transition the reviewed ADR to Implementing and append one Applied batch
containing its declared claim operations. Add the new tested `clean-integration` invariant and update
`maintainable-code-stage-coverage`, `maintainable-code-subagent-contract`,
`implementer-role-contract`, `maintainable-code-review-lenses`, and
`maintainable-code-design-guide` with final current behavior and correct Origin or Revised-by
provenance. Keep claim boundaries precise: the new invariant owns the partial, consumer manifest,
proportional semantics, and scenario proof; existing claims retain their stage, agent, review, and
guide responsibilities.

Add an Unreleased feature entry for adopter-visible clean-integration behavior. Run `./x render` so
the decision index, current-state projections, lock, guide, docs, skill trees, and agent trees move
with authored sources. Inspect representative generated Core and Full outputs for both runtimes,
including the guide, brainstorming, direct execution, plan writing where applicable, implementer,
and both reviewers. Record concept-preserving readings, intentional profile differences, absence of
contradictory fragments, and absence of severity policy in the phase evidence. Run focused tests,
`./awf check staged`, and the full gate before phase close.

### Phase close

Land tests, the canonical operative rule, approved consumers, claim mutations, generated outputs,
ADR application history, and changelog as one independently green transaction. After the closing
commit exists, run `./x audit-local 4b0276413..HEAD` and require a clean result before phase review.

```commit
feat(rendering): make clean integration operative (applies ADR batch)
```

## Definition of done

- `dod: clean-integration-operative` Every approved design, planning, implementation, and review path applies one proportional shared clean-integration rule while the canonical guide retains doctrine ownership and AF-005 severity policy remains outside scope.
- `dod: clean-integration-proven` Contract, empty-data, generated-output, and deterministic behavioral scenarios prove one authored home, explicit applicable consumers, bounded refactoring, practical retirement, residual-debt visibility, YAGNI, and runtime and footprint parity.

## Notes

Apply the plan-flexibility rule from the plan-writing contract. Record only material route changes or
review findings another phase, reviewer, or terminal assurance can rely on. After implementation
assurance settles, `awf-effort-workflow` reconciles those records, appends only the ADR's Implemented
event, changes this plan to `status: Implemented`, renders the decision index and lock, and commits
that lifecycle-only transaction before managed-topology removal.

- Phase review found that the initial deterministic evaluations asserted shared prose rather than
  scenario-specific outcomes, the single-home test scanned only the consumer manifest, reviewer
  stages were not independently covered by their named claim test, and reviewer lens wording could
  be read as suppressing the required informational severity field. The settlement adds input and
  outcome fixtures with mutation checks, scans the complete authored skill, agent, and partial
  surface with direct and paraphrased negative cases, covers both review stages, and clarifies that
  clean-integration lenses do not define or change severity or blocking policy. The review-lens
  claim correction is recorded through its Reapplied event; the approved AF-005 boundary is
  unchanged.
- The single verify pass found that scenario inputs and outcomes were still diagnostic labels over one
  shared prose match. The residual settlement now evaluates each input through a distinct accepted
  disposition, rejects its prohibited counterpart, mutation-checks every governing clause, and
  separately covers durable choice, risk increase, external behavior change, and outcome expansion.
  This strengthens the approved proof without changing operative semantics.
