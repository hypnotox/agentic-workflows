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
Latitude: exact
Applying: ["decision-artifact-routing-boundaries:mechanize-objective-contracts-only", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: ["internal/project/spine_test.go", "internal/project/golden_test.go", "internal/project/example_wiring_test.go"]

Extend the existing rendering-contract tests before changing template prose:

- In `internal/project/spine_test.go`, strengthen `TestAdrReviewerAgent`,
  `TestWritingPlansTemplate`, `TestProposingAdrTemplate`, `TestBrainstormingTemplate`,
  `TestV3ADRTemplateEmptyDataFallback`, and `TestAgentsDocGuide`. Require the semantic anchors
  `remains meaningful after implementation`, `post-implementation`, `counterfactual`,
  `mechanism itself is load-bearing`, and `reasoned finding`; require plan routing to name paths,
  commands, task order, rollout batches, and ordinary test transactions; require proposing guidance
  to say `preserve exactly the frontmatter emitted by` `awf new adr`; reject an instruction that
  chooses a literal `current-state-vN` marker.
- In `internal/project/golden_test.go`, add `TestADRReadmeDecisionRouting` beside the existing
  ADR-template publication helper and extend `TestEndToEndGolden`. Render the ADR README directly
  and require the same durability tests, routing categories, historical-record preservation, and
  coherent unset-data output. Assert that the full-project ADR-template override, AGENTS guide,
  proposing skill, and ADR reviewer carry their assigned contract and that the reviewer no longer
  asks the ADR body for an implementation inventory.
- In `internal/project/example_wiring_test.go`, extend `TestExampleAdopterWiring` to assert the
  Sundial Pi and Claude proposing-ADR and ADR-reviewer outputs preserve scaffold authority and the
  semantic routing lens, and that Sundial `AGENTS.md` carries the authority-lifetime routing rule
  without no-value residue.

Run `go test ./internal/project -run '^(TestAdrReviewerAgent|TestWritingPlansTemplate|TestProposingAdrTemplate|TestBrainstormingTemplate|TestV3ADRTemplateEmptyDataFallback|TestAgentsDocGuide|TestADRReadmeDecisionRouting|TestEndToEndGolden|TestExampleAdopterWiring)$'`.
The expected terminal state before the production edits is a failure caused only by the newly
asserted missing or contradictory guidance; after the remaining tasks it exits zero.

### Task 1.2: Define the generic authoring boundary
Latitude: exact
Applying: ["decision-artifact-routing-boundaries:route-by-durability", "decision-artifact-routing-boundaries:test-decision-content", "decision-artifact-routing-boundaries:keep-directives-in-plans", "decision-artifact-routing-boundaries:preserve-historical-records", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: ["templates/adr-readme/README.md.tmpl", "templates/adr-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", "templates/agents-doc/AGENTS.md.tmpl", "templates/skills/brainstorming/SKILL.md.tmpl", "templates/skills/proposing-adr/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl"]

Make the generic published guidance agree on one routing model:

- Expand the ADR README's authoring guidance with the post-implementation and counterfactual tests,
  the durable-mechanism exception, and a compact routing table. Derive generic anti-patterns from
  the representative historical findings without citing project-specific record numbers: rollout
  inventories, proof transactions, affected-file lists, commands, ordering, rollout batches, and
  ordinary test transactions route to plans, while durable policy, ownership, compatibility,
  safety, and reproducibility constraints remain valid Decision examples. State that historical
  records remain history rather than retrofit or classification targets.
- Tighten the ADR template's `Decision` prompt to require durable commitments and direct execution
  details to the plan. Preserve coherent fallback prose under unset template data.
- State in the plans README that implementation directives are the plan's responsibility and that
  discovering a new durable choice routes back to ADR authoring rather than hiding the choice in a
  task.
- Add a concise authority-lifetime routing rule to the agent guide's Workflow section, then align
  brainstorming, proposing-ADR, and writing-plans skills with it. In the proposing skill, replace
  the hardcoded `current-state-v3` convention with the exact positive instruction `preserve exactly
  the frontmatter emitted by` `awf new adr`; do not substitute another literal marker.

Do not cite numbered project ADRs in embedded generic templates. Do not add a new artifact,
section, Decision-item kind, parser rule, or keyword lint.

### Task 1.3: Make semantic review and project overrides consistent
Latitude: exact
Applying: ["decision-artifact-routing-boundaries:review-semantics", "decision-artifact-routing-boundaries:keep-directives-in-plans", "decision-artifact-routing-boundaries:mechanize-objective-contracts-only"]
Paths: ["templates/agents/adr-reviewer.md.tmpl", "internal/catalog/standard.go", ".awf/parts/adr-template/body.md", ".awf/skills/parts/writing-plans/conventions-tasks.md", ".awf/agents/adr-reviewer.yaml"]

Update the universal ADR reviewer to apply the post-implementation, counterfactual, and routing
tests to every Decision item. It must classify a misplaced implementation directive as a reasoned
finding and allow a mechanism when the ADR explains why it is itself load-bearing.

Remove the ADR-review requirement that the ADR body declare exact same-commit documentation work,
regeneration tasks, or other implementation inventories. Delete the `doc-currency (ADR-level)`
universal lens and the `## Doc-currency checklist` section from the ADR reviewer template. Remove
`doc-currency` from the ADR reviewer's `Sections` list in `internal/catalog/standard.go`, delete its
`docCurrencyItems` default, and delete the project replacement `docCurrencyItems` mapping from
`.awf/agents/adr-reviewer.yaml`. Preserve ADR-review ownership of rationale quality, consequences,
Decision-to-State-changes fidelity, topic cohesion, and claim agreement. Plan review and lifecycle
execution continue to enforce implementation completeness and atomic claim mutations.

Mirror the generic boundary in the project-owned ADR-template and writing-plan overrides so render
does not restore the old ambiguity. Preserve all existing `missingkey=zero`, no-value-residue,
working-memory, and publication-safety contracts.

### Task 1.4: Render and verify every published target
Kind: batch
Latitude: exact
Applying: ["decision-artifact-routing-boundaries:mechanize-objective-contracts-only", "decision-artifact-routing-boundaries:scaffold-owns-format"]
Paths: [".awf/awf.lock", "AGENTS.md", ".pi/agents/adr-reviewer.md", ".pi/skills/awf-brainstorming/SKILL.md", ".pi/skills/awf-proposing-adr/SKILL.md", ".pi/skills/awf-writing-plans/SKILL.md", ".claude/agents/adr-reviewer.md", ".claude/skills/awf-brainstorming/SKILL.md", ".claude/skills/awf-proposing-adr/SKILL.md", ".claude/skills/awf-writing-plans/SKILL.md", "docs/decisions/README.md", "docs/decisions/template.md", "docs/plans/README.md", "examples/sundial/.awf/awf.lock", "examples/sundial/AGENTS.md", "examples/sundial/docs/decisions/README.md", "examples/sundial/docs/decisions/template.md", "examples/sundial/docs/plans/README.md", "examples/sundial/.pi/agents/adr-reviewer.md", "examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md", "examples/sundial/.pi/skills/sundial-proposing-adr/SKILL.md", "examples/sundial/.pi/skills/sundial-writing-plans/SKILL.md", "examples/sundial/.claude/agents/adr-reviewer.md", "examples/sundial/.claude/skills/sundial-brainstorming/SKILL.md", "examples/sundial/.claude/skills/sundial-proposing-adr/SKILL.md", "examples/sundial/.claude/skills/sundial-writing-plans/SKILL.md"]
Representative: Run `./x render`; the awf Pi proposing skill contains `preserve exactly the frontmatter emitted by`, the awf ADR reviewer contains the post-implementation and counterfactual tests plus `reasoned finding`, and the ADR and plan guides route durable authority and execution details complementarily.
Edge: The Sundial ADR guide, ADR template, plan guide, AGENTS guide, and Pi/Claude proposing/reviewer outputs carry the same generic anchors without awf-specific paths, numbered ADR citations, unresolved values, or a literal current authoring format instruction; both awf lock files record the rendered hashes.
Post-check: Run `./x check`; run `git diff --check`; run `! rg -n 'Required frontmatter:.*current-state-v3|format.*\(`current-state-v[0-9]+`\)' .pi/skills/*-proposing-adr/SKILL.md .claude/skills/*-proposing-adr/SKILL.md examples/sundial/.pi/skills/*-proposing-adr/SKILL.md examples/sundial/.claude/skills/*-proposing-adr/SKILL.md`; run `test -z "$(git diff --name-only | grep -Fvx -f <(printf '%s\n' 'internal/project/spine_test.go' 'internal/project/golden_test.go' 'internal/project/example_wiring_test.go' 'templates/adr-readme/README.md.tmpl' 'templates/adr-template/template.md.tmpl' 'templates/plans-readme/README.md.tmpl' 'templates/agents-doc/AGENTS.md.tmpl' 'templates/skills/brainstorming/SKILL.md.tmpl' 'templates/skills/proposing-adr/SKILL.md.tmpl' 'templates/skills/writing-plans/SKILL.md.tmpl' 'templates/agents/adr-reviewer.md.tmpl' 'internal/catalog/standard.go' '.awf/parts/adr-template/body.md' '.awf/skills/parts/writing-plans/conventions-tasks.md' '.awf/agents/adr-reviewer.yaml' '.awf/awf.lock' 'AGENTS.md' '.pi/agents/adr-reviewer.md' '.pi/skills/awf-brainstorming/SKILL.md' '.pi/skills/awf-proposing-adr/SKILL.md' '.pi/skills/awf-writing-plans/SKILL.md' '.claude/agents/adr-reviewer.md' '.claude/skills/awf-brainstorming/SKILL.md' '.claude/skills/awf-proposing-adr/SKILL.md' '.claude/skills/awf-writing-plans/SKILL.md' 'docs/decisions/README.md' 'docs/decisions/template.md' 'docs/plans/README.md' 'examples/sundial/.awf/awf.lock' 'examples/sundial/AGENTS.md' 'examples/sundial/docs/decisions/README.md' 'examples/sundial/docs/decisions/template.md' 'examples/sundial/docs/plans/README.md' 'examples/sundial/.pi/agents/adr-reviewer.md' 'examples/sundial/.pi/skills/sundial-brainstorming/SKILL.md' 'examples/sundial/.pi/skills/sundial-proposing-adr/SKILL.md' 'examples/sundial/.pi/skills/sundial-writing-plans/SKILL.md' 'examples/sundial/.claude/agents/adr-reviewer.md' 'examples/sundial/.claude/skills/sundial-brainstorming/SKILL.md' 'examples/sundial/.claude/skills/sundial-proposing-adr/SKILL.md' 'examples/sundial/.claude/skills/sundial-writing-plans/SKILL.md'))"`; every command must reach a clean or zero-finding terminal state.

Run `./x render` from the repository root and inspect its complete diff. Do not hand-edit generated
files. The post-check rejects any changed path outside the source, test, and manifest-governed
output scope above. Confirm that no terminal historical ADR body changed; staging remains exclusively
in the Phase close.

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

The terminal implementation-review transaction edits
`.awf/topics/parts/rendering/templates/current-state.md` to add the unbacked
`rendering/templates:decision-artifact-routing` claim with `Origin:
ADR-decision-artifact-routing-boundaries`, a reasoned `Verify:` procedure, and no proof marker;
directly applies the ADR operation and freezes both the ADR and this plan as `Implemented`. Run
`./x render` in that same transaction and stage the resulting
`docs/topics/rendering/templates.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock` with the claim,
Applied event, and status flips. The objective rendering tests are regressions for publication
mechanisms and must not be marked as backing for the unbacked semantic claim.

Plan review confirmed that every `Paths:` value remains repository-root relative, as required by
the plan-v2 convention; the reviewer's absolute-path suggestion was rejected. The user approved
requiring generic examples to derive from the representative historical audit findings while
preserving the frozen records. After the single verify pass, the user also approved adding the
three omitted Sundial guide outputs and replacing broad or non-expanding changed-path patterns with
an explicit exhaustive allowlist.
