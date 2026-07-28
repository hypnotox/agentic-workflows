---
date: 2026-07-28
adrs: [168]
status: Proposed
---
# Plan: Implement Maintainable Code Design Guidance

## Goal

Implement ADR-0168 by publishing a mandatory, extensible maintainable-code design guide and making
its design, refactor, handoff, and review obligations active across every required workflow stage and
rendered target.

Non-goals: introduce a language-specific coding standard, mandate named patterns, add a separate
maintainability skill, or change reviewer agents from report-only judges.

## Architecture summary

Add the guide as a catalog-derived mandatory plain singleton so the existing output-plan, layout,
convention-part, drift, and document-map machinery owns it without a production special case. Then
integrate short references to that guide into the existing workflow-skill sections, strengthen the
scoped implementer brief, and add maintainability lenses to the existing reviewer-agent sections.
Each independently gated phase applies the matching ADR-0168 V2 claim and renders its current-state
and adopter outputs. The final review-lens phase freezes this plan and completes ADR-0168.

## File structure

Every filesystem path in this plan is exact and rooted at
`/home/hypno/Projects/agentic-workflows`; shorter paths in required rendered prose and Go literals are
output values, not executor path notation.

- **Created:**
  `/home/hypno/Projects/agentic-workflows/templates/docs/maintainable-code-design.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/docs/maintainable-code-design.md`.
- **Modified source, tests, and authority:**
  `/home/hypno/Projects/agentic-workflows/internal/catalog/standard.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/project_test.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/docs_sections_test.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/render_tree_test.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/spine_test.go`,
  `/home/hypno/Projects/agentic-workflows/internal/project/target_test.go`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/proposing-adr/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/writing-plans/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/tdd/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/executing-plans/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/executing-direct/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/subagent-driven-development/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/skills/bugfix/SKILL.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/agents/plan-reviewer.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/agents/code-reviewer.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/templates/agents/adr-reviewer.md.tmpl`,
  `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`,
  `/home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`,
  `/home/hypno/Projects/agentic-workflows/docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md`,
  `/home/hypno/Projects/agentic-workflows/docs/plans/2026-07-28-implement-maintainable-code-design-guidance.md`.
- **Modified generated shared outputs:**
  `/home/hypno/Projects/agentic-workflows/AGENTS.md`,
  `/home/hypno/Projects/agentic-workflows/docs/config-reference.md`,
  `/home/hypno/Projects/agentic-workflows/docs/domains/rendering.md`,
  `/home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md`,
  `/home/hypno/Projects/agentic-workflows/.awf/awf.lock`.
- **Modified generated skills:** the nine exact paths under each target root:
  `awf-brainstorming/SKILL.md`, `awf-proposing-adr/SKILL.md`,
  `awf-refactor-coupling-audit/SKILL.md`, `awf-writing-plans/SKILL.md`, `awf-tdd/SKILL.md`,
  `awf-executing-plans/SKILL.md`, `awf-executing-direct/SKILL.md`,
  `awf-subagent-driven-development/SKILL.md`, and `awf-bugfix/SKILL.md`, rooted respectively at
  `/home/hypno/Projects/agentic-workflows/.claude/skills/` and
  `/home/hypno/Projects/agentic-workflows/.pi/skills/`.
- **Modified generated agents:** `plan-reviewer.md`, `code-reviewer.md`, and `adr-reviewer.md`, rooted
  respectively at `/home/hypno/Projects/agentic-workflows/.claude/agents/` and
  `/home/hypno/Projects/agentic-workflows/.pi/agents/`.
- **Deleted:** none.

## Phase 1: Publish the mandatory design guide

- [ ] **Task 1.1: Publish, register, prove, apply, and commit the mandatory guide as one atomic change.**
  Create
  `templates/docs/maintainable-code-design.md.tmpl` with H1 `# Maintainable Code Design` and these
  exact, ordered `awf:section` identifiers and H2 headings:

  | Section identifier | Heading |
  |---|---|
  | `decision-posture` | `Decision posture` |
  | `contextual-heuristics` | `SOLID, DRY, and YAGNI` |
  | `semantic-modeling` | `Semantic modeling` |
  | `boundaries-and-dependencies` | `Boundaries and dependency direction` |
  | `pattern-toolbox` | `Illustrative pattern toolbox` |
  | `preparatory-refactoring` | `Preparatory refactoring` |
  | `failure-modes` | `Failure modes` |

  Write complete adopter-neutral prose with the following closed content contract:
  - `decision-posture` starts from the requested behavior and surrounding model, prefers cohesive
    ownership and explicit seams, and requires abstraction or indirection to earn its cost for the
    actual change.
  - `contextual-heuristics` treats SOLID, DRY, and YAGNI as questions rather than compliance rules:
    cohesion and dependency direction matter, duplication is assessed by shared policy rather than
    textual similarity, and speculative flexibility is rejected.
  - `semantic-modeling` distinguishes domain meaning from storage, transport, UI, and framework
    shapes; it calls for explicit state, invariants, transitions, and ownership where the behavior
    needs them, without requiring wrapper types mechanically.
  - `boundaries-and-dependencies` keeps policy independent of volatile representations, directs
    dependencies deliberately, localizes translation, minimizes public surface, and creates testable
    seams without test-only distortion.
  - `pattern-toolbox` presents Strategy, Adapter, Facade, State, value objects, repositories, and
    ports-and-adapters as a non-exhaustive vocabulary. For every example, state both the problem it
    can solve and the warning that a direct implementation is preferable when the problem is absent.
  - `preparatory-refactoring` always asks whether the current model can carry the change. Include a
    bounded enabling refactor when it prevents duplicated policy, inappropriate coupling,
    representation leakage, or a workaround. For materially larger work, require the user choice to
    perform it first, include it in the current effort, defer it in a durable project-owned record,
    or decline it with the trade-off stated. Forbid silent scope growth and refactoring manufactured
    only to satisfy a heuristic.
  - `failure-modes` names bolt-on correctness, representation leakage, duplicated policy, dependency
    inversion by accident, speculative abstraction, pattern checklists, test-shaped production
    design, hidden debt, and unbounded cleanup; give a terse correction for each.

  Do not reference this repository's commands, module names, packages, Go-specific constructs, or
  file layout. Use only ASCII punctuation. Verify with
  `go test ./internal/project -run TestMaintainableCodeDesignGuide` after adding the focused test and
  catalog entry below; it must pass.

  **Register and prove the catalog-derived singleton.** In
  `internal/catalog/standard.go`, add this exact entry to `Catalog.Docs` beside the other mandatory
  document-map docs:

  ```go
  "maintainable-code-design": {Mandatory: true, DocumentMap: true, Title: "Maintainable Code Design", Desc: "decision framework for cohesive models, explicit boundaries, dependencies, refactoring, and testable design", Path: "maintainable-code-design.md", TemplateKey: "maintainableCodeDesign", TID: "docs/maintainable-code-design.md.tmpl", Sections: []string{"decision-posture", "contextual-heuristics", "semantic-modeling", "boundaries-and-dependencies", "pattern-toolbox", "preparatory-refactoring", "failure-modes"}},
  ```

  Do not add a branch to `internal/project/output_plan.go`, `internal/project/layout.go`,
  `internal/project/singleton.go`, or `internal/project/scaffold.go`; the new entry must flow through
  their existing catalog projections. Extend `testLayout()` in `internal/project/project_test.go`
  with `"maintainableCodeDesign": "docs/maintainable-code-design.md"`. Update
  `TestAgentsDocDocumentMapListsMandatorySingletonsUnconditionally` so its pinned expected membership
  includes the new entry.

  Add `TestMaintainableCodeDesignGuide` to `internal/project/docs_sections_test.go`. Mark it with
  `// invariant: rendering/guide-and-doc-templates:maintainable-code-design-guide` and have it:
  1. assert the exact catalog metadata and ordered section list above;
  2. assert `render.ParseSections` returns those section markers exactly once and in that order;
  3. render with empty vars/data and `testLayout()`, then call `assertNoLeaks` and assert the H1, every
     H2, `SOLID`, `DRY`, `YAGNI`, `Strategy`, `Adapter`, all four larger-refactor choices, and the
     anti-mechanical warning are present;
  4. reject `./x`, `github.com/hypnotox/agentic-workflows`, `internal/`, `.go`, `Go package`, and any
     unresolved-value or template residue;
  5. scaffold with `skills: []`, `agents: []`, and `docs: []`, sync, and assert both
     `docs/maintainable-code-design.md` and its full title/link/description line in `AGENTS.md` exist.

  Add `TestMaintainableCodeDesignPartOverride` to `internal/project/render_tree_test.go`. Scaffold a
  project with `.awf/parts/maintainable-code-design/decision-posture.md` containing a unique body,
  sync, and assert the unique body replaces only the default `decision-posture` body while the other
  six headings remain. Run
  `go test ./internal/catalog ./internal/project -run 'Test(MaintainableCodeDesign|AgentsDocDocumentMap|UnifiedDocModel|AdrSingletonSectionParity)'`;
  it must pass with no findings.

  **Apply the guide claim and render the first lifecycle batch.** Append this exact claim
  to `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`:

  ```markdown
  ### `invariant: maintainable-code-design-guide`

  The standard catalog renders `docs/maintainable-code-design.md` as a mandatory document-map singleton with ordered convention-part sections for decision posture, contextual heuristics, semantic modeling, boundaries and dependencies, an illustrative pattern toolbox, preparatory refactoring, and failure modes; empty project data remains coherent, adopter-neutral, language-agnostic, and free of repository-specific content.
  Origin: ADR-0168
  Backing: test
  ```

  Immediately before editing ADR-0168, derive `S` as one greater than the highest repository-global
  `state-sequence` currently present under `docs/decisions/`; do not reserve an authoring-time
  literal. In ADR-0168 set frontmatter status to `Implementing`, append the Implementing event with
  content digest `5e6e3b2f3b3b066a5faec3ad1a7d81accd2599ce89546edb2d5f556a371eaa49`, then append an Applied
  event at sequence `S` whose operations field is the verb `add` followed by the qualified ID
  `rendering/guide-and-doc-templates:maintainable-code-design-guide`. Run `./awf check` after
  the edit; it must accept `S` as the next consecutive sequence and report only implementation drift
  that `./x render` will resolve. Run `./x render`; it must complete
  successfully and update `docs/maintainable-code-design.md`, `AGENTS.md`,
  `docs/config-reference.md`, `docs/domains/rendering.md`, `docs/decisions/INDEX.md`, and
  `.awf/awf.lock` without manual generated-file edits. Run `./x check`; it must report clean drift.

  **Verify and commit.** Run:

  ```bash
  git add -- /home/hypno/Projects/agentic-workflows/templates/docs/maintainable-code-design.md.tmpl /home/hypno/Projects/agentic-workflows/internal/catalog/standard.go /home/hypno/Projects/agentic-workflows/internal/project/project_test.go /home/hypno/Projects/agentic-workflows/internal/project/docs_sections_test.go /home/hypno/Projects/agentic-workflows/internal/project/render_tree_test.go /home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md /home/hypno/Projects/agentic-workflows/docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md /home/hypno/Projects/agentic-workflows/docs/maintainable-code-design.md /home/hypno/Projects/agentic-workflows/AGENTS.md /home/hypno/Projects/agentic-workflows/docs/config-reference.md /home/hypno/Projects/agentic-workflows/docs/domains/rendering.md /home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md /home/hypno/Projects/agentic-workflows/.awf/awf.lock
  ```

  Then run `awf check --staged`, followed by `./x gate`; both must exit successfully with zero
  findings. Commit with:

  ```commit
  feat(rendering): add design guide (applies 0168 batch)
  ```

## Phase 2: Integrate the design obligation across workflow stages

- [ ] **Task 2.1: Integrate, prove, apply, render, and commit every stage obligation atomically.**
  Modify only the
  existing sections/bodies below; do not add catalog section identifiers. Every stage points to
  `` `{{ .layout.maintainableCodeDesign }}` `` and summarizes only the action that stage owns:
  - `templates/skills/brainstorming/SKILL.md.tmpl`, existing `design-sections`: require the settled
    design to identify the semantic model and ownership, representation boundaries, dependency
    direction, test seams, and preparatory-refactor decision before approach approval.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`, existing `procedure-write`: when the decision is
    structural, preserve those settled choices, constraints, and enabling work in Context/Decision;
    do not replace them with a pattern name.
  - `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, existing `audit-shape-selection`: assess
    whether the requested behavior would create duplication, coupling, representation leakage, or a
    workaround, and feed the bounded/larger refactor result into ADR scope.
  - `templates/skills/writing-plans/SKILL.md.tmpl`, existing `procedure-write-plan`: convert the
    settled model, boundaries, dependency direction, representation translations, refactor decision,
    prohibited shortcuts, and validation into ordered executable tasks.
  - `templates/skills/tdd/SKILL.md.tmpl`, fixed procedure: before implementing, assess whether a
    bounded enabling refactor is needed to prevent duplication, coupling, representation leakage, or
    a workaround; escalate materially larger work through the four guide dispositions. Select the
    smallest seam that proves the behavior while supporting the intended model, and reject tests that
    force production representation leakage or needless indirection.
  - `templates/skills/executing-plans/SKILL.md.tmpl`, existing `procedure-per-task`: preserve the
    plan's structural choices, reassess when grounded source contradicts them, and stop rather than
    bolt correctness onto the wrong abstraction.
  - `templates/skills/executing-direct/SKILL.md.tmpl`, fixed procedure body: assess bounded enabling
    refactoring before editing, preserve settled boundaries, and return to brainstorming for a larger
    choice rather than silently expanding scope or accepting a workaround.
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl`, existing
    `procedure-extract-context`: preserve and reassess the same structural choices at orchestration
    time; the detailed handoff payload remains Phase 3.
  - `templates/skills/bugfix/SKILL.md.tmpl`, fixed procedure around the root-cause step: assess whether
    the root cause is an unsuitable model or boundary, include bounded enabling work that prevents a
    workaround, and escalate materially larger work through the four user choices.

  Keep all prose target-neutral and language-agnostic. Do not duplicate the guide's pattern list or
  full heuristic explanations. Do not weaken TDD minimality, bugfix one-concern discipline, direct
  execution approval boundaries, or implementation plan-adherence.

  **Add focused stage-coverage tests.** In `internal/project/spine_test.go`, ensure both
  `testLayout()` and `withLayoutDefaults()` make `maintainableCodeDesign` available to direct golden
  renders. Add `TestMaintainableCodeStageCoverage`, marked
  `// invariant: rendering/workflow-skill-templates:maintainable-code-stage-coverage`. Render all nine
  affected skills with empty vars/data and the conditional skill set needed to exercise their active
  branches. Use a table keyed by skill name to assert every output cites
  `docs/maintainable-code-design.md` plus its stage semantics:
  - brainstorming: model, ownership, representations, dependency direction, refactor decision;
  - proposing-adr and coupling audit: structural constraints and enabling work;
  - writing-plans: ordered tasks, prohibited shortcuts, validation;
  - TDD: bounded/larger refactor assessment, model-supporting seam, and no test-induced distortion;
  - executing-plans, executing-direct, subagent-driven development, and bugfix: preserve/reassess,
    bounded enabling refactor, no bolt-on workaround, and escalation when the larger choice applies.

  Call `assertNoLeaks` for every render. Run
  `go test ./internal/project -run 'Test(MaintainableCodeStageCoverage|BrainstormingTemplate|ProposingAdrTemplate|RefactorCouplingAuditTemplate|WritingPlansTemplate|TddTemplate|ExecutingPlansTemplate|ExecutingDirectTemplate|SubagentDrivenDevelopmentTemplate|BugfixTemplate|ManagedContextCallersChooseProjection)'`;
  it must pass.

  **Apply the stage claim and render all target outputs.** Append this exact claim to
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`:

  ```markdown
  ### `invariant: maintainable-code-stage-coverage`

  Brainstorming, ADR proposal, coupling audit, plan writing, test-driven development, inline plan execution, direct execution, subagent-driven development, and bug fixing each render a concise stage-specific obligation pointing to the mandatory maintainable-code guide: designs settle models and boundaries, plans make them executable, and implementation preserves or explicitly reassesses them instead of bolting correctness onto an unsuitable abstraction.
  Origin: ADR-0168
  Backing: test
  ```

  Append the next ADR-0168 Applied event at sequence `S+1`; its operations field is the verb `add`
  followed by the qualified ID
  `rendering/workflow-skill-templates:maintainable-code-stage-coverage`. Run `./awf check`
  after the edit; it must accept the event as the next consecutive sequence.

  Run `./x render`. It must update each of the nine generated skill paths enumerated in the phase-2
  staging command under both absolute target roots, plus `docs/domains/rendering.md`,
  `docs/decisions/INDEX.md`, and `.awf/awf.lock`.
  Run `./x check`; it must report clean drift. Inspect `git diff --check` and
  `git diff -- .claude/skills .pi/skills`; neither may show whitespace errors, unresolved template
  actions, or semantic omissions between targets.

  **Verify and commit.** Run:

  ```bash
  git add -- /home/hypno/Projects/agentic-workflows/templates/skills/brainstorming/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/proposing-adr/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/refactor-coupling-audit/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/writing-plans/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/tdd/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/executing-plans/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/executing-direct/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/subagent-driven-development/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/bugfix/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/internal/project/spine_test.go /home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/workflow-skill-templates/current-state.md /home/hypno/Projects/agentic-workflows/docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-brainstorming/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-proposing-adr/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-refactor-coupling-audit/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-writing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-tdd/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-executing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-executing-direct/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-subagent-driven-development/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-bugfix/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-brainstorming/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-proposing-adr/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-refactor-coupling-audit/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-writing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-tdd/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-executing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-executing-direct/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-subagent-driven-development/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-bugfix/SKILL.md /home/hypno/Projects/agentic-workflows/docs/domains/rendering.md /home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md /home/hypno/Projects/agentic-workflows/.awf/awf.lock
  ```

  Then run `awf check --staged`, followed by `./x gate`; both must exit successfully with zero
  findings. Commit with:

  ```commit
  feat(rendering): integrate design stages (applies 0168 batch)
  ```

## Phase 3: Carry structural constraints into scoped implementer briefs

- [ ] **Task 3.1: Close, prove, apply, render, and commit the scoped handoff contract atomically.**
  In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`, extend the existing
  `procedure-extract-context` list so each implementer prompt carries only task-relevant facts in
  these exact categories: semantic boundary and ownership, external/internal representations and
  their translation point, allowed dependency direction, preparatory-refactor decision, prohibited
  bolt-on shortcuts, and validation expectations. Add explicit instructions that the implementer
  preserves those choices, reports when grounded source invalidates them, and does not replan,
  broaden the task, or perform unrelated cleanup. In `dispatch-conventions`, make this scoped design
  context part of both the Pi `subagent_implement` task and the generic fresh-context implementer
  prompt. Do not alter sequential dispatch, `allowCommits`, commit permission, the four completion
  statuses, or per-task review behavior.

  In `templates/skills/executing-plans/SKILL.md.tmpl`'s existing `procedure-per-task` Ground bullet,
  require the inline orchestrator to extract the same six categories from the plan for the current
  task before editing. This preserves semantic parity for inline execution without pretending that
  inline work dispatches a subagent.

  **Prove scoped content and multi-target semantics.** Add
  `TestMaintainableCodeSubagentContract` to `internal/project/spine_test.go`, marked
  `// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract`. Render
  `subagent-driven-development` once with `targetSubagentTools: true` and once without it, and render
  `executing-plans`. Assert all six context categories appear in both dispatch branches, the
  no-replanning/no-scope-broadening boundary appears, and the existing status values, sequential
  dispatch rule, `allowCommits` on Pi, and report-only per-task review remain present.

  Add `TestMaintainableCodeMultiTargetParity` to `internal/project/target_test.go`. Scaffold exact
  targets `[claude, pi]`, exact skills `[subagent-driven-development]`, and `agents: []`; no catalog
  neighbor is structurally required for this render-only semantic test. Render all outputs and assert:
  - each target's subagent-driven-development skill contains all six handoff semantics and the
    prohibited broadening language, even though dispatch syntax differs;
  - each target emits the affected skill once;
  - `docs/maintainable-code-design.md` is emitted once as a neutral artifact.

  Run
  `go test ./internal/project -run 'Test(MaintainableCodeSubagentContract|MaintainableCodeMultiTargetParity|SubagentDrivenDevelopmentTemplate|ExecutingPlansTemplate|MultiTargetRender)'`;
  it must pass.

  **Apply the handoff claim and render target outputs.** Append this exact claim to
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`:

  ```markdown
  ### `invariant: maintainable-code-subagent-contract`

  Every scoped implementation brief carries only the task-relevant semantic boundaries and ownership, representations and translation points, dependency direction, preparatory-refactor decision, prohibited bolt-on shortcuts, and validation expectations; the implementer preserves those choices or reports invalidating source facts without becoming a second planner, broadening scope, or performing unrelated cleanup, and inline plan execution extracts the same context for its current task.
  Origin: ADR-0168
  Backing: test
  ```

  Append the next ADR-0168 Applied event at sequence `S+2`; its operations field is the verb `add`
  followed by the qualified ID
  `rendering/workflow-skill-templates:maintainable-code-subagent-contract`. Run `./awf check`
  after the edit; it must accept the event as the next consecutive sequence.

  Run `./x render`; it must update executing-plans and subagent-driven-development under both target
  skill trees, plus `docs/domains/rendering.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`. Run
  `./x check`; it must report clean drift.

  **Verify and commit.** Run:

  ```bash
  git add -- /home/hypno/Projects/agentic-workflows/templates/skills/subagent-driven-development/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/templates/skills/executing-plans/SKILL.md.tmpl /home/hypno/Projects/agentic-workflows/internal/project/spine_test.go /home/hypno/Projects/agentic-workflows/internal/project/target_test.go /home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/workflow-skill-templates/current-state.md /home/hypno/Projects/agentic-workflows/docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-subagent-driven-development/SKILL.md /home/hypno/Projects/agentic-workflows/.claude/skills/awf-executing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-subagent-driven-development/SKILL.md /home/hypno/Projects/agentic-workflows/.pi/skills/awf-executing-plans/SKILL.md /home/hypno/Projects/agentic-workflows/docs/domains/rendering.md /home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md /home/hypno/Projects/agentic-workflows/.awf/awf.lock
  ```

  Then run `awf check --staged`, followed by `./x gate`; both must exit successfully with zero
  findings. Commit with:

  ```commit
  feat(rendering): scope maintainability handoffs (applies 0168 batch)
  ```

## Phase 4: Add structural review and complete ADR-0168

- [ ] **Task 4.1: Add, prove, apply, render, and commit the review lenses as the terminal atomic change.**
  Modify
  only the `universal-lenses` sections of these templates; each lens points to
  `` `{{ .layout.maintainableCodeDesign }}` `` and retains the opening and shared-tail report-only
  instructions:
  - `templates/agents/plan-reviewer.md.tmpl`: add `maintainable-design` to check that relevant model,
    ownership, representations, translation boundaries, dependency direction, and test seams are
    explicit; necessary enabling refactors are ordered before dependent behavior, bounded to the
    failure they prevent, and deterministically verifiable; larger refactors carry an explicit
    approved/deferred/declined disposition; needless indirection and pattern mandates are findings.
  - `templates/agents/code-reviewer.md.tmpl`: add `maintainable-design` to check cohesion, coupling,
    dependency direction, representation leakage, duplicated policy, testability, needless
    indirection, conformance to the settled design, and whether the implementation bolted behavior
    onto an unsuitable abstraction or silently broadened refactoring scope.
  - `templates/agents/adr-reviewer.md.tmpl`: add `structural-design` with an explicit condition. Run
    it only when a Decision changes a semantic model, representation, module/package boundary,
    dependency direction, ownership boundary, or comparable structural contract. When active, check
    cohesion, representation isolation, dependency direction, enabling-refactor disposition,
    testable seams, and justification for indirection; when no trigger exists, skip this lens rather
    than manufacturing structural requirements.

  Do not add editing, fixing, committing, or re-review instructions to any reviewer.

  **Prove review coverage, conditionality, and report-only preservation.** Extend
  `TestPlanReviewerAgent`, `TestCodeReviewerAgent`, and `TestAdrReviewerAgent` in
  `internal/project/spine_test.go` with the exact maintainability dimensions above and the rendered
  guide path. Add a focused `TestMaintainableCodeReviewLenses`, marked
  `// invariant: rendering/workflow-skill-templates:maintainable-code-review-lenses`, which renders
  all three agents with empty data and asserts:
  - the plan reviewer requires explicit, ordered, bounded, verifiable structural work;
  - the code reviewer covers all eight ADR-listed dimensions: cohesion, coupling, dependency
    direction, representation leakage, duplication, testability, needless indirection, and settled
    design conformance;
  - the ADR reviewer names every structural trigger and says the lens runs only when one is present;
  - every agent still says `Report-only` and contains no directive to edit, apply a fix, commit, or
    loop a re-review.

  Extend the existing empty-data/report-only golden cases rather than weakening their banned-phrase
  list. Run
  `go test ./internal/project -run 'Test(PlanReviewerAgent|CodeReviewerAgent|AdrReviewerAgent|MaintainableCodeReviewLenses|CatalogTemplateGoldenEmptyData)'`;
  it must pass.

  **Apply the final claim, freeze records, and render the terminal lifecycle transaction.** Append
  this exact claim to
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`:

  ```markdown
  ### `invariant: maintainable-code-review-lenses`

  Plan review checks that structural choices and necessary enabling refactors are explicit, ordered, bounded, approved or durably dispositioned when larger, and verifiable; code review checks cohesion, coupling, dependency direction, representation leakage, duplicated policy, testability, needless indirection, and settled-design conformance; ADR review applies the same structural lens only when a decision changes a semantic model, representation, ownership, module, or package boundary, dependency direction, or comparable structural contract. All reviewer agents remain report-only.
  Origin: ADR-0168
  Backing: test
  ```

  Append the final ADR-0168 Applied event at sequence `S+3`; its operations field is the verb `add`
  followed by the qualified ID
  `rendering/workflow-skill-templates:maintainable-code-review-lenses`. Then append the
  Implemented event with content digest
  `5e6e3b2f3b3b066a5faec3ad1a7d81accd2599ce89546edb2d5f556a371eaa49` and set frontmatter status
  to `Implemented`. Run `./awf check` after the edit; it must accept the final sequence, Applied
  partition, digest, and terminal status.

  Flip this plan's frontmatter `status:` from `Proposed` to `Implemented`. In its Notes section,
  record only concrete execution findings, if any; do not restate the design. Run `./x render`; it
  must update all three reviewer agents under `.claude/agents` and `.pi/agents`, plus
  `docs/domains/rendering.md`, `docs/decisions/INDEX.md`, and `.awf/awf.lock`. Run `./x check`; it must
  report clean drift. Confirm `git grep 'status: Implementing' --
  docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md` returns no output and
  `git grep 'status: Proposed' -- docs/plans/2026-07-28-implement-maintainable-code-design-guidance.md`
  returns no output.

  **Verify and commit.** Run:

  ```bash
  git add -- /home/hypno/Projects/agentic-workflows/templates/agents/plan-reviewer.md.tmpl /home/hypno/Projects/agentic-workflows/templates/agents/code-reviewer.md.tmpl /home/hypno/Projects/agentic-workflows/templates/agents/adr-reviewer.md.tmpl /home/hypno/Projects/agentic-workflows/internal/project/spine_test.go /home/hypno/Projects/agentic-workflows/.awf/topics/parts/rendering/workflow-skill-templates/current-state.md /home/hypno/Projects/agentic-workflows/docs/decisions/0168-make-maintainable-code-design-a-workflow-obligation.md /home/hypno/Projects/agentic-workflows/docs/plans/2026-07-28-implement-maintainable-code-design-guidance.md /home/hypno/Projects/agentic-workflows/.claude/agents/plan-reviewer.md /home/hypno/Projects/agentic-workflows/.claude/agents/code-reviewer.md /home/hypno/Projects/agentic-workflows/.claude/agents/adr-reviewer.md /home/hypno/Projects/agentic-workflows/.pi/agents/plan-reviewer.md /home/hypno/Projects/agentic-workflows/.pi/agents/code-reviewer.md /home/hypno/Projects/agentic-workflows/.pi/agents/adr-reviewer.md /home/hypno/Projects/agentic-workflows/docs/domains/rendering.md /home/hypno/Projects/agentic-workflows/docs/decisions/INDEX.md /home/hypno/Projects/agentic-workflows/.awf/awf.lock
  ```

  Then run `awf check --staged`, followed by `./x gate`; both must exit successfully with zero
  findings. Commit with:

  ```commit
  feat(rendering): add maintainability review lenses (implements 0168)
  ```

## Verification

- Run `go test ./...`; it must exit successfully.
- Run `./x render && ./x check`; render must complete and check must report clean drift.
- Stage the complete final transaction, run `awf check --staged`, then run `./x gate`; both must exit
  successfully with zero findings before the final commit.
- Run `git diff --check` and `git status --short`; the first must produce no output, and the second
  must show no uncommitted files after the final commit.
- Inspect `docs/maintainable-code-design.md`, one Claude skill, the corresponding Pi skill, and all
  six generated reviewer-agent files. The guide must be language-agnostic, target skills must carry
  the same semantic obligations, and reviewers must remain report-only.

## Notes

- The exact section identifiers are the public convention-part API. Do not rename them during
  implementation without returning the Proposed plan to review.
- Let `S` be the next repository-global state sequence at execution start. The four ADR application
  batches use `S`, `S+1`, `S+2`, and `S+3` without reserving stale authoring-time values.
- The existing catalog-derived output and layout paths are sufficient; adding a production special
  case or a schema migration is out of scope and is a plan deviation.
