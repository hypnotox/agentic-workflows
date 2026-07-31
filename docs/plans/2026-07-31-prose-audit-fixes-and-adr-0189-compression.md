---
date: 2026-07-31
adrs: [0189]
status: Proposed
---
# Plan: Prose audit fixes and ADR-0189 compression

## Goal

Land every sanctioned fix from the 2026-07-30 skill and agent prose audit
(docs/research/skill-agent-prose-audit-2026-07-30.md, sections A and B as corrected by
section D and by the 2026-07-31 post-0187 re-verification), and apply ADR-0189's
compress-with-reference decision for the governed model-selection prose as the deferred
final transaction. Non-goals: no section-id renames, no catalog `Sections` set changes,
and no behavioural change to any workflow contract; the plan changes what prose says and
where it lives, never what the chain requires.

All line numbers in this plan were verified against the effort branch at commit 142dba0c
(post-ADR-0187). They are indicative for locating a site; every task's acceptance is a
command reaching a terminal state, never a line number or a count. Where a task edits a
rendered-output-pinning test, the only sanctioned assertion changes are the two named in
Task 2.2 (a scope correction, claim quoted there) and the Phase 7 claim revision ADR-0189
authorises; any other invariant-marked assertion that blocks a trim means the trim is out
of scope - keep the pinned phrase and record the finding in Notes.

Every phase runs in the effort worktree
`.awf/worktrees/audit-skill-and-agent-prose-for-concision-and-steering-accuracy`, edits
templates and config sources only (never rendered files), and closes with
`./x render` + `./x check` clean, `awf check --staged` clean, and `./x gate` green.
Rendered outputs under `.claude/`, `.pi/`, `examples/sundial/`, `AGENTS.md`, and
`docs/` are regenerated and staged with each phase's source changes.

## Architecture summary

Seven phases: five audit-fix transactions ordered from pure prose corrections to
catalog-touching ones, then the shared-partial extractions ADR-0189 decision 2 bounds,
then the deferred ADR-0189 application transaction (template compression + claim update +
status flips) that lands under the terminal-review flow per the deferred-flip contract.

## File structure

- **Created:**
  - `templates/partials/model-selection.md` (Phase 7)
  - `templates/partials/context-orientation.md`, `templates/partials/staged-transaction.md`,
    `templates/partials/escalation-menu.md`, `templates/partials/coverage-oracle.md`,
    `templates/partials/exploration-ladder.md` (Phase 6)
- **Modified:**
  - `templates/skills/`: writing-plans, reviewing-plan, reviewing-plan-resync,
    subagent-driven-development, executing-plans, executing-direct, bugfix, debugging,
    tdd, adr-lifecycle, refactor-coupling-audit, roadmap-graduation, proposing-adr,
    reviewing-impl, reviewing-adr, exploring (SKILL.md.tmpl each)
  - `templates/agents/`: plan-reviewer, adr-reviewer, code-reviewer, explorer,
    grounding-checker, implementer (md.tmpl each, as named per task)
  - `templates/partials/`: review-spine-head.md, review-spine-tail.md,
    checkpoint-approval.md, checkpoint-routine.md
  - `templates/agents-doc/AGENTS.md.tmpl` (Phase 7),
    `templates/docs/workflow.md.tmpl` (Phase 3)
  - `internal/catalog/standard.go`, `internal/project/render.go`,
    `internal/project/spine_test.go`, `internal/project/phase_transaction_ownership_test.go`,
    `internal/project/subagent_model_selection_test.go`,
    `internal/project/guide_scopes_test.go`, `internal/evals/chain_test.go` (only if
    Task 4.1's relationship edits shift plain golden expectations; its
    `TestUnifiedEffortWorkflowCoverage` phrases are never edited)
  - `x` (comment lines 19-22 only, Phase 3)
  - `.awf/config.yaml`, `.awf/docs/glossary.yaml`, `.awf/docs/pitfalls.yaml`,
    `.awf/agents/plan-reviewer.yaml`,
    `.awf/agents/adr-reviewer.yaml`, `.awf/skills/parts/debugging/debugging-surfaces.md`,
    `.awf/parts/workflow/local-hooks.md`, `.awf/parts/workflow/composing-the-gate.md`,
    `.awf/docs/parts/testing/tiers.md`, `.awf/docs/parts/development/command-runner.md`,
    `.awf/parts/working-with-awf/config-and-overrides.md`
  - `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`,
    `docs/decisions/0189-compress-governed-dispatch-guidance-with-reference-and-shared-partials.md`,
    this plan (status flip, Phase 7)
  - All rendered outputs these sources produce (regenerated, never hand-edited)
- **Deleted:** none.

## Phase 1: Steering corrections, prose-only

**Execution mode: inline.** One transaction: template and catalog-description prose whose
current text steers incorrectly. No test asserts any of the defective literals below as an
invariant proof; if `./x gate` surfaces a plain golden-literal mismatch, update that
golden expectation to the corrected prose in the same transaction.

- [ ] **Task 1.1: Fix the broken plan-scaffold command (A1).** In
  `templates/skills/writing-plans/SKILL.md.tmpl:56`, replace the interpolation
  `` run `{{ .prefix }} new plan "<Title>"` `` with the binary-resolving conditional
  already used at `templates/skills/proposing-adr/SKILL.md.tmpl:64`, adapted to the plan
  command: `` run `{{ if .runnerEnabled }}./awf{{ else }}awf{{ end }} new plan "<Title>"` ``
  (preserve any surrounding `{{ with .vars.* }}` override the proposing-adr site carries
  only if an equivalent var exists for plans; otherwise the plain conditional). In the
  same sentence delete the alternative ``or copy `{{ .layout.plansTemplate }}` `` and its
  connective so the scaffold command is the only path, matching proposing-adr's ban on
  hand-copying generated files. Post-check: `grep -n '{{ .prefix }} new plan'
  templates/skills/writing-plans/SKILL.md.tmpl` returns no output, and after render
  `grep -rn 'sundial new plan' examples/sundial/` returns no output.
- [ ] **Task 1.2: Uncount every reviewer-lens enumeration (A2).** Replace stale lens
  counts/lists with count-free references so the text cannot drift again:
  - `templates/skills/reviewing-plan/SKILL.md.tmpl:16`: replace the five-name lens
    enumeration in the when-fires prose with "its universal lenses".
  - `templates/skills/reviewing-plan/SKILL.md.tmpl:39`: replace "all five lenses:
    scope-completeness, executability, doc-currency, convention-alignment,
    testing-discipline" with "all universal lenses".
  - `templates/skills/reviewing-plan-resync/SKILL.md.tmpl:29` and `:65`: replace "The
    other three lenses (executability, convention-alignment, testing-discipline)" and
    "The other three ran during..." with "The remaining lenses" / "The remaining lenses
    ran during...".
  - `templates/agents/plan-reviewer.md.tmpl:48`: replace "The other three lenses
    (`executability`, `convention-alignment`, `testing-discipline`)" with "The remaining
    lenses".
  - `internal/catalog/standard.go:164` (code-reviewer `Description`): replace the
    five-lens enumeration "correctness, plan-adherence, testing discipline, doc currency,
    and convention alignment" with a count-free description, e.g. "covering its universal
    review lenses from correctness through convention alignment".
  Post-check: `grep -rn 'five lenses\|other three' templates/ internal/` returns no
  output (catches the "all five lenses" dispatch strings, the when-fires five-name
  parenthesis lead-in, and both "other three lenses"/"other three ran" forms), and the
  code-reviewer `Description` in `internal/catalog/standard.go` no longer enumerates
  lens names (visual check of the edited string).
- [ ] **Task 1.3: Fix the resync skill's contradictions (A5).** In
  `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`:
  - Move the plan-path identification instruction ("Identify the plan path (named in the
    just-settled ADR(s) or the most recently-modified file under
    `{{ .layout.plansDir }}/` matching `YYYY-MM-DD-*.md`)."), currently only in the
    non-Pi else branch at line 26, to shared prose immediately before the branch pair at
    line 25 so both branches render it (reword each branch to reference "the identified
    plan path").
  - Frontmatter description (line 4) and when-fires body (line 15): replace the
    "Invoked by `{{ .prefix }}-reviewing-adr`..." sole-invoker phrasing with wording that
    names both invokers, e.g. "Invoked after review settles - by
    `{{ .prefix }}-reviewing-adr` as its terminal follow-on, or by
    `{{ .prefix }}-reviewing-plan` when at least one linked ADR exists." The catalog
    entry (`internal/catalog/standard.go:276`) already lists both `UsuallyFollows`
    values; do not edit it.
  - `templates/agents/plan-reviewer.md.tmpl:50`: reword "after the linked ADR review
    converges" so resync mode is described as following whichever review settled, not
    only an ADR review.
  Post-check: after render, `grep -n 'Identify the plan path' .pi/skills/awf-reviewing-plan-resync/SKILL.md`
  returns a match (Pi branch now carries it).
- [ ] **Task 1.4: Fix the sdd memory-writer exception and hook-subject drift (A6).** In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl:33`:
  - Delete the sentence "The child does not edit parent memory unless the user explicitly
    transfers sole effort-writer ownership to it." and replace with the unconditional
    form matching the implementer contract (`templates/agents/implementer.md.tmpl:42-45`)
    and the agent guide: "The child never edits parent memory."
  - Split the sentence "Commit only after both commands pass; the hook repeats the staged
    check as defense in depth, then creates the declared phase-closing commit." so the
    owner is the commit's subject: "Commit only after both commands pass; the hook
    repeats the staged check as defense in depth. The owner then creates the declared
    phase-closing commit."
  Post-check: `grep -n 'transfers sole effort-writer' templates/` returns no output.
- [ ] **Task 1.5: Fix the degraded review-digest line (A11).** In
  `templates/partials/review-spine-tail.md:19`, change
  `{{ with .data.digestLabel }}{{ . }}{{ else }}Review{{ end }} review complete (N lenses, M findings).`
  to
  `{{ with .data.digestLabel }}{{ . }} review complete{{ else }}Review complete{{ end }} (N lenses, M findings).`
  Post-check: all three reviewer agents set `digestLabel` in the catalog
  (`internal/catalog/standard.go:139,158,180`), so no rendered tree exercises the
  degraded branch - verify it through the empty-data/publication-safety template sweep
  in `internal/project` (the test that renders templates with empty variables) instead
  of a rendered-tree grep: the swept output contains "Review complete" and never
  "Review review".
- [ ] **Task 1.6: Small accuracy corrections (A12).**
  - `templates/skills/bugfix/SKILL.md.tmpl:36` (step 5): replace the generic "Run the
    project's review step as the terminal step." with the same skill-conditional the
    frontmatter (line 3) already uses: "Run
    `{{ if index .skills "reviewing-impl" }}{{ .prefix }}-reviewing-impl{{ else }}the project's review step{{ end }}`
    as the terminal step."
  - `.awf/skills/parts/debugging/debugging-surfaces.md:5`: replace "after any sync" with
    "after any render". Line 12: replace "use the target-native governed exploration
    loader" with "invoke `awf-exploring`" (this is a repo-local part; the literal skill
    name is correct here).
  - `templates/agents/grounding-checker.md.tmpl:21`: replace "Assess whether the effort
    needs a decision record..." with "Assess whether the work needs a decision
    record..." (effort is a reserved term).
  - `templates/skills/executing-plans/SKILL.md.tmpl:35` and
    `templates/skills/subagent-driven-development/SKILL.md.tmpl:40`: remove "prior
    concerns" from BOTH inventories in each file - the dirty-stop inventory and the
    ownership-transfer inventory (the phrase occurs twice per file). Justification: the
    implementer's stopped-report schema
    (`templates/agents/implementer.md.tmpl:101-108`) defines exactly five fields and no
    such field, and no schema anywhere defines the field for the parent-authored
    transfer inventory either - both inventories reference a field nothing produces.
    Do not add a field to the implementer instead (the schema is the contract; the
    skills drifted).
  (The tdd "validate" correction moved into Task 5.1(b), which rewrites that line.)
  Post-check: `grep -rn 'prior concerns' templates/skills/` returns no output;
  `grep -n 'after any sync' .awf/skills/parts/` returns no output.
- [ ] **Phase-close: stage, check, gate, and commit.** `./x render` then `./x check`
  (clean); stage all source and rendered changes; `awf check --staged` (clean);
  `./x gate` (green); commit:

```commit
fix(rendering): correct stale steering in skill and agent prose
```

## Phase 2: Confine the allowCommits literal to Pi

**Execution mode: inline.** One transaction covering the template fix and its two test
edits (A3, as re-verified 2026-07-31: two edits, one invariant claim, not D6's three).

- [ ] **Task 2.1: Move the literal into the Pi branch.** In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl:33`, the dispatch step
  currently instructs "Call one implementation child alone in its parent tool batch with
  `allowCommits: true`..." unconditionally, with only the following model-selection
  clause branched on `{{ if .targetSubagentTools }}`. Restructure so the
  `` with `allowCommits: true` `` phrase renders only inside the
  `{{ if .targetSubagentTools }}` (Pi) branch; the generic branch instead says "and state
  commit-capable phase-owner mode in the brief" (the implementer contract defaults an
  unspecified mode to commit-disabled helper). Post-check: after render,
  `grep -n 'allowCommits' .claude/skills/awf-subagent-driven-development/SKILL.md`
  returns no output while
  `grep -n 'allowCommits' .pi/skills/awf-subagent-driven-development/SKILL.md` returns a
  match.
- [ ] **Task 2.2: Update the two generic-render test expectations.** Exactly two test
  sites assert the literal in generic (non-Pi) renders and need editing:
  - `internal/project/spine_test.go:875` (`TestSubagentDrivenDevelopmentTemplate`, plain
    golden test, no invariant marker): remove `allowCommits: true` from the generic
    `loadBearing` expectations and add the new generic-branch phrase ("state
    commit-capable phase-owner mode in the brief") in its place.
  - `internal/project/phase_transaction_ownership_test.go:80`
    (`TestPhaseTransactionOwnershipAcrossWorkflowSurfaces`, proof for
    `invariant: rendering/workflow-skill-templates:phase-transaction-ownership`, Origin
    ADR-0166): the test has no Pi variant (neither `assertContract` call sets
    `targetSubagentTools`, and `assertAll("subagent", ...)` shares one clause list
    across variants). Remove `allowCommits: true` from that shared clause list and
    assert the generic-branch phrase in its place; do NOT add a Pi variant here. The
    Pi-side literal remains pinned by the existing Pi case in
    `internal/project/spine_test.go:628` (`TestMaintainableCodeSubagentContract`,
    ADR-0168), which is the surviving pin.
  This is a scope correction, not a weakened assertion. The claim text of
  `phase-transaction-ownership` reads, in full: "A rendered plan phase is one
  independently green coherent implementation transaction with an explicit per-phase
  inline or subagent-driven owner; checkbox tasks are ordered steps rather than default
  dispatch, review, checkpoint, or commit boundaries. One commit-capable implementer owns
  a complete subagent-driven phase from a known green baseline through staged check,
  gate, and closing commit, while the parent owns inline integration, sequential
  commit-disabled batch helpers, report-only review settlement, phase checkpointing, and
  explicit dirty-state recovery without blind task-level succession." It never names
  `allowCommits`; the commit-capable-owner commitment is still asserted via the new
  generic phrase and the Pi-branch literal. The other two `allowCommits` test sites
  (`internal/project/spine_test.go:559` and `:628`, ADR-0168 proofs) already render or
  branch Pi-only and need no edit; touching them is forbidden in this phase.
- [ ] **Phase-close: stage, check, gate, and commit.** `./x render`, `./x check`,
  stage, `awf check --staged`, `./x gate` all clean/green; commit:

```commit
fix(rendering): confine the allowCommits dispatch literal to Pi
```

## Phase 3: Remove the fictitious gate fast/full split

**Execution mode: inline.** One transaction (A4 + D5): the config var, the four
hand-written parts that restate the split, and every rendered output, same commit.

- [ ] **Task 3.1: Unset the var and retire the stale hook-compat claims.** In
  `.awf/config.yaml:201`, delete the line `gateCmdFull: ./x gate full`. Once unset, the
  regenerated `.awf/hooks/pre-push.sh` composes plain `./x gate`, so NOTHING generated
  passes `full` any more; every "exists for pre-push hook compatibility" sentence
  becomes false in the same commit. Update the `x` script comment (lines 19-22) to the
  new truth: "the optional `full` arg is accepted as a no-op legacy argument; no
  rendered artifact passes it; awf has no slower tier." The `full` acceptance in `x`
  itself stays (external callers may still pass it).
- [ ] **Task 3.2: Correct the guarded and unguarded template prose and the four
  convention parts.**
  - `templates/skills/bugfix/SKILL.md.tmpl:32` and
    `templates/skills/debugging/SKILL.md.tmpl:42`: the "(fast tier)" parenthetical sits
    OUTSIDE the `{{ if .vars.gateCmdFull }}` guard and renders unconditionally; delete
    the parenthetical from both templates (the guarded `Run ... full ...` sentence
    falls away on its own once the var is unset in this repo, and stays coherent for
    adopters who set the var).
  - `templates/docs/workflow.md.tmpl:67` and `:89`: the "a fuller tier" / "the fuller
    tier ... before merging" prose is unconditional; wrap each sentence in a
    `{{ if .vars.gateCmdFull }}...{{ end }}` guard so it renders only where a fuller
    tier exists (publication-safe: with the var unset the surrounding prose reads
    complete).
  - `.awf/parts/workflow/local-hooks.md:3`: remove `gateCmdFull` from the config list;
    where it names the pre-push command, say it runs the single gate.
  - `.awf/parts/workflow/composing-the-gate.md:12`: keep the "there is no slower tier"
    statement; restate the `full` arg as a no-op legacy argument nothing rendered
    passes; remove any instruction to choose between tiers.
  - `.awf/docs/parts/testing/tiers.md:3-7`: rewrite to the single-tier reality: one
    gate, `./x gate`, with `full` accepted as a no-op legacy argument.
  - `.awf/docs/parts/development/command-runner.md:10`: same restatement; drop any
    phrasing implying a distinct fuller run or that a rendered hook passes `full`.
  The part edits are qualifying-instruction edits (non-contractual prose); preserve
  each part's other content.
- [ ] **Task 3.3: Verify the rendered fallout.** `./x render` regenerates
  `.awf/hooks/pre-push.sh`, the debugging and bugfix skills, and `docs/workflow.md`,
  `docs/testing.md`, `docs/development.md`. Post-check: `./x check` clean;
  `grep -rn 'gate full' .claude/skills/ .pi/skills/` returns no output;
  `grep -rn 'fast tier' templates/skills/ .claude/skills/` returns no output;
  `grep -n 'fuller tier' docs/workflow.md` returns no output;
  `bash -n .awf/hooks/pre-push.sh` exits 0.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage config, parts, and all
  rendered outputs; `awf check --staged`, `./x gate`; commit:

```commit
fix(config): drop the fictitious gate fast tier
```

## Phase 4: Catalog relationships and skill-kind vocabulary

**Execution mode: inline.** One transaction (A9, A10, E3, E4): catalog relationship
fixes, the four "task skill" body rewordings, the render-key vocabulary rename, and the
roadmap-graduation corrections. Catalog edits regenerate every target's guide and the
sundial example.

- [ ] **Task 4.1: Align the catalog relationship map (A9).** In
  `internal/catalog/standard.go` (relationship map, lines 263-281):
  - `debugging` (line 269): remove `"executing-direct"` from `CommonFollowUps` (no body
    routes it; executing-direct's entry condition contradicts it), leaving
    `[]string{"bugfix"}`.
  - `tdd` (line 268): add `UsuallyFollows: []string{"bugfix", "debugging"}` (its actual
    invokers; both bodies invoke `{{ .prefix }}-tdd` conditionally). Keep its
    `CommonFollowUps` unchanged.
  - `refactor-coupling-audit` (line 280): remove `UsuallyFollows: []string{"exploring"}`
    (inverted: the audit body invokes exploring as a sub-step; the structural
    `RequiresSkills: ["exploring"]` at line 110 stays), and add `"proposing-adr"` to
    `CommonFollowUps` (the one skill its output exists to feed, per its own body line
    25).
  - `executing-plans` (line 266) and `subagent-driven-development` (line 267): add each
    to the other's `CommonFollowUps` (their bodies hand mixed-plan phases to each
    other).
  - `roadmap-graduation` (line 281): reword `Trigger` to cover all three body cases:
    "Use when a roadmap entry graduates to an ADR or a PR, or is explicitly dropped."
  Post-check: `go test ./internal/catalog/ ./internal/evals/` passes (update any plain
  relationship-golden expectations to the corrected map; the evals chain tests assert
  chain nodes, and none of the edited entries adds or removes a chain node).
- [ ] **Task 4.2: Reword the four support-skill bodies (A10, catalog wins).** Replace
  the "task skill" self-description with "support skill" (and adjust the surrounding
  sentence so it still reads naturally, keeping the "sits off the workflow chain"
  meaning) at exactly these sites: `templates/skills/tdd/SKILL.md.tmpl:8`,
  `templates/skills/adr-lifecycle/SKILL.md.tmpl:11`,
  `templates/skills/refactor-coupling-audit/SKILL.md.tmpl:12` and `:25`,
  `templates/skills/roadmap-graduation/SKILL.md.tmpl:8` and `:15`.
  `templates/skills/bugfix/SKILL.md.tmpl:8` ("The fix-and-verify task skill.") is
  correctly `WorkflowTask` and MUST NOT change; debugging has no self-description and
  gets none. Post-check: `grep -ln 'task skill' templates/skills/*/SKILL.md.tmpl`
  returns only the bugfix template.
- [ ] **Task 4.3: Rename the taskSkillRows vocabulary layer (E3).** The render key
  `taskSkillRows` produces one advisory row for every enabled skill (function comment,
  `internal/project/render.go:112-113`), so "task" is wrong inside and out. Rename the
  Go method and render key `taskSkillRows` to `skillRows` at
  `internal/project/render.go:98` and `:112-113`, rename every template reference to the
  key (locate with `grep -rn 'taskSkillRows' templates/`), update the test references
  `p.taskSkillRows()` at `internal/project/guide_scopes_test.go:46` and `:76` and the
  `"taskSkillRows"` data key at `:70`, update the glossary entry
  `.awf/docs/glossary.yaml:6` (key and description: one advisory row per enabled skill,
  all kinds), rename the worked example at `.awf/docs/pitfalls.yaml:446`, and reword
  the test comments that use "task skill" to mean any non-chain skill at
  `internal/project/spine_test.go:1740`, `:1913`, `:1950`. Leave
  `internal/evals/chain_test.go:123` and `:237` unchanged (their usage names the actual
  `WorkflowTask` skills and is correct). Post-check:
  `grep -rn 'taskSkillRows' templates/ internal/ .awf/` returns no output (the token
  legitimately survives in this plan file and in git history); `./x gate` green (the
  gate compiles tests; `go build ./...` alone would miss the test-file references).
- [ ] **Task 4.4: Fix roadmap-graduation's frontmatter and commit instruction (A9, E4,
  B3).** In `templates/skills/roadmap-graduation/SKILL.md.tmpl`:
  - Frontmatter description (line 3): align with the three-case trigger from Task 4.1
    ("...a roadmap entry graduates to an ADR or a PR, or is explicitly dropped...").
  - Section 4 "Explicit drop" (line 48): change the commit template to subject
    `docs(roadmap): drop <item>` with the one-line reason in the commit body, removing
    `- <one-line reason>` from the subject; the following sentence ("The reason goes in
    the commit body, not the file.") then agrees with the template.
  - `failure-modes` and `same-commit` are catalog-declared sections for this skill
    (`internal/catalog/standard.go:117-118`), so their `awf:section` markers MUST stay
    (deleting one breaks `rendering/catalog-and-targets:skill-section-parity`,
    ADR-0054, and changing the catalog `Sections` set is a plan non-goal). Empty the
    `failure-modes` default body (adopters keep the override point) and reduce the
    `same-commit` default body to the single fold-in sentence for step 3, so the
    same-commit rule renders once in the intro (line 8) and once in the procedure.
  Note: roadmap-graduation is not enabled in this repo, so these edits produce no
  rendered diff here; the sundial example and golden template tests are the rendered
  surfaces. Post-check: the template contains exactly one `docs(roadmap): drop`
  template string, both `awf:section` markers survive
  (`grep -c 'awf:section' templates/skills/roadmap-graduation/SKILL.md.tmpl` is
  unchanged from before the edit), and `go test ./internal/project/` section-parity
  tests pass.
- [ ] **Phase-close: stage, check, gate, and commit.** `./x render`, `./x check`, stage
  (including every regenerated guide and the sundial example), `awf check --staged`,
  `./x gate`; commit:

```commit
fix(rendering): align catalog relationships and skill-kind vocabulary
```

## Phase 5: Concision cuts in place

**Execution mode: inline.** One transaction for the restatement cuts that need no new
partial (B2 preamble trims, B3 notes cuts, B6 filler, B5 sidecars). Binding constraint:
`TestUnifiedEffortWorkflowCoverage` (`internal/evals/chain_test.go:276`, invariant proof
for `rendering/workflow-skill-templates:unified-effort-workflow-coverage`) pins phrases
that must survive in EVERY rendered skill body - "standalone memory is forbidden", the
repository-authority phrase, and "writer" everywhere, plus "minimal simple" in tdd and
roadmap-graduation and "never edit" in refactor-coupling-audit. Read
`internal/evals/chain_test.go:300-340` and enumerate the asserted phrases before
authoring any trim; the test itself is not edited in this phase, and a cut a pinned
phrase blocks is out of scope.

- [ ] **Task 5.1: Trim the effort/working-memory preambles (B2).** Batch task in two
  site groups (all `templates/skills/<name>/SKILL.md.tmpl`):
  (a) Checkpoint-bearing sites, whose included checkpoint partial restates part of the
  contract: brainstorming:21, proposing-adr:39, writing-plans:49, reviewing-adr:20,
  reviewing-plan:23, reviewing-plan-resync:22, reviewing-impl:8, executing-direct:16,
  executing-plans:21, subagent-driven-development:22, bugfix:21, debugging:35.
  Representative (brainstorming:21): trim the preamble to the operative core plus the
  pinned phrases: "A minimal simple fix uses no effort. Carry the effort slug and exact
  `.awf/efforts/<slug>/memory.md` path through every step; children receive them
  read-only and never edit shared memory. Repository sources and current-state
  documentation outrank checkpoint prose; standalone memory is forbidden and one
  user-managed writer remains responsible. The full protocol lives in the checkpoint
  below." (The opening sentence is load-bearing for brainstorming: its pinned "minimal
  simple" phrase has no other occurrence in that body, and brainstorming includes
  `checkpoint-approval.md`, which does not carry it; sites including
  `checkpoint-routine.md` regain the phrase from the partial and may drop the opening
  sentence where it would double.) Edge (reviewing-impl:8): additionally
  keep the reviewer clause "the report-only reviewer receives slug/path only as context
  and never edits shared memory" verbatim. Also remove executing-direct's separate
  restatement at line 20 ("a minimal simple fix remains effort-free") - its preamble
  core keeps the contract, and "minimal simple" is not pinned for that skill.
  (b) Partial-less sites, whose preamble is the body's only contract carrier: tdd:19,
  adr-lifecycle:50, refactor-coupling-audit:33, roadmap-graduation:28. Keep the full
  contract sentence set including each site's pinned extra phrase ("minimal simple" in
  tdd and roadmap-graduation, "never edit" in refactor-coupling-audit); cut only
  phrases doubled within the same site, and do NOT add a "checkpoint below" reference
  (nothing renders one there). At the tdd site fold in the A12 wording correction:
  "create or resume exactly one immutable slugged effort" replaces "validate exactly
  one immutable slugged effort".
  Post-check: `./x gate` green with `internal/evals/chain_test.go` unedited; every
  phrase `TestUnifiedEffortWorkflowCoverage` asserts still renders in its skill body.
- [ ] **Task 5.2: Merge the checkpoint partials' overlapping items (B2).** In
  `templates/partials/checkpoint-approval.md` and
  `templates/partials/checkpoint-routine.md`, items 1 and 2 restate each other
  (create-or-resume/owns-memory/standalone-forbidden in item 1;
  validate-slug-and-path/one-writer/reviewer-never-edits in item 2). In each partial,
  merge into: item 1 = classify the outcome and create-or-resume the single effort with
  its owned path (keep the minimal-fix exception sentence verbatim); item 2 = validate
  slug/path/`Effort:` header and preserve one user-managed writer with report-only
  children (keep the reviewer-never-edits sentence verbatim). Cut only the
  double-stated phrases; every distinct contract element survives exactly once per
  partial. Post-check: `./x gate` green; within the rendered
  `.claude/skills/awf-brainstorming/SKILL.md`, each contract element appears once in
  the trimmed preamble and once per included checkpoint partial occurrence, and no
  longer twice within a single checkpoint block (read the rendered checkpoint block to
  confirm; the binding check is the gate).
- [ ] **Task 5.3: Cut notes-restating-the-file (B3).**
  - `templates/skills/proposing-adr/SKILL.md.tmpl:83-87`: delete the three Notes
    bullets restating the lifecycle pointer (line 12), the frontmatter/sections block
    (lines 33-35), and the INDEX regen (lines 63-64); keep any Notes bullet carrying
    novel content.
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl:107,109`: delete the two Notes items
    restating the intro (line 9) and step 4's INDEX rule (lines 64-65).
  - `templates/skills/reviewing-impl/SKILL.md.tmpl`: keep the independence statement in
    the preamble (line 8) and step 3 (line 34); delete the third restatement in Notes
    (line 86). Keep the docs-only rule in its `docs-only-check` section (line 29);
    delete the preamble duplicate (line 15).
  - `templates/skills/executing-plans/SKILL.md.tmpl:29`: delete the literal second
    sentence "No drift from the plan." keeping "...and do not drift from the plan."
  - `templates/skills/debugging/SKILL.md.tmpl:14-16` vs `:19-21`: the symptom-list
    default section and When-to-invoke render back-to-back near-verbatim; empty the
    symptom-list default (the section stays for adopter override; its default body
    becomes empty) so When-to-invoke is the single statement. Verify at execution that
    this pre-0187 finding still holds; if the corpus changed here, skip and note.
  - `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`: delete the "Test-coupling
    planning rule" section body that restates category 2 (verify at execution; rendered
    line 94 pre-0187). The section marker stays (empty default) if it is
    catalog-declared; otherwise remove the heading with the body.
  - writing-plans Notes: the audit's "restates positioning" sub-claim was found WEAK at
    re-verification; re-read `templates/skills/writing-plans/SKILL.md.tmpl:84,88` at
    execution and cut only a genuinely verbatim duplicate, else skip and note.
  Post-check: `./x render && ./x check` clean; no catalog-declared section is removed
  (`go test ./internal/project/` section-parity tests pass unchanged).
- [ ] **Task 5.4: Cut filler and name the jargon (B6).**
  - `templates/skills/exploring/SKILL.md.tmpl:51`: delete the "Pi is deeply
    integrated..." Notes sentence.
  - `templates/skills/executing-direct/SKILL.md.tmpl:19`: replace "larger authority
    batch" with plain language naming what it is ("a larger approved change set").
  - `templates/skills/refactor-coupling-audit/SKILL.md.tmpl:35`: split the run-on
    "audit shape" paragraph and open it by defining the choice ("Pick the audit shape:
    run the categories inline, or dispatch exploration children per category.").
  - `templates/skills/executing-plans/SKILL.md.tmpl:44` and
    `templates/skills/subagent-driven-development/SKILL.md.tmpl:28`: expand "V2
    operation batches" to "the ADR's State changes operation batches" at first use.
- [ ] **Task 5.5: Uncount refactor-coupling-audit and track its surviving categories
  (A7, corrected D3).** In `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`:
  frontmatter (line 5) and body (line 12) drop "6-category" in favour of "the coupling
  audit"; the audit-shape section (line 35) drops "Preserve all six categories" in
  favour of "Preserve the structured output contract"; visible heading numbers ("### 1.
  ...") are removed while every `category-N-*` section id and marker stays exactly as
  is (renaming an id breaks `skill-section-parity`, ADR-0054, and orphans adopter
  override parts); each category section carries its own output-line instruction (move
  the per-category lines - including "Codegen sites:" and "Constructor paths:" - out of
  the shared Output block into their category sections, so a sidecar-dropped section
  drops its output line with it); the Notes pointer at the dangling "`customise:` hints
  above" is rewritten to name the real sparse-render mechanism, the sidecar
  `sections: <id>: {drop: true}` (per the corrected D3 finding: this repo's
  `.awf/skills/refactor-coupling-audit.yaml` drops categories 4 and 5 by sidecar; no
  convention part exists for this skill). Post-check: rendered
  `.claude/skills/awf-refactor-coupling-audit/SKILL.md` shows unnumbered headings with
  no gap-implying jump, its Output block demands only lines its surviving sections
  taught, and `grep -n 'all six\|6-category' templates/skills/refactor-coupling-audit/`
  returns no output.
- [ ] **Task 5.6: Reduce the sidecar duplication (B5, E2).**
  - `.awf/agents/plan-reviewer.yaml` `step-exactness`: reduce the ~200-word item to
    its EXISTING final sentence, kept verbatim as the sole content: "Reject task-level
    boundaries, cross-phase definitions, dead-code exceptions, plan-wide mode
    inference, and placeholders." (User ruling 2026-07-31: this sentence carries the
    four rejects the universal executability lens does not enumerate; everything else
    in the item restates that lens and is deleted. Do not author a new sentence.) Keep
    the sibling focus items untouched (they carry incident narrative).
  - `.awf/agents/adr-reviewer.yaml`: delete the `decision-clarity` (lines 14-15) and
    `consequences-honesty` (lines 16-17) focus items (verbatim restates of universal
    lenses 1 and 4 with no "kept deliberately" note) and the INDEX-regen docCurrency
    item (line 7; the template tail at `templates/agents/adr-reviewer.md.tmpl:43`
    already carries the unconditional INDEX-regen check). The docCurrency checks at
    lines 8 (AGENTS.md currency) and 11 (update/rename provenance) are load-bearing
    and MUST NOT be touched. If a deleted item carries a clause not in the universal
    lens, keep that clause as a one-line item instead.
- [ ] **Phase-close: stage, check, gate, and commit.** `./x render`, `./x check`, stage,
  `awf check --staged`, `./x gate`; commit:

```commit
feat(rendering): trim restated prose across skills and agents
```

## Phase 6: Shared-spine partials

**Execution mode: inline.** One transaction for the B4 extractions ADR-0189 decision 2
authorises. Bounds (binding for every task here): preserve every section id, every
catalog `Sections` set, and `awf:edit` overridability; a partial carries no nested
include and no section marker; content and line structure may change, contract meaning
may not. The extracted text becomes its own line(s) at each including site (directives
are line-anchored); restructure surrounding sentences accordingly.

- [ ] **Task 6.1: Context-orientation partial.** Create
  `templates/partials/context-orientation.md` holding the shared parenthetical
  currently at `templates/skills/refactor-coupling-audit/SKILL.md.tmpl:35`,
  `templates/skills/tdd/SKILL.md.tmpl:21`, `templates/skills/debugging/SKILL.md.tmpl:37`,
  `templates/skills/bugfix/SKILL.md.tmpl:23`,
  `templates/skills/writing-plans/SKILL.md.tmpl:60`. The sites vary a mid-sentence
  clause ("where the suspect surface / the change / a task touches a claimed surface"),
  which one line-anchored include cannot express per-site; per the user ruling
  (2026-07-31), NORMALIZE the clause to one wording across all five sites - "where the
  work touches a claimed surface" - and extract the whole normalized sentence ("start
  with bare context to orient on the owning domains and applicable current-state
  claims, then drill down with `awf topic` where the work touches a claimed surface")
  into the partial. Replace each site's copy with the include directive on its own
  line, restructuring the surrounding sentence so the frame stands alone. Post-check:
  `grep -rln 'drill down with' templates/skills/` returns no output (the phrase lives
  only in the partial); rendered skills still each carry the full sentence.
- [ ] **Task 6.2: Staged-transaction partial.** Create
  `templates/partials/staged-transaction.md` with the shared text at
  `templates/skills/adr-lifecycle/SKILL.md.tmpl:69`,
  `templates/skills/executing-plans/SKILL.md.tmpl:31`,
  `templates/skills/subagent-driven-development/SKILL.md.tmpl:33` ("Stage the complete
  transaction, run `awf check --staged`, then ... gate ... Commit only after both
  commands pass."; extract the exact common core, leaving each site's surrounding
  specifics - hook sentence, owner sentence - in place). The implementer agent's
  numbered child form stays untouched. Same include mechanics and post-check shape as
  6.1 (grep the core phrase in templates/skills/ - only includes remain).
- [ ] **Task 6.3: Escalation-menu and coverage-oracle partials.** Create
  `templates/partials/escalation-menu.md` carrying the FULL frame "perform it first,
  include it in the current effort, defer it in a durable project-owned record, or
  decline it with the trade-off stated" (the trailing "with the trade-off stated" is
  asserted for tdd and bugfix at `internal/project/spine_test.go:550` and `:562` under
  the `maintainable-code-stage-coverage` invariant proof at `:518` - a truncated
  partial breaks that proof); skill sites: bugfix, refactor-coupling-audit, tdd. The
  fourth copy at `templates/docs/maintainable-code-design.md.tmpl:38` stays as that
  doc's canonical prose home and is NOT converted to an include. Create
  `templates/partials/coverage-oracle.md` for the "Coverage may never regress. A fix
  that breaks an existing passing test is itself a bug." frame; sites:
  `templates/skills/bugfix/SKILL.md.tmpl:58`,
  `templates/skills/debugging/SKILL.md.tmpl:63`,
  `templates/skills/tdd/SKILL.md.tmpl:30`. The tdd variant is the outlier (colon
  separator, ends "regression"); unify on the bugfix/debugging wording and adjust
  tdd:30 - the other two sites already match. debugging:63 carries a trailing
  `{{ if .layout.docs.debugging }}` clause that must survive on the line after the
  include. Include at each site; same post-check shape as 6.1, plus
  `go test ./internal/project/ -run MaintainableCodeStage` passes unedited.
- [ ] **Task 6.4: Exploration-ladder partial.** Create
  `templates/partials/exploration-ladder.md` holding the breadth and detail ladder
  definitions (`targeted < bounded < broad` as an adaptive maximum;
  `paths < summary < analysis` independent of breadth) currently duplicated between
  `templates/skills/exploring/SKILL.md.tmpl:14-19,22-25` and
  `templates/agents/explorer.md.tmpl` (Breadth / Report-detail sections). Include it
  from both files. Constraint: the claim
  `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts` pins the
  rendered explorer body's ladder content - the partial preserves that rendered content
  verbatim (ordering words included); run
  `go test ./internal/project/ -run 'Explorer|Grounding'` and the full gate to confirm
  no invariant assertion moved. Also trim the exploring skill's dispatch sentence to
  "provide the task, breadth, and detail" where it currently restates contracts the
  explorer agent already carries, and cut the identity self-repeat pairs
  (`templates/agents/explorer.md.tmpl:3` vs `:9`,
  `templates/agents/grounding-checker.md.tmpl:3` vs `:8`): keep the Identity-section
  sentence, reduce the line-3 opener to its non-duplicated remainder.
- [ ] **Task 6.5: Reviewer intros into the spine head.** The three reviewer agent
  templates open with a structurally identical intro
  (`templates/agents/adr-reviewer.md.tmpl:3`, `templates/agents/plan-reviewer.md.tmpl:3`,
  `templates/agents/code-reviewer.md.tmpl:3`: "Independent, [...] reviewer for X ...
  structured findings ... mechanical/reasoned/user-decision ... Report-only: it does not
  edit, commit, or re-review."). Move the shared frame into
  `templates/partials/review-spine-head.md` (which all three already include), keeping
  each agent's subject phrase ("for ADRs under...", "for plans under...", "for awf
  implementation diffs") as the per-file remainder on its own line before or after the
  include. Also delete the review-spine-tail internal restatement
  (`templates/partials/review-spine-tail.md:24` restating line 11). Post-check: render;
  the three rendered agents each still open with a complete intro; `./x gate` green.
- [ ] **Task 6.6: Execution-spine dedup between executing-plans and sdd.** Extract the
  identical dirty-stop recovery frame (inventory list, staged-transaction reference, V2
  ownership sentence) shared by `templates/skills/executing-plans/SKILL.md.tmpl:35` and
  `templates/skills/subagent-driven-development/SKILL.md.tmpl:40` into the existing
  partials where they fit (staged-transaction from 6.2) or inline-shared wording; the
  genuinely differing verb ("restart the complete revised phase" inline vs "redispatch
  the complete revised phase" dispatched) is semantic and stays per-file - do NOT unify
  it. INDEX-regen doubling inside proposing-adr and adr-lifecycle fell in Task 5.3.
  Post-check: `./x render && ./x check` clean.
- [ ] **Phase-close: stage, check, gate, and commit.** `./x render`, `./x check`, stage,
  `awf check --staged`, `./x gate`; commit:

```commit
feat(rendering): extract shared prose spines into partials
```

## Phase 7: ADR-0189 application - compression, claim update, flips

**Execution mode: inline.** This is the deferred final transaction of ADR-0189's single
declared operation (`update rendering/workflow-skill-templates:deliberate-subagent-model-selection`).
Per the deferred-flip contract, it executes under the terminal-review flow: after the
terminal review of Phases 1-6 settles, `awf-reviewing-impl` lands this transaction (with
`awf-adr-lifecycle` supplying the mechanics) immediately before worktree integration
follow-through; the plan's `status:` co-flips here. Everything below is ONE commit -
claim update, template compression, test literals, rendered output, ADR flip, plan flip,
INDEX regen - because `awf check` validates claim provenance against the ADR's
operations.

- [ ] **Task 7.1: Create the model-selection partial and its guide consumer.** Create
  `templates/partials/model-selection.md` carrying the full tier definitions exactly as
  the guide states them today (source the text from
  `templates/agents-doc/AGENTS.md.tmpl:58`: "Every governed subagent dispatch chooses
  the smallest model expected to complete reliably: `small` is for narrow, mechanical,
  low-ambiguity work; `standard` is for substantive but bounded work; and `large` is for
  broad, intricate, cross-cutting, or high-consequence work. Uncertainty, failed
  reasoning, or widened scope requires reconsideration and possible escalation. A
  runtime with model selection chooses explicitly; an unsupported runtime uses its
  harness default and notes that explicit selection is unavailable."). Replace the
  guide template's inline paragraph with the include directive on its own line inside
  the existing `workflow` section (the section id and marker stay). The include regex
  (`internal/render/include.go:13`) allows only whitespace before the directive, and
  `templates/agents-doc/AGENTS.md.tmpl:58` currently opens
  `{{end}}{{ end }}Every governed subagent dispatch...` - use exactly:
  `{{end}}{{ end -}}` on the preceding line (the trim marker swallows the newline),
  then the include directive on its own line, so no blank line is introduced. The
  guide is the partial's only consumer; no skill site includes it (ADR-0189 decision
  1). Post-check: rendered `AGENTS.md` workflow section is byte-identical for this
  paragraph.
- [ ] **Task 7.2: Compress every governed dispatch section.** Batch task over the 13
  governed dispatch sections (26 template branch copies) in
  `templates/skills/{brainstorming,executing-plans,exploring,reviewing-adr,reviewing-impl,reviewing-plan,reviewing-plan-resync,subagent-driven-development}/SKILL.md.tmpl`.
  Representative (non-Pi branch of a review skill): replace the full block with the
  compressed rule: "Choose the smallest reliable tier - `small` (narrow, mechanical),
  `standard` (substantive but bounded), or `large` (broad or high-consequence) -
  escalating after uncertainty, failed reasoning, or widened scope; select the smallest
  reliable target-native model explicitly, or use the harness default and note that
  explicit selection is unavailable. Full tier definitions: the agent guide's workflow
  section." Pi-branch edge: same first clause, branch tail instead reads "omit the
  `model` field to use configured role routing, overriding deliberately with an exact
  tier reference. Never pass `default`, `auto`, or `inherit parent` as a model value."
  (The prohibition sentence is retained: it guards values the Pi runtime rejects, per
  `rendering/pi-workflows:pi-subagent-model-routing`, and the branch rule must keep
  its semantic content exactly.) Exact final wording is fixed by Task 7.3's pinned
  literals - author
  7.2 and 7.3 together so template text and test literals are written from one string
  set; the branch rule sentences are claim-load-bearing and must keep their semantic
  content exactly. Exhaustive site list: the `deliberateSelectionDispatches` table in
  `internal/project/subagent_model_selection_test.go` enumerates every section; the
  batch covers precisely those sites, both branches each. Post-check: `grep -rn
  'smallest model expected to complete reliably' templates/skills/` returns no output
  (the full phrase lives only in the partial and the guide); every compressed site
  still names the three tiers and the escalation trigger.
- [ ] **Task 7.3: Refresh the pinned test literals.** In
  `internal/project/subagent_model_selection_test.go`, replace the
  `deliberateSelectionCommon` clause set (lines 23-29) and the
  `deliberateSelectionPiRule`/`deliberateSelectionNonPiRule` strings (lines 32-33) with
  the compressed forms from Task 7.2, and add a pin asserting the full definitional
  paragraph renders in the guide (assert the rendered `AGENTS.md` contains the Task 7.1
  paragraph). The proof marker comment
  (`invariant: rendering/workflow-skill-templates:deliberate-subagent-model-selection`)
  stays on the test. The `deliberateSelectionDispatches` section table is unchanged
  (same sections, same branches). Post-check: `go test ./internal/project/ -run
  SubagentModelSelection` passes against the compressed templates and fails if any
  dispatch section loses its tier names, escalation trigger, or branch rule.
- [ ] **Task 7.4: Trim the working-with-awf convention part (ADR-0189 decision 4).** In
  `.awf/parts/working-with-awf/config-and-overrides.md:13`, replace the full
  model-selection restatement (plus its extra Pi sentence) with the compressed rule and
  a reference to the agent guide's workflow section. Rendered `docs/working-with-awf.md`
  regenerates in this commit; post-check: `grep -c 'smallest model expected to complete
  reliably' docs/working-with-awf.md` returns 0.
- [ ] **Task 7.5: Apply the claim update.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, revise
  `invariant: deliberate-subagent-model-selection`. Its current prose, quoted in full:
  "Every final governed subagent dispatch chooses the smallest model expected to
  complete reliably from the semantic small, standard, and large tiers and reconsiders
  escalation after uncertainty, failed reasoning, or widened scope. Pi uses configured
  role routing only by omitting the model field and overrides deliberately with an
  exact tier reference; other targets select a target-native model explicitly where
  supported and otherwise use the harness default with a visible unsupported-selection
  note. Generic rendered guidance contains no Pi tool name, provider-specific model
  reference, price, context limit, or registry catalog, and every affected template
  renders coherently with empty variables."
  Keep that prose verbatim and append this exact sentence: "Each governed dispatch
  section carries the compressed tier-and-escalation rule with its target branch rule,
  and the full tier definitions render once per target in the agent guide's workflow
  section, sourced from the shared model-selection partial." Add
  `Revised-by: ADR-0189` under `Origin: ADR-0173`. `Backing: test` and the proof marker
  are unchanged.
- [ ] **Task 7.6: Flip the statuses.** Append to ADR-0189's Status history the direct
  Implemented event with its batch state sequence and the Applied operation
  (`update rendering/workflow-skill-templates:deliberate-subagent-model-selection`),
  per the `awf-adr-lifecycle` contract for a direct Implemented transaction; set
  `status: Implemented` in its frontmatter. Flip this plan's frontmatter to
  `status: Implemented`, recording any execution findings in Notes. Run `./x render` so
  `docs/decisions/INDEX.md` and all rendered outputs regenerate.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction
  (topic part, ADR, plan, templates, test, config part, every rendered output);
  `awf check --staged` clean (this is the commit where claim provenance is validated
  against ADR-0189's operation); `./x gate` green; commit:

```commit
feat(invariants): compress dispatch guidance (implements 0189)
```

## Verification

Beyond the per-phase gates, after Phase 7:

- `./x check` and `awf check` clean in the worktree; `./x gate` green.
- `grep -rn 'smallest model expected to complete reliably' templates/skills/` returns no
  output; the phrase renders exactly once in `AGENTS.md` and once in
  `docs/working-with-awf.md`'s referenced guide location (via the guide, not a second
  full copy).
- `grep -rn 'five lenses\|other three\|prior concerns' templates/ internal/` and
  `grep -rn 'taskSkillRows' templates/ internal/ .awf/` return no output (this plan
  file legitimately still carries the old token as history; `internal/evals/chain_test.go`
  keeps its correct "task skills" comments per Task 4.3).
- `grep -ln 'task skill' templates/skills/*/SKILL.md.tmpl` returns only the bugfix
  template.
- The sundial example re-renders clean (`./x check` covers it) and
  `grep -rn 'sundial new plan' examples/sundial/` returns no output.
- ADR-0189 and this plan both read `status: Implemented`; `docs/decisions/INDEX.md`
  reflects it and was generated, not hand-edited.

## Notes

- Deferred, needs its own decision per ADR-0189 decision 2's bound: merging the
  path-identification step pairs in the review skills (audit finding, D7) requires
  dropping a catalog `Sections` entry and is out of this plan's scope.
- Deferred: the audit's A9 note that executing-plans' `tdd-opt-in` catalog section
  renders empty by default is left as-is (the section is the adopter's override point);
  only the catalog relationship entries changed here.
- Execution-time re-verification owed (pre-0187 findings not re-checked at HEAD):
  debugging symptom-list overlap (Task 5.3), refactor-coupling-audit Test-coupling
  planning rule section (Task 5.3), writing-plans Notes duplicate (Task 5.3, skip if
  weak).
