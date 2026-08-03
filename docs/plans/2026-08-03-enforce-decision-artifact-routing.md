---
format: plan-v2
date: 2026-08-03
adrs: [decision-artifact-routing-boundaries]
status: Proposed
---
# Plan: Enforce Decision Artifact Routing

## Goal

Publish and enforce one coherent routing boundary for ADR decisions, implementation-plan directives,
and effort memory, while leaving historical ADR bodies and the ADR prose parser unchanged.

## Architecture summary

One inline rendering transaction first pins the objective template contracts in existing project,
golden, and adopter-wiring tests, then updates the generic templates, project overrides, reviewer
catalog, and generated outputs those tests cover. Semantic classification remains reviewer-owned;
no production parser or prose linter is introduced. The single pending state operation,
`add rendering/templates:decision-artifact-routing`, is intentionally reserved for the direct
terminal flip after implementation review, where the unbacked policy claim, ADR `Implemented`
event, and plan `Implemented` status land atomically.

## Phase 1: Publish the routing contract

**Execution mode: inline.**

Completes: ["routing-guidance", "reviewer-boundary", "scaffold-format-authority"]

### Task 1.1: Pin objective rendering contracts
Applying: ["decision-artifact-routing-boundaries:mechanize-objective-contracts-only", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: ["internal/project/spine_test.go", "internal/project/golden_test.go", "internal/project/example_wiring_test.go"]

Extend the existing rendering-contract tests before changing template prose:

- In `internal/project/spine_test.go`, strengthen the ADR README, ADR template, brainstorming,
  proposing-ADR, writing-plans, and ADR-reviewer golden assertions at their existing per-artifact
  test functions. Assert the positive post-implementation/counterfactual boundary, plan ownership
  of paths/commands/order/rollout/ordinary test transactions, the durable-mechanism exception,
  semantic reasoned review, and preservation of scaffold-emitted frontmatter. Assert that current
  proposing guidance does not instruct an author to select a literal format marker.
- In `internal/project/golden_test.go`, extend the existing full-project render assertions so the
  project-owned ADR-template override and generated awf outputs contain the same routing contract
  and the ADR reviewer no longer asks the ADR body for an implementation inventory.
- In `internal/project/example_wiring_test.go`, extend the existing Pi/Claude Sundial checks to
  assert the adopter's proposing-ADR and ADR-reviewer outputs preserve scaffold authority and the
  semantic routing lens without no-value residue.

Run `go test ./internal/project -run 'Test(ADR|Adr|Brainstorming|WritingPlan|Golden|Example)'`.
The expected terminal state before the production edits is a failure caused only by the newly
asserted missing or contradictory guidance; after the remaining tasks it exits zero.

### Task 1.2: Define the generic authoring boundary
Applying: ["decision-artifact-routing-boundaries:route-by-durability", "decision-artifact-routing-boundaries:test-decision-content", "decision-artifact-routing-boundaries:keep-directives-in-plans", "decision-artifact-routing-boundaries:preserve-historical-records", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: ["templates/adr-readme/README.md.tmpl", "templates/adr-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl"]

Make the generic published guidance agree on one routing model:

- Expand the ADR README's authoring guidance with the post-implementation and counterfactual tests,
  the durable-mechanism exception, and a compact routing table. Use generic positive examples of
  policy, ownership, compatibility, safety, and reproducibility constraints and generic negative
  examples of paths, commands, ordering, rollout batches, and ordinary test transactions. State
  that historical records remain history rather than retrofit targets.
- Tighten the ADR template's `Decision` prompt to require durable commitments and direct execution
  details to the plan. Preserve coherent fallback prose under unset template data.
- State in the plans README that implementation directives are the plan's responsibility and that
  discovering a new durable choice routes back to ADR authoring rather than hiding the choice in a
  task.
- Align brainstorming, proposing-ADR, and writing-plans skills with that routing. In the proposing
  skill, replace the hardcoded `current-state-v3` convention with an instruction to preserve
  exactly the frontmatter emitted by `awf new adr`; do not substitute another literal marker.

Do not cite numbered project ADRs in embedded generic templates. Do not add a new artifact,
section, Decision-item kind, parser rule, or keyword lint.

### Task 1.3: Make semantic review and project overrides consistent
Applying: ["decision-artifact-routing-boundaries:review-semantics", "decision-artifact-routing-boundaries:keep-directives-in-plans", "decision-artifact-routing-boundaries:mechanize-objective-contracts-only"]
Paths: ["templates/agents/adr-reviewer.md.tmpl", "internal/catalog/standard.go", ".awf/parts/adr-template/body.md", ".awf/skills/parts/writing-plans/conventions-tasks.md", ".awf/agents/adr-reviewer.yaml"]

Update the universal ADR reviewer to apply the post-implementation, counterfactual, and routing
tests to every Decision item. It must classify a misplaced implementation directive as a reasoned
finding and allow a mechanism when the ADR explains why it is itself load-bearing.

Remove the ADR-review requirement that the ADR body declare exact same-commit documentation work,
regeneration tasks, or other implementation inventories. Remove the ADR reviewer's `doc-currency`
section from its catalog section list and remove the generic and project `docCurrencyItems` data
that only fed that section. Preserve ADR-review ownership of rationale quality, consequences,
Decision-to-State-changes fidelity, topic cohesion, and claim agreement. Plan review and lifecycle
execution continue to enforce implementation completeness and atomic claim mutations.

Mirror the generic boundary in the project-owned ADR-template and writing-plan overrides so render
does not restore the old ambiguity. Preserve all existing `missingkey=zero`, no-value-residue,
working-memory, and publication-safety contracts.

### Task 1.4: Render and verify every published target
Kind: batch
Applying: ["decision-artifact-routing-boundaries:mechanize-objective-contracts-only", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: [".awf/awf.lock", ".pi/agents/adr-reviewer.md", ".pi/skills/awf-brainstorming/SKILL.md", ".pi/skills/awf-proposing-adr/SKILL.md", ".pi/skills/awf-writing-plans/SKILL.md", ".claude/agents/adr-reviewer.md", ".claude/skills/awf-brainstorming/SKILL.md", ".claude/skills/awf-proposing-adr/SKILL.md", ".claude/skills/awf-writing-plans/SKILL.md", "docs/decisions/README.md", "docs/decisions/template.md", "docs/plans/README.md", "examples/sundial/.pi/agents/adr-reviewer.md", "examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md", "examples/sundial/.pi/skills/sundial-proposing-adr/SKILL.md", "examples/sundial/.pi/skills/sundial-writing-plans/SKILL.md", "examples/sundial/.claude/agents/adr-reviewer.md", "examples/sundial/.claude/skills/sundial-brainstorming/SKILL.md", "examples/sundial/.claude/skills/sundial-proposing-adr/SKILL.md", "examples/sundial/.claude/skills/sundial-writing-plans/SKILL.md"]
Representative: Run `./x render` and verify the awf Pi proposing skill preserves scaffold-emitted frontmatter, the awf ADR reviewer applies semantic routing, and the ADR and plan guides name complementary authority.
Edge: Verify the Sundial Pi and Claude outputs carry the generic contract without awf-specific paths, numbered ADR citations, unresolved values, or a literal current authoring format instruction.
Post-check: Run `./x check`; run `git diff --check`; run `! rg -n 'Required frontmatter:.*current-state-v3|format.*\(`current-state-v[0-9]+`\)' .pi/skills/*-proposing-adr/SKILL.md .claude/skills/*-proposing-adr/SKILL.md examples/sundial/.pi/skills/*-proposing-adr/SKILL.md examples/sundial/.claude/skills/*-proposing-adr/SKILL.md`; all commands must reach a clean or zero-finding terminal state.

Run `./x render` from the repository root and stage every generated change it reports. Inspect the
complete render diff; do not hand-edit generated files or assume the enumerated outputs are the
entire changed set. Confirm that no terminal historical ADR body changed.

### Phase close

Stage the complete rendering transaction explicitly. Run `./x check staged` and `./x gate`; both
must exit zero. Create the single phase-closing commit:

```commit
feat(rendering): enforce decision artifact routing
```

## Definition of done

- `dod: routing-guidance` ADR, plan, and brainstorming/authoring guidance route durable commitments, implementation directives, and transient context consistently in awf and Sundial outputs.
- `dod: reviewer-boundary` ADR review reports misplaced directives semantically without requiring implementation inventories in the ADR body, while plan and lifecycle checks retain execution ownership.
- `dod: scaffold-format-authority` Proposing guidance preserves scaffold-emitted frontmatter and contains no duplicated literal current-format instruction.

## Notes

The terminal implementation-review transaction adds the unbacked
`rendering/templates:decision-artifact-routing` claim with `Origin: ADR-decision-artifact-routing-boundaries`,
a reasoned `Verify:` procedure, and no proof marker; directly applies the ADR operation and freezes
both the ADR and this plan as `Implemented`. The objective rendering tests are regressions for
publication mechanisms and must not be marked as backing for the unbacked semantic claim.
