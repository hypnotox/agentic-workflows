---
date: 2026-07-30
adrs: [179]
status: Proposed
---
# Plan: Rendered explorer and grounding-checker role contracts

## Goal

Execute ADR-0179: move the explorer and grounding-checker child-facing contracts out of the generated
Pi extension's `rolePrompt()` literals into rendered agent artifacts, delete `rolePrompt`, pair the two
skills that dispatch those roles directly, and apply the ADR's six `State changes` operations. The
design and its rationale live in ADR-0179; this file is the execution record only.

Non-goals: extracting the wider Pi dispatch spine (the `run`, `toolResult`, and metadata boilerplate
stays as it is per ADR-0179 item 13), and closing the pre-existing `debugging` and
`refactor-coupling-audit` pairing gap (ADR-0179 item 6 accepts it).

## Architecture summary

Three phases, each one green transaction with one closing commit.

Phase 1 lands both contracts as rendered artifacts and rewires the Pi extension to load them, which is
necessarily one transaction: the extension cannot load an agent that is not yet enabled, and
`rolePrompt` cannot be half-deleted. It applies the ADR's first five operations, in declaration order,
in that single commit.

Phase 2 pairs the two dispatchers and bumps the config schema, which must follow Phase 1 because
`RequiresAgent` validation fails at project open until the named agents exist and are enabled.

Phase 3 applies the sixth operation, a prose-only claim narrowing, and freezes both records. The
descriptive-surface corrections live in Phase 1 instead, because ADR-0179 item 11 requires them in the
same commit as the two new `AgentSpec` entries.

Operation-to-transaction assignment. ADR-0179 declares six operations, and they land in exactly two
batches, because at most one new application batch is legal per transition (`pairOps`,
`internal/currentstate/transition.go:190`). Batch one carries five operations in one `Applied` event at
one sequence, in declaration order, exactly as ADR-0156 lands eleven operations at its sequence 41
(`docs/decisions/0156-rendered-awf-wrapper-replaces-the-co-owned-command-runner.md:198`).

| Batch | Operations | Transaction |
|---|---|---|
| One | add `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`, add `rendering/pi-workflows:pi-role-contract-loader`, update `rendering/pi-workflows:pi-implement-role-artifact`, update `rendering/workflow-skill-templates:bounded-exploration-reporting`, update `rendering/workflow-skill-templates:cross-runtime-exploration-dispatch` | Phase 1 |
| Two | update `rendering/workflow-skill-templates:implementer-role-contract` | Phase 3 |

Phase 2 applies no operation and appends no status event. Phase 1 flips the ADR frontmatter to
`status: Implementing` and appends exactly two history events, the `Implementing` status event then the
one `Applied` batch; `HistoryTransitionValid` (`internal/adr/format.go:133`) accepts nothing else for
that transition. Splitting five operations into five `Applied` events is rejected outright. Phase 3
flips the frontmatter to `status: Implemented` and appends the second `Applied` event followed by the
`Implemented` status event, in that order, which is the only shape the same function accepts from
`Implementing`.

Leaving one operation for batch two is not merely convenient: `OperationProgress`
(`internal/adr/application.go:115`) requires an `Implementing` ADR to have both applied and remaining
operations, so a five-of-six split is what makes the intermediate state legal at all.

## File structure

- **Created:** `templates/agents/explorer.md.tmpl`, `templates/agents/grounding-checker.md.tmpl`,
  `internal/migrate/explorergroundingclosure.go` (registration only; it reuses `applyCloseEnabledSet`).
- **Modified:** `internal/catalog/standard.go`, `internal/catalog/catalog.go`,
  `internal/catalog/catalog_test.go`, `templates/pi/awf-subagents/index.ts.tmpl`,
  `templates/skills/exploring/SKILL.md.tmpl`, `templates/skills/brainstorming/SKILL.md.tmpl`,
  `internal/project/spine_test.go`, `internal/project/target_test.go`,
  `internal/project/render_tree_test.go`, `internal/project/skillrefs_test.go`,
  `internal/project/unused_test.go`,
  `internal/project/context_artifacts_test.go`, `internal/project/project.go`,
  `internal/project/version_test.go`, `internal/migrate/migrate.go`,
  `internal/migrate/dropworkflowtelemetry_test.go`, `internal/migrate/workflowtelemetry_test.go`,
  `internal/migrate/closeenabledset_test.go`, `tools/pi-extension-test/tests/index.test.ts`,
  `.awf/config.yaml`, `examples/sundial/.awf/config.yaml`,
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`,
  `.awf/topics/parts/rendering/pi-workflows/current-state.md`, `README.md`,
  `changelog/CHANGELOG.md`, `docs/decisions/0179-rendered-explorer-and-grounding-checker-role-contracts.md`,
  plus every file `./x render` regenerates. That set includes, and is not limited to, the rendered agents
  and skills, the rendered Pi extension in both trees, `AGENTS.md`, `docs/decisions/INDEX.md`,
  `docs/config-reference.md`, both `awf.lock` files, and the generated current-state documents that carry
  claim prose verbatim: `docs/topics/rendering/workflow-skill-templates.md`,
  `docs/topics/rendering/pi-workflows.md`, and `docs/domains/rendering.md`. Stage from `git status` after
  rendering rather than from this list alone, so a regenerated file is never left behind for
  `awf check --staged` to report as drift.
- **Deleted:** none.

## Phase 1: Both role contracts become rendered artifacts

**Execution mode: inline.** This phase is one independently green coherent implementation transaction.
Checkbox tasks are ordered steps, not transaction boundaries.

Why this is one transaction and not two: the Pi extension's explore and grounding call sites are
rewired to load `.pi/agents/explorer.md` and `.pi/agents/grounding-checker.md`, which only exist once
those agents are enabled and rendered, and the loader fails closed when they are absent. Splitting
agent-creation from wiring would leave one commit whose Pi extension throws on every exploration and
grounding dispatch.

- [ ] **Task 1.1: Create `templates/agents/explorer.md.tmpl`.** The body reuses the current
  `rolePrompt("explore")` sentences verbatim, minus the two per-call interpolations and minus the Pi
  limiter sentence, so the existing pinned literals move rather than change. Do not add backticks around
  `targeted`, `bounded`, `broad`, `paths`, `summary`, or `analysis`: Task 1.9 moves the current
  unbackticked spellings into the explorer wants slice verbatim, so a backtick here silently breaks that
  assertion. (This is not a tool-agnostic constraint:
  `rendering/workflow-skill-templates:skill-prose-tool-agnostic` targets `write`, `edit`, and `read`
  only, in three forms, backticked, "via <verb>", and "<verb> tool/call"
  (`internal/project/tool_agnostic_test.go:36-38`), and none of those words appear in this body at all.)
  Carry no decision-record citation in any comment (`TestTemplateSourceResidue` forbids it). Exact
  content:

  ```
  # explorer

  You are a fresh-context exploration subagent dispatched to satisfy one information need. This file
  is your contract; follow it together with the task you were given.

  <!-- awf:section identity -->
  ## Identity

  You are a fresh-context exploration subagent. Read files and run evidence-producing commands only. This is report-only: do not edit files or commit.
  <!-- awf:end -->

  <!-- awf:section single-need -->
  ## One information need

  Handle exactly one information need. Do not bundle unrelated questions and do not recursively delegate.

  The parent may run independent information needs concurrently as separate calls, but refinement of an earlier result stays sequential.
  <!-- awf:end -->

  <!-- awf:section breadth -->
  ## Breadth

  Breadth is ordered targeted < bounded < broad. targeted locates one declaration, implementation, file, or exact fact; bounded investigates within a named symbol, package, component, or subsystem; broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts.

  Treat the selected breadth as an adaptive maximum: start with the cheapest targeted lookup, widen only when evidence requires it, and never widen beyond the selected maximum. If the boundary is exhausted, report that explicitly.

  For broad searches, the project search universe is tracked files plus non-ignored untracked working-tree files under the current repository root. Include tracked generated and vendored files. Exclude ignored files, .git, nested repositories, and external dependencies unless the task explicitly brings one of those surfaces into scope.
  <!-- awf:end -->

  <!-- awf:section report-detail -->
  ## Report detail

  Report detail is ordered paths < summary < analysis and is independent of breadth. paths returns only relevant file:line or file:start-end locations with minimal labels and no search narrative; summary returns grounded locations plus concise explanations of what each contains and why it matters; analysis directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty.
  <!-- awf:end -->

  <!-- awf:section grounding-and-outcomes -->
  ## Grounding and outcomes

  Ground every material claim with file:line evidence.

  Distinguish not-found, inconclusive, and unverified outcomes. Not-found is successful execution and begins exactly: Not found within <breadth> boundary: <what was searched>. A broad absence report must name the project search universe and searched surfaces. A not-found result may suggest one concise next refinement. An inconclusive or unverified result is not an absence claim.
  <!-- awf:end -->

  <!-- awf:section report-discipline -->
  ## Report discipline

  Return only the relevant final report, never the search narrative or intermediate activity.

  Retain no search session or state. After a not-found, inconclusive, unverified, or insufficient report, the parent may issue a new fresh-context call that corrects the task, changes report detail, or widens breadth.
  <!-- awf:end -->
  ```

  The template uses no `.data` key and contains no `{{ if }}` conditional. That is deliberate: a data
  key would force a `dataKeys` entry in `internal/configspec`, and a conditional would force an
  `unsetFallbackCases` entry in `internal/project/spine_test.go`, and nothing in this contract is
  project-specific enough to warrant an adopter override. If a later change introduces either, both
  obligations reattach.

- [ ] **Task 1.2: Create `templates/agents/grounding-checker.md.tmpl`.** The body reuses the current
  `rolePrompt("grounding")` sentences plus the five child-obligation bullets migrating from
  `templates/skills/brainstorming/SKILL.md.tmpl:49-53`. No Go test pins any of this prose (verified:
  the only proofs of `rendering/pi-workflows:pi-dedicated-grounding-dispatch` at
  `internal/project/output_plan_test.go:197` and `internal/project/target_test.go:549` assert tool-name
  routing, not persona text), so the bullets are reworded from parent-facing instructions into
  child-facing obligations. Same no-data, no-conditional constraint as Task 1.1. Exact content:

  ```
  # grounding-checker

  You are a fresh-context grounding-check subagent dispatched to test one agreed design against the
  repository. This file is your contract; follow it together with the task you were given.

  <!-- awf:section identity -->
  ## Identity

  You are a fresh-context grounding-check subagent. Read and run evidence-producing commands, but do not edit files or commit.

  You do not see the conversation that produced the design. Work only from the brief you were given. If that brief names a working-memory file for context, read it if you need to, but never edit it.
  <!-- awf:end -->

  <!-- awf:section verification-scope -->
  ## What to verify

  Verify the supplied design's factual premises against source and architecture: do the named types, functions, and packages exist, and does the approach fit the project's architecture as described?

  Surface unstated assumptions and edge cases the design glossed over.

  Assess whether the effort needs a decision record, a plan, or narrower scope.

  Check convention fit: does the design contradict a current-state claim, an Accepted or Implemented decision record, or an invariant in the project's agent guide?
  <!-- awf:end -->

  <!-- awf:section return-schema -->
  ## What to return

  Return findings only as {kind: open-question | possible-issue, topic, detail, grounding, confidence: verified | interpreted | unverified}.

  Ground each finding with file:line evidence. The confidence field is load-bearing: verified means the claim was mechanically confirmed against source; interpreted means the reading requires judgment; unverified means the claim could not be confirmed.

  This pass is advisory and single-pass. Report findings; never gate, rewrite, or commit.
  <!-- awf:end -->
  ```

- [ ] **Task 1.3: Register both `AgentSpec` entries in `internal/catalog/standard.go`.** Append both
  entries to the `Agents: map[string]AgentSpec{` literal at `:118`, after the existing `implementer`
  entry. The map is not alphabetical (it reads `adr-reviewer`, `plan-reviewer`, `code-reviewer`,
  `implementer`), so append rather than trying to slot them in by name. Each `Sections` list must match
  its template's `awf:section` marker names exactly, in order. Both omit `RequiresSkills` and `Data`, per
  Tasks 1.1 and 1.2. Exact content:

  ```go
  "explorer": {
      Name:        "explorer",
      Description: "Fresh-context exploration subagent for {{ .prefix }} repository questions, handling one information need under a selected breadth and report detail.\nReturns a grounded report only.",
      Sections:    []string{"identity", "single-need", "breadth", "report-detail", "grounding-and-outcomes", "report-discipline"},
  },
  "grounding-checker": {
      Name:        "grounding-checker",
      Description: "Fresh-context grounding-check subagent for {{ .prefix }} designs, testing factual premises, assumptions, altitude, and convention fit against the repository.\nReturns advisory findings only.",
      Sections:    []string{"identity", "verification-scope", "return-schema"},
  },
  ```

- [ ] **Task 1.4: Enable both agents in both config trees.** Add `explorer` and `grounding-checker` to
  the `agents:` list in `.awf/config.yaml` and in `examples/sundial/.awf/config.yaml`, preserving each
  list's existing ordering convention. This must happen in this phase, not with Phase 2's migration:
  the rewired extension in Task 1.6 loads the rendered agent files, which only render when enabled.

- [ ] **Task 1.5: Add a golden test per agent in `internal/project/spine_test.go`.** The functions must
  be named exactly `TestExplorerAgent` and `TestGroundingCheckerAgent`:
  `TestEveryCatalogArtifactHasGoldenTest` (`internal/project/catalog_sweep_test.go:140-155`) greps
  `spine_test.go` for the literal string `func Test<CamelCasedAgentName>Agent(`, and
  `TestNoOrphanGoldenTest` (`:170`) enforces the reverse direction. Model both on `TestImplementerAgent`
  (`spine_test.go:277`): render via `renderAgentGolden(t, "<name>", data)` with `data` of
  `{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}}`, assert `name: <agent>`
  appears, then assert one representative phrase per section so a dropped section fails loudly. For
  `explorer` those phrases are `"Handle exactly one information need"`, `"refinement of an earlier
  result stays sequential"`, `"Breadth is ordered targeted < bounded < broad"`, `"paths < summary <
  analysis"`, `"Ground every material claim with file:line evidence"`, and `"Retain no search session or
  state"`. For `grounding-checker` they are `"do not edit files or commit"`, `"Work only from the brief
  you were given"`, `"Surface unstated assumptions"`, `"advisory and single-pass"`, and
  `"open-question | possible-issue"`.

  `TestExplorerAgent` additionally asserts the ABSENCE of `"Selected breadth maximum"` and `"at most ten
  active exploration children"`, which is what proves the new claim's clause that no rendered body
  carries per-call or runtime-specific text. Without these negative assertions that clause has no
  backing.

- [ ] **Task 1.6: Rewire both Pi call sites and delete `rolePrompt`.** In
  `templates/pi/awf-subagents/index.ts.tmpl`:
  1. Add two path constants beside `IMPLEMENTER_PATH`: `EXPLORER_PATH = ".pi/agents/explorer.md"` and
     `GROUNDING_CHECKER_PATH = ".pi/agents/grounding-checker.md"`, matching the existing constant's
     spelling and placement.
  2. Add two loaders beside `loadImplementer`, each supplying all four `ContractSource` fields
     (`relative`, `noun`, `repair`, `prepend`); `prepend` is required and interpolated unconditionally at
     `:225`, so neither may omit it:

     ```ts
     function loadExplorer(deps: ExtensionDependencies, root: string): Promise<string> {
       return loadAgentContract(deps, root, {
         relative: EXPLORER_PATH,
         noun: "explorer",
         repair: "Enable the explorer agent and run awf render.",
         prepend: "You are the governed exploration subagent. You are report-only: never edit or commit.",
       });
     }

     function loadGroundingChecker(deps: ExtensionDependencies, root: string): Promise<string> {
       return loadAgentContract(deps, root, {
         relative: GROUNDING_CHECKER_PATH,
         noun: "grounding-checker",
         repair: "Enable the grounding-checker agent and run awf render.",
         prepend: "You are the governed grounding-check subagent. You are report-only: never edit or commit.",
       });
     }
     ```
  3. At the grounding call site (currently `:695`), replace `rolePrompt("grounding")` with a contract
     loaded before the `run` call. `deps` and `root` are already in scope there, as the implement site
     at `:782` demonstrates. The grounding call appends no per-call suffix:

     ```ts
     const contract = await loadGroundingChecker(deps, root);
     return toolResult("grounding", params.task, await run("grounding", params.task, GROUNDING_TOOLS, contract, selected.model, metadata, signal, onUpdate, queuedAt), metadata);
     ```
  4. At the explore call site (currently `:723`), replace the `rolePrompt("explore", {...})` argument
     with the loaded body plus the three per-call suffix lines, joined by `"\n"` in this order. The
     limiter sentence must remain a literal in this file: `target_test.go:298`
     (`TestPiStructuredExplorationContractRender`) searches the rendered extension for `queues the rest
     FIFO with abort-aware removal`, and keeping it here is what leaves that test untouched.

     ```ts
     const contract = [
       await loadExplorer(deps, root),
       `Selected breadth maximum: ${params.breadth}`,
       `Selected report detail: ${params.detail}`,
       "Pi admits at most ten active exploration children and queues the rest FIFO with abort-aware removal.",
     ].join("\n");
     ```
  5. Delete the entire `rolePrompt` function (`:174-201`) and its now-unused `role` parameter union.
     Both callers are gone after steps 3 and 4, so nothing references it. Delete the
     `ExplorationBreadth` and `ExplorationDetail` type aliases (`:76-77`) in the same step: their only
     reference in the file is `rolePrompt`'s signature at `:174`, and the tool's own parameters are typed
     by `StringEnum([...])` at `:708-709`, so retaining them would leave two unreferenced aliases in
     generated output that nothing flags (`tsconfig` sets no `noUnusedLocals`).

  Verify with `grep -n "rolePrompt\|ExplorationBreadth\|ExplorationDetail"
  templates/pi/awf-subagents/index.ts.tmpl`, which must return no output.

- [ ] **Task 1.7: Name both agents in both dispatching skills.** In
  `templates/skills/exploring/SKILL.md.tmpl:31`, inside the `{{ else }}` (non-Pi) branch only, change
  `Dispatch one target-native fresh-context exploration subagent with task, breadth, detail, boundary,
  outcome, and report contracts in its brief.` to `Dispatch the `explorer` agent as one target-native
  fresh-context exploration subagent with task, breadth, detail, boundary, outcome, and report
  contracts in its brief.` This is an addition: the generic phrase `target-native fresh-context
  exploration subagent` must survive verbatim, because it is pinned at `target_test.go:430` and `:491`
  and `spine_test.go:939` and `:1459`, including in the `unsetFallbackCases` table. Do not touch the
  `{{ if .targetSubagentTools }}` branch.

  In `templates/skills/brainstorming/SKILL.md.tmpl`, in the `{{ else }}` branch of the
  `grounding-check-output-format` section, change `dispatch ONE fresh-context subagent for exploration`
  to `dispatch the `grounding-checker` agent as ONE fresh-context subagent for exploration`. Leave the
  section name and its parent-facing paragraphs (brief synthesis, surface-findings, advisory
  single-pass) in place.

  Both additions need assertions, or the revised `cross-runtime-exploration-dispatch` claim and the new
  claim's skill-naming sentence are unbacked. `TestCrossRuntimeExplorationDispatch` currently asserts
  only `target-native fresh-context exploration subagent` for the non-Pi case
  (`internal/project/target_test.go:430`); add the literal `` `explorer` agent `` to that same non-Pi
  wants slice. Add a brainstorming counterpart in the same test asserting `` `grounding-checker` agent ``
  appears in its non-Pi render. Keep both assertions on the non-Pi shape only, matching the narrowed
  claim sentence; do not assert either name under `targetSubagentTools`.

- [ ] **Task 1.8: Migrate the five child-obligation bullets out of brainstorming.** Delete
  `templates/skills/brainstorming/SKILL.md.tmpl:49-53` (the five bullets, now carried by Task 1.2's
  template) and rewrite the `:48` lead-in `Ask the subagent specifically to:` so it does not dangle
  above nothing. Replacement: `The child's rendered contract carries its verification obligations and
  return schema; the brief supplies only the design to test.` Keep the following paragraphs, which are
  parent-facing.

- [ ] **Task 1.9: Revise `TestBoundedExplorationReporting` for the moved prose.** In
  `internal/project/target_test.go:454-490`, the `contracts` map currently holds two entries, `"Pi
  fixed prompt"` (whose body is the rendered Pi extension) and the exploring-skill entry, each with its
  own `wants` slice carrying target-specific spellings. Split it three ways:
  1. Keep the exploring-skill entry and its `wants` unchanged; its spellings (`Ground every material
     claim with file/line evidence`) differ from the Pi list's and are not affected.
  2. Rename `"Pi fixed prompt"` to `"Pi per-call suffix"` and reduce its `wants` to exactly the four
     literals that remain in the extension: `"at most ten active exploration children"`, `"queues the
     rest FIFO with abort-aware removal"`, `"Selected breadth maximum:"`, and `"Selected report
     detail:"`. The label must stop claiming a fixed prompt, since that phrasing is precisely what the
     `bounded-exploration-reporting` update retires.
  3. Add a third entry `"explorer agent"` whose body is `renderAgentGolden(t, "explorer", ...)` and
     whose `wants` are the remaining literals moved out of the old Pi list verbatim: `"independent
     information needs concurrently"`, `"refinement of an earlier result stays sequential"`, `"Breadth
     is ordered targeted < bounded < broad"`, `"targeted locates one declaration, implementation, file,
     or exact fact"`, `"bounded investigates within a named symbol, package, component, or
     subsystem"`, `"broad searches across the project search universe, including relevant source,
     tests, documentation, decisions, and workflow artifacts"`, `"adaptive maximum: start with the
     cheapest targeted lookup, widen only when evidence requires it, and never widen beyond the
     selected maximum"`, `"If the boundary is exhausted, report that explicitly"`, `"tracked files plus
     non-ignored untracked working-tree files under the current repository root"`, `"tracked generated
     and vendored files"`, `"ignored files"`, `".git"`, `"nested repositories"`, `"external
     dependencies unless the task explicitly brings one of those surfaces into scope"`, `"paths <
     summary < analysis"`, `"paths returns only relevant file:line or file:start-end locations with
     minimal labels and no search narrative"`, `"summary returns grounded locations plus concise
     explanations of what each contains and why it matters"`, `"analysis directly answers the task with
     an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and
     uncertainty"`, `"Ground every material claim with file:line evidence"`, `"Not-found is successful
     execution and begins exactly: Not found within <breadth> boundary: <what was searched>"`, `"broad
     absence report must name the project search universe and searched surfaces"`, `"not-found result
     may suggest one concise next refinement"`, `"inconclusive or unverified result is not an absence
     claim"`, and `"new fresh-context call that corrects the task, changes report detail, or widens
     breadth"`.

  The fixture at `target_test.go:455` renders with `agents: []`; add `explorer` to it. Note the reason:
  step 3's `renderAgentGolden` renders the template directly and does not read that fixture, so this edit
  is not what makes step 3 work. It is required because Phase 2's `RequiresAgent` pairing makes the
  fixture fail project open without it, and doing it here keeps this test green across both phases.

- [ ] **Task 1.10: Add the TypeScript behaviour seam for both new roles.** In
  `tools/pi-extension-test/tests/index.test.ts`, add one test per role modelled exactly on `"the
  implementation role loads its contract from the rendered agent and fails closed without it"` at
  `:768`: a present-document case asserting the call does not fail, an `ENOENT` case asserting the
  rejection matches `/Missing Pi explorer \.pi\/agents\/explorer\.md\. Enable the explorer agent and
  run awf render\./` (and the grounding-checker equivalent), and a whitespace-body case asserting
  `/has no instruction body; run awf render\./`. Add one further assertion the implementer test has no
  need for: for `subagent_explore`, assert `h.requests[0].systemPrompt` contains both the loaded body
  text and `Selected breadth maximum:`, proving the per-call suffix is appended rather than replacing
  the contract. This seam is required because it caught four real mutations in the Part A work that
  Go-side source pins missed.

- [ ] **Task 1.11: Author the two new claims and update the three existing ones.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, append a new claim block
  after the file's last claim:

  ```
  ### `invariant: explorer-and-grounding-role-contracts`

  The rendered explorer body defines its report-only identity, one information need with no bundling or recursive delegation, concurrent independent needs with sequential refinement, breadth ordered targeted < bounded < broad as an adaptive maximum with its project search universe, report detail ordered paths < summary < analysis independent of breadth, file:line grounding, the distinction between not-found, inconclusive, and unverified outcomes with the exact not-found opening, final-report-only output, and statelessness across calls. The rendered grounding-checker body defines its report-only identity, that it works only from its brief and never edits the working memory that brief may name, its verification obligations across factual premises, unstated assumptions, altitude, and convention fit, and a closed finding schema whose confidence field distinguishes verified, interpreted, and unverified. The exploring and brainstorming skills each name their dispatched agent in the branch that dispatches a target-native subagent, and neither rendered body carries per-call or runtime-specific text.
  Origin: ADR-0179
  Backing: test
  ```

  The third sentence is deliberately scoped to the target-native branch, not to "every dispatch branch".
  ADR-0179 item 5 and Task 1.7 add the agent name only inside each skill's `{{ else }}` branch; the
  `{{ if .targetSubagentTools }}` branch names the Pi tool and no agent. The implementer precedent reads
  more broadly only because `implementer` sits outside the conditional
  (`templates/skills/executing-plans/SKILL.md.tmpl:31`), which is what lets `TestImplementerAgent`
  assert it under both capability shapes. Claiming both branches here while editing one would reintroduce
  precisely the over-broad-claim defect ADR-0179 item 9 exists to correct.

  In the same file, replace `bounded-exploration-reporting`'s prose (`:7`) with text that stops
  claiming a Pi fixed prompt, and append `ADR-0179` to a new `Revised-by:` line (it currently has
  none, so insert `Revised-by: ADR-0179` between `Origin:` and `Backing:`, matching the field order at
  `:19-21`):

  ```
  The rendered exploration guidance and the rendered explorer agent define adaptive breadth and grounded reporting, keep refinement sequential, and permit independent information needs to run concurrently, while Pi's per-call suffix supplies the selected breadth and report detail and makes Pi queue above ten active children in FIFO and abort-aware order.
  ```

  Replace `cross-runtime-exploration-dispatch`'s prose with text admitting the agent name, and append
  `ADR-0179` to its `Revised-by:` line (creating the line if absent):

  ```
  The core exploring skill renders for every target with one semantic breadth-and-detail protocol; the Pi target uses its awf-owned subagent_explore tool while non-Pi targets are directed to the named explorer agent as a generic target-native fresh-context exploration subagent, with no Pi tool name leaking into their output.
  ```

  In `.awf/topics/parts/rendering/pi-workflows/current-state.md`, append the loader claim:

  ```
  ### `invariant: pi-role-contract-loader`

  The generated Pi extension loads every dispatched role's contract from its rendered agent artifact through one shared loader that reads the file, strips frontmatter, prepends the role's per-call authority line, and fails with an actionable enable-and-render repair naming that role on a missing file or an empty instruction body. No dispatched role's prose remains inline in the extension.
  Origin: ADR-0179
  Backing: test
  ```

  And narrow `pi-implement-role-artifact` (`:85`) to its implement-specific remainder, appending
  `ADR-0179` to a `Revised-by:` line:

  ```
  The generated Pi extension builds the implementation child's role prompt from the rendered implementer agent at its `.pi/agents/` path, prepending the commit-authority role line for the call's mode. The before-and-after git snapshot fails a commit-capable implementation call whose HEAD is unchanged, naming the required stopped inventory, and retains the existing commit-forbidden violation, its message, cancellation, cleanup, and bounded-diagnostic reporting.
  ```

  Place a proof marker for each of the two new claims. `// invariant:
  rendering/workflow-skill-templates:explorer-and-grounding-role-contracts` goes on Task 1.5's two
  golden tests.

  `rendering/pi-workflows:pi-role-contract-loader` needs a new Go test, unconditionally: `testGlobs` is
  `**/*_test.go` (`.awf/config.yaml:51-52`), so Task 1.10's TypeScript seam cannot carry a proof marker
  at all, and `TestPiImplementRoleArtifact` (`internal/project/target_test.go:765`) asserts only the
  implementer role. Add to `internal/project/target_test.go`:

  ```go
  // invariant: rendering/pi-workflows:pi-role-contract-loader
  func TestPiRoleContractLoader(t *testing.T) {
      body := renderPiExtensionFile(t, "awf-subagents/index.ts")
      for _, want := range []string{
          "loadExplorer", "loadGroundingChecker",
          ".pi/agents/explorer.md", ".pi/agents/grounding-checker.md",
          "Enable the explorer agent and run awf render.",
          "Enable the grounding-checker agent and run awf render.",
      } {
          if !strings.Contains(body, want) {
              t.Errorf("Pi extension missing loader element %q", want)
          }
      }
      for _, banned := range []string{
          "You are a fresh-context exploration subagent. Read files",
          "You are a fresh-context grounding-check subagent. Read and run",
      } {
          if strings.Contains(body, banned) {
              t.Errorf("role prose survived inline in the extension: %q", banned)
          }
      }
  }
  ```

  The banned literals mirror the survived-prose guard already at `target_test.go:781`, and are the
  opening clauses of the two bodies as they exist in `rolePrompt` today, so they fail loudly if a branch
  is left behind. Confirm with `./x check`, which fails both on a claim with no matching proof and on a
  proof naming an unknown claim.

- [ ] **Task 1.12: Flip the ADR to `Implementing` and append batch one.** Two edits to
  `docs/decisions/0179-rendered-explorer-and-grounding-checker-role-contracts.md`, both required in
  this same transaction:
  1. Change the frontmatter `status: Proposed` to `status: Implementing`. Without it `validateV2History`
     fails with "latest Status history status Implementing does not match frontmatter status Proposed"
     (`internal/adr/format.go:360`) and `OperationProgress` fails with "status Proposed cannot have
     applied operations" (`internal/adr/application.go:110`).
  2. Append exactly two lines to `## Status history`: the `Implementing` status event, then ONE
     `Applied` event carrying all five batch-one operations comma-separated at a single sequence, in the
     ADR's declaration order. Five separate `Applied` lines are rejected by `pairOps` with "appends 5
     application batches; at most one new batch is allowed per transition".

  Neither the content digest nor the state sequence may be precomputed here, because the sequence moves
  whenever any sibling ADR applies a batch. Obtain both with the documented method (`docs/pitfalls.md`,
  "An ADR's frozen digest and next state-sequence cannot be read directly"): write the digest as 64
  zeros, run `./x check` and read the computed digest from the failure, correct it, then let the
  duplicate-sequence error name the next free sequence. The digest excludes `Status history`, so the same
  value repeats on this `Implementing` event and Phase 3's `Implemented` event. Exact shape, with the two
  bracketed values substituted:

  ```
  - 2026-07-30: Implementing; content-sha256: <computed>
  - 2026-07-30: Applied; state-sequence: <next>; operations: add `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`, add `rendering/pi-workflows:pi-role-contract-loader`, update `rendering/pi-workflows:pi-implement-role-artifact`, update `rendering/workflow-skill-templates:bounded-exploration-reporting`, update `rendering/workflow-skill-templates:cross-runtime-exploration-dispatch`
  ```

- [ ] **Task 1.13: Correct the three README enumerations.** ADR-0179 item 11 requires these to land in
  the same commit as the two new `AgentSpec` entries, which Task 1.3 adds in this phase; `README.md:249`
  becomes false the moment Task 1.3 lands, so deferring it would ship a knowingly-stale surface between
  commits. ADR-0177 item 8 set the precedent and its own plan honoured it in the same phase.

  Each site gets an exact replacement. The rule is to drop the count rather than increment it, because a
  bumped number is what ADR-0177 got wrong and the next agent would falsify it again.

  `README.md:12`, currently `retrospective; independent review agents that read each artifact with fresh
  context; a`, becomes:

  ```
  retrospective; dispatched agents that read or implement with fresh context, reviewers among them; a
  ```

  `README.md:46-48`, currently opening `- **Agents**, likewise per runtime. Three review agents
  (`adr-reviewer`, `plan-reviewer`, `code-reviewer`) are each dispatched with fresh context, so the
  author never grades its own work, and are report-only. One `implementer` agent carries the contract
  for`, becomes:

  ```
  - **Agents**, likewise per runtime. The review agents (`adr-reviewer`, `plan-reviewer`,
    `code-reviewer`) are each dispatched with fresh context, so the author never grades
    its own work, and are report-only. The `explorer` and `grounding-checker` agents are
    report-only too. The `implementer` agent carries the contract for
  ```

  Preserve whatever text follows on the original line after "carries the contract for"; only the
  enumeration ahead of it changes.

  `README.md:249`, currently `` `adr-lifecycle`, and `exploring`) and four agents. ``, becomes:

  ```
  `adr-lifecycle`, and `exploring`) and every catalog agent.
  ```

  Two surfaces need no edit and must not be touched reflexively: `internal/configspec`'s agents-key
  description carries no count, and `cmd/awf/main.go`'s package comment names artifact kinds without
  counting them.

- [ ] **Task 1.14: Render, then verify the whole phase.** Run `./x render`, then `./x check`, which must
  print `awf check: clean`. Expect the render to rewrite the rendered agents, both skills, the Pi
  extension in both trees, `AGENTS.md`, `docs/config-reference.md`, `docs/decisions/INDEX.md` (the status
  flip from Task 1.12 changes it), `docs/topics/rendering/workflow-skill-templates.md`,
  `docs/topics/rendering/pi-workflows.md`, `docs/domains/rendering.md`, and both `awf.lock` files. Run
  `./x gate` early rather than only at the phase close: the `AgentSpec` and template guards fail fast and
  each costs a round trip.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction by path; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(rendering): render the explorer and grounding-checker contracts
```

## Phase 2: Pair the dispatchers and bump the config generation

**Execution mode: inline.** This phase is one independently green coherent implementation transaction.
Checkbox tasks are ordered steps, not transaction boundaries.

This phase must follow Phase 1: `RequiresAgent` is hard validation that fails at project open, so the
pairing cannot land before the agents it names exist and are enabled.

- [ ] **Task 2.1: Pair the two direct dispatchers.** In `internal/catalog/standard.go`, add
  `RequiresAgent: "explorer"` to the `"exploring"` spec (`:49`) and `RequiresAgent:
  "grounding-checker"` to the `"brainstorming"` spec (`:10`). Field order follows the paired specs at
  `:23` and `:29`, where `RequiresAgent` precedes `RequiresSkills`: `brainstorming` becomes `Core,
  RequiresAgent, RequiresSkills, Sections` and `exploring`, which carries no `RequiresSkills`, becomes
  `Core, RequiresAgent, Sections`. Do not touch `debugging` (`:45`) or `refactor-coupling-audit`
  (`:107`): they consume exploring transitively and ADR-0179 item 6 accepts that gap.

- [ ] **Task 2.2: Admit both pairings to the dispatcher allowlist.** In
  `internal/catalog/catalog_test.go`, add two entries to `nonReviewingDispatchers` (`:107-110`):
  `"exploring": "explorer"` and `"brainstorming": "grounding-checker"`. Without them
  `TestReviewingSkillSpecsArePaired` errors at `:123` for any non-`reviewing-` skill carrying
  `RequiresAgent`. The allowlist is deliberately closed rather than a blanket exemption, so this
  addition is required rather than incidental.

- [ ] **Task 2.3: Reword the `RequiresAgent` doc comment.** In `internal/catalog/catalog.go:57-61`,
  the comment enumerates the field's users as "a reviewer for the reviewing skills, the implementer for
  the plan-execution skills". Replace that parenthetical enumeration with a generic phrase such as "the
  agent each dispatching skill sends work to", so the next pairing does not have to touch it. Keep the
  surrounding explanation of why the pairing must be loud. Carry no decision-record citation into a
  template; this is a Go source comment, so an existing citation there may stay as it is.

- [ ] **Task 2.4: Add the paired agent to every affected fixture.** This is a batch task. Each site
  enables `exploring` or `brainstorming` and must gain the paired agent in its `agents:` list, or
  `Open` fails. Exact representative, `internal/project/render_tree_test.go:115`:

  ```
  -	cfg := "prefix: example\n" + debuggingVars + "skills: [debugging, exploring]\nagents: []\n"
  +	cfg := "prefix: example\n" + debuggingVars + "skills: [debugging, exploring]\nagents: [explorer]\n"
  ```

  Exact edge, `internal/project/target_test.go:367`, whose list is already populated and needs both
  agents because `explorationFixtureConfig` enables both skills:

  ```
  -agents: [adr-reviewer, code-reviewer, implementer, plan-reviewer]
  +agents: [adr-reviewer, code-reviewer, explorer, grounding-checker, implementer, plan-reviewer]
  ```

  Exhaustive affected-site set, eight edits: `internal/project/render_tree_test.go:115` and `:141` (both
  `[explorer]`), `internal/project/skillrefs_test.go:88` (`[explorer]`),
  `internal/project/context_artifacts_test.go:260` (`[explorer]`),
  `internal/project/unused_test.go:63` and `:175` (both `[explorer]`; each enables `exploring` and
  `refactor-coupling-audit` with local sidecars only for `agents-doc` and `workflow`, so `exploring`
  itself is a catalog skill and does require its agent), `internal/project/spine_test.go:1704`
  (`[explorer]` only: its `brainstorming` carries a `local: true` sidecar and is therefore exempt, but the
  `exploring` in the same list is not), and `internal/project/target_test.go:367` (both agents, per the
  edge above). `internal/project/target_test.go:455` is already given `[explorer]` by Task 1.9, so it
  needs no further edit here.

  The distinguishing test for every candidate site is whether it reaches `Open` with a non-local
  `exploring` or `brainstorming` in its enabled set. `checkNodeRequirements` only errors; unlike the
  `applyCloseEnabledSet` migration path it does not self-heal. A site that upgrades instead of opening
  (for example `cmd/awf/run_test.go:861`, which goes through `runUpgrade`) needs no edit for that reason.

  Three sites that look affected are deliberately excluded, each for a verified reason. Do not edit them:
  - `internal/project/project_test.go:1458` and `internal/project/skillrefs_test.go:102` both declare a
    `skills/brainstorming.yaml` sidecar with `local: true`, and `checkKindAgainstCatalog` skips both the
    pool and closure checks for a local sidecar (`if sc.Local { continue }`,
    `internal/project/validate.go:105`). `Open` does not fail at either site, so an edit there would be
    invisible and misleading.
  - `internal/migrate/closeenabledset_test.go:82` never opens a project at all (`closeFixture` only
    writes `.awf/config.yaml` and sidecars), so it cannot fail project open. It is handled by Task 2.4a
    instead, which turns it into a real assertion rather than pre-seeding it.

  Deterministic post-check: `go test ./internal/project/ ./internal/migrate/` passes. A fixture missed
  by this batch fails with a closure-validation error naming the skill and the missing agent.

- [ ] **Task 2.4a: Make the closure fixture prove the new pairing edge.** In
  `internal/migrate/closeenabledset_test.go`, leave the fixture input at `agents: []`: pre-seeding
  `grounding-checker` there would make the closure a no-op and suppress the behaviour the test exists to
  prove. Instead, fill the currently-empty positive-want loop at `:99`
  (`for _, want := range []string{}`, which asserts nothing today) with `"- grounding-checker"`, so
  `TestCloseEnabledSetDropsDormantAndCloses` now proves that `applyCloseEnabledSet` closes the
  `brainstorming` to `grounding-checker` pairing it did not have to handle before. This converts a
  vacuous loop into backing for `config/migrations-and-locks:close-enabled-set-migration` over the new
  edge. Post-check: `go test ./internal/migrate/ -run TestCloseEnabledSetDropsDormantAndCloses` passes,
  and reverting Task 2.1's `brainstorming` pairing makes it fail.

- [ ] **Task 2.5: Register the generation-24 migration.** Create
  `internal/migrate/explorergroundingclosure.go` holding only a doc comment explaining that the
  generation closes the enabled set for adopters who already enable a dispatching skill (no new
  `Apply` function: it reuses `applyCloseEnabledSet`). Append to the `migrations` slice in
  `internal/migrate/migrate.go` after the `{To: 23, ...}` entry at `:57`:

  ```go
  {To: 24, Name: "explorer-grounding-closure", Apply: applyCloseEnabledSet},
  ```

- [ ] **Task 2.6: Map the generation and bump the version.** In `internal/project/project.go`, add
  `24: "0.28.0",` to `minVersionBySchema` (`:42`) and change `Version` (`:31`) from `"0.27.0"` to
  `"0.28.0"`. The map entry is forced: without it every gated command refuses before rendering,
  including the upgrade and gate meant to prove the change. The `0.28.0` target is ADR-0179 item 7's
  deliberate choice, so that one version is not the declared floor for two generations.

- [ ] **Task 2.7: Update the four literals pinning the old generation.** In
  `internal/project/version_test.go`: at `:15` change `minVersionBySchema[23]` to
  `minVersionBySchema[24]` and its failure message to name generation 24, keeping the
  highest-generation-equals-`Version` intent; at `:29` change the unmapped-generation case from
  `ValidateSchemaMinimumVersion(24, Version)` to `ValidateSchemaMinimumVersion(25, Version)` so it
  still exercises the "no minimum" arm. In `internal/migrate/dropworkflowtelemetry_test.go:11` change
  `Current() != 23` to `Current() != 24`. In `internal/migrate/workflowtelemetry_test.go:64` append
  `,explorer-grounding-closure` to the expected joined migration-name list.

- [ ] **Task 2.8: Upgrade both config trees to generation 24 and add the changelog entry.** A bare `awf
  upgrade` is not executable here: no `awf` is on PATH, generation 24 exists only in the from-source
  binary, and `examples/sundial`'s rendered wrapper sets no `awfInvokeCmd`, so it resolves a
  bootstrap-pinned released binary that cannot know generation 24. Use the block ADR-0177 Part A's plan
  established (`docs/plans/2026-07-29-rendered-implementer-role-contract-adr-0177-part-a.md:463-470`),
  which must run after Task 2.7 because the generation-pinning tests otherwise block it:

  ```bash
  bindir="$(mktemp -d)"
  go build -o "$bindir/awf" ./cmd/awf
  ./awf upgrade
  (cd examples/sundial && "$bindir/awf" upgrade)
  rm -rf "$bindir"
  ```

  Expect both commands to report the closure adding nothing, because Task 1.4 already enabled both
  agents, and to stamp `"schemaVersion": 24` and `"awfVersion": "0.28.0"` into `.awf/awf.lock` and
  `examples/sundial/.awf/awf.lock`, which currently record generation 23 at version `0.27.0`.

  Add a `changelog/CHANGELOG.md` `[Unreleased]` entry (the section at `:9`) covering generation 24 and
  the two Pi tools that now fail closed without their rendered agent, as `docs/releasing.md:66` requires
  for adopter-facing change.

- [ ] **Task 2.9: Render and verify.** Run `./x render` then `./x check`, which must print `awf check:
  clean`. Expect the render to rewrite `AGENTS.md`, `docs/config-reference.md`, and both `awf.lock` files;
  stage from `git status` rather than a fixed list. Confirm the generation gate accepts the tree:
  `./awf list` must succeed rather than refuse on schema generation.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction by path; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(config): pair the exploration dispatchers at generation 24
```

## Phase 3: Narrow the implementer claim and freeze both records

**Execution mode: inline.** This phase is one independently green coherent implementation transaction.
Checkbox tasks are ordered steps, not transaction boundaries.

- [ ] **Task 3.1: Narrow `implementer-role-contract`'s second sentence.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, replace the claim's second
  sentence, currently `The subagent-driven-development and executing-plans skills name that agent in
  every dispatch branch and address their own imperatives to an explicit subject.`, with:

  ```
  The subagent-driven-development and executing-plans skills name that agent in every dispatch branch, and their parent-facing imperatives for raising concerns, preserving the plan's settled design, running the context command, and inventorying batch returns each carry an explicit subject.
  ```

  Append `ADR-0179` to the claim's `Revised-by:` line, creating the line if absent. The four named
  categories cover all five literals pinned at `internal/project/spine_test.go:350`, `:357`,
  `:360-361`, and `:365` without asserting anything about imperatives no test checks. No test changes:
  the existing proof already establishes exactly this.

- [ ] **Task 3.2: Flip the ADR to `Implemented` and append batch two.** Two edits, both required in this
  same transaction:
  1. Change the frontmatter `status: Implementing` to `status: Implemented`. The same
     lastStatus-versus-frontmatter check at `internal/adr/format.go:360` fails without it, and
     `OperationProgress` additionally rejects an `Implemented` ADR with any remaining operation
     (`internal/adr/application.go:120`), which batch two is what satisfies.
  2. Append exactly two lines to `## Status history`, the sixth operation's `Applied` line THEN the
     `Implemented` line, in that order. `HistoryTransitionValid` accepts only
     `[Applied, Status]` for `Implementing -> Implemented`, and `validateV2History` additionally
     requires the final `Applied` event immediately before the terminal event on this explicit path.

  Reuse the digest from Phase 1 unchanged, since it excludes `Status history`. Obtain the next free
  sequence by the same documented method, because sibling ADRs may have consumed sequences since Phase 1.
  The `Implemented` event carries the digest and no state-sequence, since the explicit `Applied` events
  carry the sequences:

  ```
  - 2026-07-30: Applied; state-sequence: <next>; operations: update `rendering/workflow-skill-templates:implementer-role-contract`
  - 2026-07-30: Implemented; content-sha256: <same digest as the Implementing event>
  ```

- [ ] **Task 3.3: Flip the plan status and record findings.** Change this plan's frontmatter `status:
  Proposed` to `status: Implemented`, and record in Notes below anything that surfaced during
  execution: a wrong diff, an unsliceable phase, a guard neither the ADR nor this plan listed.

- [ ] **Task 3.4: Render and verify.** Run `./x render` then `./x check`, which must print `awf check:
  clean`. Expect it to regenerate `docs/decisions/INDEX.md` for the status change,
  `docs/topics/rendering/workflow-skill-templates.md` and `docs/domains/rendering.md` for Task 3.1's claim
  prose, and both locks; stage from `git status` rather than a fixed list.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction by path; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
docs(invariants): narrow the implementer contract claim and freeze 0179
```

## Verification

Whole-effort acceptance, beyond each phase's gate:

- `grep -rn "rolePrompt" templates/ .pi/ examples/sundial/.pi/` returns no output: no role prose
  remains inline in the extension or its rendered copies.
- `./awf topic rendering/workflow-skill-templates` and `./awf topic rendering/pi-workflows` both list
  their new claim with `[backing: test]`, and `./x check` reports clean, which is what proves every
  claim has a matching proof marker and every marker a known claim.
- `./awf topic rendering/workflow-skill-templates:bounded-exploration-reporting` no longer contains the
  phrase `Pi's fixed prompt`.
- The rendered explorer and grounding-checker agents exist for every enabled target that has an
  `AgentDir`, verifiable as a non-empty result from `ls .pi/agents/ .claude/agents/` containing both new
  names.
- `./x gate` passes on the final commit, which subsumes 100% coverage, the dead-code gate, the prose
  gate, and the drift check.
- ADR-0179's `## Status history` ends in `Implemented`, and every one of its six declared operations
  appears in exactly one `Applied` event.

## Notes

- **Finding to carry into the ADR resync:** ADR-0179 item 5's wording ("Both dispatching skills name
  their agent, symmetrically") is broader than what the item's own mechanics produce. The name is added
  only in each skill's non-Pi `{{ else }}` branch, because the Pi branch names the Pi tool instead. This
  plan's Task 1.11 therefore narrows the new claim's third sentence to the target-native branch rather
  than claiming every branch. The ADR's decision is unaffected; only a claim sentence derived from it
  needed scoping, and scoping it here is what prevents shipping the same over-broad-claim defect ADR-0179
  item 9 exists to correct.
- **Finding to carry into review:** ADR-0179 item 8 counts `target_test.go:298`'s pin among its
  test-edit obligations. Verified during planning that it needs no edit:
  `TestPiStructuredExplorationContractRender` searches the rendered extension for `queues the rest FIFO
  with abort-aware removal`, and Task 1.6 step 4 keeps that literal in the extension as a per-call
  suffix. The ADR's substantive point (the claim stays proven without a claim operation) holds; only its
  framing of the pin as an edit is loose. This plan therefore schedules no change there.
- Both new agent templates are deliberately data-free and conditional-free, which is a plan-level
  choice inside ADR-0179's design rather than something the ADR mandates. It avoids a `dataKeys` entry
  in `internal/configspec` and an `unsetFallbackCases` entry in `internal/project/spine_test.go`. If a
  later change adds a `.data` key or a `{{ if }}` to either template, both obligations reattach, and
  `TestConditionalTemplatesHaveFallbackCases` reads the case table rather than the test body, so a
  second render inside a golden test does not satisfy it.
- The `debugging` and `refactor-coupling-audit` pairing gap is out of scope per ADR-0179 item 6.
  Closing it needs either a multi-valued `RequiresAgent` or a live `RequiresSkills` closure.
- Two fixtures that look like they need the paired agent are local-sidecar exempt and are deliberately
  left alone (`internal/project/project_test.go:1458`, `internal/project/skillrefs_test.go:102`), and a
  third (`internal/migrate/closeenabledset_test.go:82`) never opens a project at all. Task 2.4a turns
  that third one into real backing for the new pairing edge instead of pre-seeding it, which would have
  suppressed the behaviour its test exists to prove. Plan review surfaced all three.
- Task 2.4's site set was twice wrong before it settled, in both directions: the first version included
  two exempt fixtures, and the corrected version still missed three genuinely affected ones
  (`internal/project/unused_test.go:63` and `:175`, `internal/project/spine_test.go:1704`), all found by
  the verify pass. The reliable discriminator is not "does the config enable the skill" but "does this
  site reach `Open` with a non-local `exploring` or `brainstorming`", since `checkNodeRequirements` only
  errors while the migration path self-heals. Executors should re-derive the set with that test rather
  than trusting the enumeration, and expect `go test ./internal/project/ ./internal/migrate/` to name
  anything still missing. ADR-0179 item 6 carried the same stale enumeration and was amended while
  `Proposed` to match this verified set, so the two records now agree rather than the plan silently
  superseding the decision.
- Out of scope and pre-existing: versions 0.23.0 through 0.27.0 have never been released
  (`changelog/CHANGELOG.md`'s newest release heading is 0.22.0), so `0.28.0` becomes the sixth
  unreleased floor. ADR-0179 item 7 records this as a release-cadence matter it does not address.
