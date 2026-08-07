---
format: plan-v2
date: 2026-08-07
adrs: [deduplicate-plan-authoring-and-execution-workflow]
status: Proposed
---
# Plan: Deduplicate Plan Authoring and Execution Workflow

## Goal

Make awf's one workflow faster to plan and execute by centralizing generic mechanics, retiring the separate plan-resync node, and reusing freshness-scoped assurance without weakening authority, review, gate, or lifecycle guarantees. Do not introduce workflow profiles, depth controls, routers, or classifiers.

## Architecture summary

Keep `internal/plan` as the typed plan model owner, compose ADR-to-plan associations once in `internal/project`, and expose them through the existing `awf context` boundary. Move generic plan and implementation protocol to its existing workflow and agent owners, while plans retain only change-specific direction. Land the query seam first, then the plan and assurance contract, then retire resync across catalog, migration, current-state authority, generated targets, and adopter documentation. Apply current-state operations with the behavior they describe; leave the ADR Implementing and this plan Proposed until independent assurance settles.

## Phase 1: Expose typed linked plans for ADR freshness

**Execution mode: subagent-driven.**

Advances: ["resync-retired", "repository-green"]
Completes: ["linked-plan-query"]

### Task 1.1: Compose reverse ADR-to-plan associations
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness"]
Paths: ["internal/project/plan_context.go", "internal/project/plan_context_test.go", "internal/plan/model.go", "internal/plan/parse.go"]

Start from the committed, approved ADR and reviewed plan in the managed worktree. Require `git status --short` to be empty, `./x check` to report zero findings, and `./x gate` to pass at 100% statement coverage.

Add one project-owned reverse association over the already-parsed plan set and ADR corpus. Normalize each `plan-v2` frontmatter `adrs:` entry through corpus identity resolution, associate it with the plan path and filename, deduplicate aliases that resolve to the same record, and return deterministic path order. Include Proposed and Implemented plans in the typed result so the query describes repository relationships rather than guessing workflow state; the review skill decides which mutable plans require renewed review. Do not scan Markdown, infer from modification time, or make Decision-level Applying assignments substitute for the plan-level ADR link.

Cover zero links, one link, several plans linked to one ADR, one plan linked to several ADRs, retained pending-slug and numbered identities resolving to the same record, unrelated plans, deterministic ordering, legacy or marker-absent plans, and unresolved links following the existing blocking plan-reference behavior. Keep parsing in `internal/plan` and corpus composition in `internal/project`; add no second parsed-plan representation.

### Task 1.2: Render linked plans through awf context
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness"]
Paths: ["internal/contextq/context_adr.go", "internal/contextq/context_projection.go", "internal/contextq/context_paths.go", "internal/contextq/render.go", "internal/contextq/context_adr_test.go", "internal/contextq/context_projection_test.go", "internal/contextq/render_test.go", "cmd/awf/context.go", "cmd/awf/context_test.go"]

Extend the existing `references` facet for an explicit governed ADR request with a compact `linked plans` collection supplied by `internal/project`. Render normalized repository-relative paths in deterministic order and omit the collection when empty. Keep tier-0 and unrelated file queries unchanged, do not make linked plans active authority, and preserve the ADR projection's pending-intent or decision-history role. Ensure spill delivery remains owned by the existing context output path.

Test explicit numbered and pending ADR requests, no linked plans, multiple linked plans, unrelated plan links, references-facet omission, canonical ordering, ordinary context output stability, and command error/presentation behavior. The command layer receives typed projection data and adds no plan parsing or relationship logic.

### Task 1.3: Apply the linked-plan query claim
Latitude: exact
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness"]
Paths: ["docs/decisions/deduplicate-plan-authoring-and-execution-workflow.md", "docs/decisions/INDEX.md", ".awf/topics/parts/tooling/context-and-topic/current-state.md", "docs/topics/tooling/context-and-topic.md", ".awf/awf.lock"]

Use `awf-adr-lifecycle` for the first incremental application. Move the ADR to Implementing, append the required content stamp, and append one Applied event for exactly:

- add `tooling/context-and-topic:adr-linked-plan-references`

In the same transaction add the test-backed invariant with Origin naming the pending ADR. State that the references facet on an explicit governed ADR request reports the deterministic repository-relative set of plans whose parsed plan-level links resolve to that ADR, without Markdown scanning, modification-time inference, current-authority promotion, or output on unrelated requests. Put the proof marker on the focused project/context test that exercises alias resolution and deterministic output. Run `./x render`, read back the ADR, generated index, source and rendered topic, and lock, then require `./x check` to report zero findings.

### Phase close

Stage the complete Phase 1 transaction explicitly and create its closing commit after `awf check staged` and `./x gate` pass:

```commit
feat(tooling): expose linked plans for ADR context
```

## Phase 2: Centralize plan mechanics and reuse phase assurance

**Execution mode: subagent-driven.**

Advances: ["repository-green"]
Completes: ["change-specific-plans", "assurance-reuse"]

### Task 2.1: Make plan authoring change-specific
Applying: ["deduplicate-plan-authoring-and-execution-workflow:change-specific-plan-ownership", "deduplicate-plan-authoring-and-execution-workflow:authority-resolved-local-detail", "deduplicate-plan-authoring-and-execution-workflow:proportionate-plan-fields", "deduplicate-plan-authoring-and-execution-workflow:precommit-plan-review"]
Paths: ["templates/skills/writing-plans/SKILL.md.tmpl", "templates/plans-template/template.md.tmpl", "templates/plans-readme/README.md.tmpl", ".awf/skills/parts/writing-plans/conventions-tasks.md", "templates/agents/plan-reviewer.md.tmpl", "internal/project/plan_detail_modes_test.go", "internal/project/docs_sections_test.go"]

Start only from the committed, reviewed Phase 1 tip with a clean worktree and green `./x check` and `./x gate`.

Rewrite the default plan contract so required task content is the change-specific observable outcome, relevant authority links, material boundaries, ordering dependencies, focused evidence, and explicit confinement where ambiguity or helpers require it. Keep `Applying`, `Context`, execution mode, Phase close, `Advances`, `Completes`, and necessary `Paths` in their current typed roles. Preserve historical grammar and parsing. Keep deterministic `Post-check` evidence for batch, glob, or pathspec populations, but stop requiring authors to repeat generic staging, gate, clean-tree, checkpoint, model-routing, or reviewer protocol in task bodies, Phase close prose, and Definition of done.

Demote `Latitude`, `Kind: batch`, `Representative`, and `Edge` from universal authoring requirements to optional aids while leaving accepted historical fields valid. Keep `Kind: spike` for uncertainty that can change outcome, material scope, durable boundaries, or verification; explicitly allow a commit-capable phase owner to resolve authority-determined local symbols, helper structure, test arrangement, and necessary omitted paths under the existing autonomy boundary. Keep commit-disabled helper confinement unchanged.

Move focused generated-prose meaning review to the implementation phase owner. Plans name concrete examples and expected readings only when load-bearing; the phase completion evidence records inspected output boundaries and result, and plan/code reviewers inspect the requirement and evidence. Preserve contradictory-fragment, concept-preserving paraphrase, and intentional literal-placeholder checks without creating a universal language validator.

Update the plan template and README to demonstrate the compact contract. Extend alignment tests to prove the plan surfaces keep typed fields, scope confinement, focused terminal-state checks, authority-resolved local detail, semantic-review ownership, missingkey-zero publication safety, and the absence of duplicated generic execution protocol.

### Task 2.2: Review the draft before its first plan commit
Applying: ["deduplicate-plan-authoring-and-execution-workflow:precommit-plan-review"]
Paths: ["templates/skills/writing-plans/SKILL.md.tmpl", "templates/skills/reviewing-plan/SKILL.md.tmpl", "internal/catalog/standard.go", "internal/project/spine_test.go", "internal/project/plan_detail_modes_test.go"]

Change the writing-to-review boundary so the report-only plan reviewer reads the newly written uncommitted plan before its initial commit. Apply mechanical corrections without a durable ledger; record substantive reasoned or user-decided findings and their dispositions in Notes; run the existing single verify pass when its trigger fires; then create one settled initial plan commit. Preserve new commits for every later substantive correction and preserve every-commit staged checks and full gate. The plan reviewer must accept an explicit uncommitted path and assess the selected working-tree snapshot rather than silently substituting HEAD.

Update catalog section declarations for the changed writing-plan and reviewing-plan sections without changing the skill set in this phase. Add rendered-contract tests for no pre-review commit, substantive Notes evidence, mechanical-no-ledger behavior, later-fix commits, report-only review, the verify-pass bound, and coherent empty-data rendering.

### Task 2.3: Make phase review reusable assurance evidence
Applying: ["deduplicate-plan-authoring-and-execution-workflow:freshness-scoped-assurance-reuse"]
Paths: ["templates/skills/executing-plans/SKILL.md.tmpl", "templates/skills/subagent-driven-development/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "templates/agents/implementer.md.tmpl", "templates/agents/code-reviewer.md.tmpl", "internal/project/phase_transaction_ownership_test.go", "internal/project/spine_test.go", "internal/evals/independent_workflow_escalation_test.go"]

Define delegated phase review evidence as the exact phase-closing commit, the complete phase scope, verification results, and verbatim deviation report. Require phase review to use the code-review correctness, plan/authority adherence, documentation, and maintainability lenses needed to cover that scope. Record its covered range and freshness in the parent settlement.

At terminal assurance, reuse only still-fresh covered phase evidence. For one phase, review only unreviewed settlement or integration changes; for several phases, focus the fresh review on cross-phase composition, settlements, and integration effects. Always run `awf audit` and this repository's `./x audit-local` across the complete final implementation range. Include every post-phase settlement commit in coverage. Divergence, changed ADR authority, reasoned post-review fixes, or any other material mutation invalidates the affected coverage and triggers renewed review before finalization.

Keep phase reviewers report-only, implementation fixes parent-owned, helpers confined, and every commit gated. Extend phase-ownership and shared-spine tests to prove the coverage evidence, audit retention, invalidation cases, no duplicate already-covered phase reading, and both target renderings under empty data.

### Task 2.4: Apply plan and assurance authority
Latitude: exact
Applying: ["deduplicate-plan-authoring-and-execution-workflow:change-specific-plan-ownership", "deduplicate-plan-authoring-and-execution-workflow:authority-resolved-local-detail", "deduplicate-plan-authoring-and-execution-workflow:proportionate-plan-fields", "deduplicate-plan-authoring-and-execution-workflow:precommit-plan-review", "deduplicate-plan-authoring-and-execution-workflow:freshness-scoped-assurance-reuse"]
Paths: ["docs/decisions/deduplicate-plan-authoring-and-execution-workflow.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/plans/README.md", "docs/plans/template.md", ".claude/skills/awf-writing-plans/SKILL.md", ".pi/skills/awf-writing-plans/SKILL.md", ".claude/skills/awf-executing-plans/SKILL.md", ".pi/skills/awf-executing-plans/SKILL.md", ".claude/skills/awf-subagent-driven-development/SKILL.md", ".pi/skills/awf-subagent-driven-development/SKILL.md", ".claude/skills/awf-reviewing-impl/SKILL.md", ".pi/skills/awf-reviewing-impl/SKILL.md", ".claude/agents/implementer.md", ".pi/agents/implementer.md", ".claude/agents/code-reviewer.md", ".pi/agents/code-reviewer.md", ".awf/awf.lock"]

Update and test these current-state claims with the exact behavior implemented in Tasks 2.1 through 2.3:

- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `rendering/workflow-skill-templates:semantic-rendering-review`
- add `rendering/workflow-skill-templates:plan-review-before-first-commit`

Append one Applied event containing exactly those operations and mutate their claim prose and provenance in the same transaction. Add `Backing: test` and named proof markers for the new invariant and preserve or update proof markers for every revised invariant. Run `./x render`, inspect both Claude and Pi plan, implementer, executor, and reviewer outputs for concept-preserving parity and intentional placeholder examples, and require `./x check` to report zero findings.

### Phase close

Stage the complete Phase 2 transaction explicitly and create its closing commit after `awf check staged` and `./x gate` pass:

```commit
refactor(rendering): centralize plan workflow mechanics
```

## Phase 3: Retire resync and complete the single workflow

**Execution mode: subagent-driven.**

Completes: ["resync-retired", "repository-green"]

### Task 3.1: Fold linked-ADR freshness into ordinary review
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness", "deduplicate-plan-authoring-and-execution-workflow:one-workflow-no-depth-controls"]
Paths: ["templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/writing-plans/SKILL.md.tmpl", "templates/partials/review-remediation-autonomy.md", "templates/docs/workflow.md.tmpl", "templates/agents/plan-reviewer.md.tmpl", "internal/project/spine_test.go", "internal/project/docs_sections_test.go", "internal/project/plan_detail_modes_test.go", "internal/evals/chain_test.go", "internal/evals/independent_workflow_escalation_test.go"]

Make full plan review consume the deterministic linked-plan set and verify every ADR in the selected plan's parsed `adrs:` set. A substantive ADR amendment or review correction invalidates earlier review of every linked Proposed plan. Route ADR review to ordinary plan review for the affected linked plans after ADR approval; if implementation already began, inventory affected completed phases and renew their assurance before progression. A plan correction that would contradict linked authority returns to ADR amendment and review first. Remove the special resync mode, narrowed lens branch, return-edge exception, and chain presentation while preserving one fresh verify pass and authority-guided remediation in ordinary plan review.

Update workflow chain and reviewer tests to prove ADR-before-plan ordering, parsed linked-plan selection rather than modification-time/session inference, post-start reassessment, no live resync dispatch, one-review freshness, and publication-safe target parity.

### Task 3.2: Remove the standard skill and migrate existing selections
Latitude: exact
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness", "deduplicate-plan-authoring-and-execution-workflow:one-workflow-no-depth-controls"]
Paths: ["internal/catalog/standard.go", "internal/catalog/graph_test.go", "internal/catalog/catalog_test.go", "internal/catalog/workflow_test.go", "internal/project/catalog_sweep_test.go", "internal/project/target_test.go", "internal/project/subagent_model_selection_test.go", "internal/project/skillrefs_test.go", "internal/project/spine_test.go", "internal/project/project_test.go", "internal/project/scaffold.go", "internal/evals/chain_test.go", "internal/evals/independent_workflow_escalation_test.go", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", ".awf/config.yaml", "cmd/awf/testdata/init-describe.json"]

Remove `reviewing-plan-resync` from the standard catalog, reviewing skill/agent structural requirements, default core selection, project configuration, target fixtures, eval chain, model-selection census, and live reference tests. Delete its template only after all consumers and section declarations no longer reference it. Preserve generic longest-token skill-reference tests using a non-retired overlapping fixture name rather than weakening their boundary. Keep historical ADRs, plans, and research documents byte-identical.

Add schema generation 38 after the current generation 37 in `internal/migrate/migrate.go`. Implement an idempotent config-tree migration that removes exactly `reviewing-plan-resync` from the top-level `skills` sequence before catalog validation, reports the removal once, preserves sequence order and every unrelated byte/value allowed by the config editor, and leaves configs without the selection unchanged. Use the existing config editing owner rather than parsing YAML in the migration. Cover block and flow forms supported by the editor, sole-item/empty result, absence, repeated application, malformed input, change reporting, schema stamping, ordered registry behavior, and forward-port behavior where applicable.

Migration paths: `internal/migrate/migrate.go`, `internal/migrate/changes.go`, a focused `internal/migrate/retireplanresync.go` and `internal/migrate/retireplanresync_test.go`, `internal/migrate/migrate_test.go`, and `internal/migrate/forwardport_test.go`. Do not remove historical structural-heading migration entries needed by projects crossing older generations before generation 38.

### Task 3.3: Apply freshness, retirement, and migration authority
Latitude: exact
Applying: ["deduplicate-plan-authoring-and-execution-workflow:linked-plan-review-freshness", "deduplicate-plan-authoring-and-execution-workflow:one-workflow-no-depth-controls"]
Paths: ["docs/decisions/deduplicate-plan-authoring-and-execution-workflow.md", "docs/decisions/INDEX.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", ".awf/topics/parts/config/migrations-and-locks/current-state.md", ".awf/domains/parts/adr-system/current-state.md", "docs/topics/rendering/workflow-skill-templates.md", "docs/topics/config/migrations-and-locks.md", "docs/domains/adr-system.md", "README.md", ".awf/docs/glossary.yaml", ".awf/docs/pitfalls.yaml", "docs/glossary.md", "docs/pitfalls.md", "docs/workflow.md", "changelog/CHANGELOG.md", "glob:.claude/skills/**", "glob:.pi/skills/**", "glob:.claude/agents/**", "glob:.pi/agents/**", ".awf/awf.lock"]
Post-check: Run `./x render`, then require `./x check` to report zero findings. Run a tracked-file search for `reviewing-plan-resync`, `plan-ADR resync`, and the Unicode plan-to-ADR resync spelling; classify every remaining result as append-only historical ADR/plan/research content or a required historical migration compatibility fixture, and require no live catalog, template, generated workflow, current-state, README, glossary, or changelog reference. Read back every mutation target reported by render and both target families.

Apply these remaining workflow operations in one Applied event with their matching claim mutations:

- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:authority-guided-review-remediation`
- update `rendering/workflow-skill-templates:memory-log-consumer-coverage`
- remove `rendering/workflow-skill-templates:workflow-chain-surfaces-resync`
- add `rendering/workflow-skill-templates:linked-plan-review-freshness`
- add `rendering/workflow-skill-templates:single-workflow-no-depth-controls`
- add `config/migrations-and-locks:retired-plan-resync-selection-migration`

The linked-plan freshness invariant names typed association, ADR-first correction, every affected Proposed plan, and post-start phase reassessment. The single-workflow invariant forbids profiles, depth controls, routers, classifiers, and runtime policy knobs. The migration invariant names pre-validation removal, idempotence, reporting, and preservation of unrelated config. Give each added invariant `Backing: test`, add or update every proof marker, and preserve provenance for updated claims.

Update README, workflow, ADR-system domain prose, glossary, and pitfalls to describe the single chain and freshness rule without rewriting historical records. Add an Unreleased adopter-facing changelog entry covering shorter plans, resync retirement, automatic selection migration, and freshness-scoped assurance. Run `./x render`; confirm the retired generated Claude and Pi skill outputs are pruned, all surviving generated prose is coherent with empty variables, no unresolved token appears, and current-state proofs are complete.

Keep the ADR Implementing and this plan Proposed. After implementation assurance settles, `effort-workflow` will number and integrate the pending ADR, reconcile final deviations into Notes, apply terminal status transitions, remove the managed topology, run retrospective, and finish the effort.

### Phase close

Stage the complete Phase 3 transaction explicitly and create its closing commit after `awf check staged` and `./x gate` pass:

```commit
feat(rendering): retire plan resync workflow
```

## Definition of done

- `dod: linked-plan-query` `awf context` exposes deterministic typed plan links for an explicit ADR request without reparsing Markdown or promoting decision history to current authority.
- `dod: change-specific-plans` New plans state change-specific intent and focused evidence while generic execution, review, gate, recovery, and checkpoint mechanics have one owner outside plan prose; historical plans remain valid.
- `dod: assurance-reuse` Fresh delegated phase-review evidence prevents duplicate review of already-covered correctness while complete-range audits and unreviewed settlement, cross-phase, integration, divergence, and changed-authority effects remain covered.
- `dod: resync-retired` Ordinary plan review owns linked-ADR freshness, the separate resync skill and generated outputs are absent, existing selections migrate automatically, and no workflow profile or depth control exists.
- `dod: repository-green` All declared State changes are Applied while the ADR remains Implementing and the plan remains Proposed, source and generated outputs are synchronized, every phase closes with clean staged checks and a 100% gate, and terminal artifact closure remains deferred until independent assurance settles.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated owners report deviations; the parent supplies each report to phase review and reconciles findings in one focused settlement commit before checkpointing or later execution. Record review findings, implementation deviations, and freshness invalidations here. After assurance settles, `effort-workflow` reconciles final Notes and owns numbering, integration, and the status-only terminal transaction.
