---
format: plan-v2
date: 2026-08-06
adrs: [authority-guided-review-remediation]
status: Implemented
---
# Plan: Implement Authority-Guided Review Remediation

## Goal

Render one authority-guided remediation boundary across the shared review spine and all four reviewing skills, so plan, ADR, resync, and implementation review apply authority-preserving corrections autonomously, including after the single verify pass, and reserve `user-decision` for a cited deviation from settled user-approved design or an active current-state claim. Do not add a policy schema, automated classifier, severity router, extra verify pass, new workflow stage, or weaker verification path, and do not change the report-only reviewer boundary, consensus adherence, required gates, or the mandatory grounded-design and settled-ADR approvals.

## Architecture summary

The contract has two audiences that never share an output file: the three reviewer agents, which are spliced from `templates/partials/review-spine-head.md`, and the four reviewing skills, which are not. `review-spine-head.md` stays the single semantic home for finding classification and is where the authority-deviation criterion is authored. A new variable-free partial `templates/partials/review-remediation-autonomy.md` owns the dispatcher-side routing obligation and, because the skills never include the spine, restates that criterion for its own reader. The restatement is deliberate and bounded: three clauses are shared, the stop criterion, the non-trigger enumeration, and the phrase naming plan resync's return edge as the sole exception to the same-artifact no-loop rule. This plan pins each verbatim once, and Task 1.1 asserts both homes render the identical clause, so they cannot drift apart in a later edit. The dispatcher partial does not restate the `mechanical` and `reasoned` definitions the spine owns.

The include engine rejects nested includes and any partial containing a section marker, so `review-spine-head.md` carries its own classification prose and never includes the new partial. The new partial scopes its stop rule to review findings, so where it renders beside the existing `implementation-autonomy` partial in `reviewing-impl`, the two read as complementary rather than as a contradiction over whether a design fork is by itself an escalation.

Each reviewing skill then loses its automatic residual promotion sentence; plan resync additionally widens its governed ADR amendment and review return edge to cover residual findings. One focused rendering test in `internal/project/spine_test.go` proves single direct inclusion, the reviewer-side classification change across all three reviewer agents, authority-guided initial and residual routing in all four skill projections, the widened resync return edge, absence of the retired escalation sentences, and coherent empty-variable rendering.

The ADR declares one `add` operation, which a one-operation ADR could carry through the direct `Proposed` to `Implemented` transaction. This plan uses the incremental path instead because `Implemented` is terminal and freezes the ADR body: deferring it keeps the record amendable through implementation review, and it matches the governed route in the ADR review skill, under which `effort-workflow` owns the status-only terminal flip after assurance settles.

Before dispatch, the parent establishes a clean and green managed-worktree baseline: `git status --short` prints no output, `./x check` reports clean, and `./x gate` passes. The phase owner receives that verified baseline and works only in `/home/hypno/Projects/agentic-workflows/.awf/worktrees/review-remediation-autonomy` until returning its completed report.

## Phase 1: Ship authority-guided review remediation

**Execution mode: subagent-driven.**

Completes: ["escalation-boundary", "residual-routing", "resync-edge", "preserved-controls", "authority-applied"]

### Task 1.1: Pin the review-remediation contract before production changes
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]

In `internal/project/spine_test.go`, add `TestAuthorityGuidedReviewRemediation` carrying the proof marker `// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestAuthorityGuidedReviewRemediation)` in its doc comment block. Write it to fail before Tasks 1.2 and 1.3 land.

Read `partials/review-remediation-autonomy.md` from `templates.FS` and assert it contains no `{{`, no line whose trimmed form starts with `#`, and none of the substrings `awf:section`, `awf:end`, or `awf:include`. Those are exactly the three tokens `ExpandIncludes` rejects in a partial, so the assertion matches the engine's actual guard.

For each of `skills/reviewing-plan/SKILL.md.tmpl`, `skills/reviewing-adr/SKILL.md.tmpl`, `skills/reviewing-plan-resync/SKILL.md.tmpl`, and `skills/reviewing-impl/SKILL.md.tmpl`, read the embedded source and assert `strings.Count(raw, "<!-- awf:include review-remediation-autonomy -->")` equals 1.

Render all four skills through `renderSkillGolden` in two variants: a configured variant with `prefix` `example`, `vars` carrying `gateCmd` `./x gate`, `layout` from `testLayout()`, `commitScopes` `` `docs(plans)` ``, `skills` enabling `effort-workflow`, `adr-lifecycle`, and `reviewing-impl`, and `targetSubagentTools` true; and an empty variant with `prefix` `example`, empty `vars`, `testLayout()`, empty `data`, and empty `skills`. Call `assertNoLeaks` on every output. Assert each output carries the dispatcher semantics: mechanical and reasoned corrections are applied autonomously; the review spine defines the classification and the dispatcher routes rather than redefines it; a newly material load-bearing choice routes through the existing grounded-design or ADR workflow and pauses only at that workflow's mandatory approval boundary; exactly one fresh verify-pass dispatch is retained; every residual finding is diagnosed under the same boundary and corrected without another same-artifact review loop; and a consensus deviation remains a user decision.

Assert the two shared clauses render byte-identically in both homes. In all four skill outputs and in all three reviewer-agent outputs, assert the presence of the exact stop criterion `every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim` and the exact non-trigger enumeration `competing clean options, severity, structural character, and the fact that a finding survived a prior correction`. These two literals are the drift guard between `review-spine-head.md` and `review-remediation-autonomy.md`; assert them as exact substrings, not paraphrases. The leading word `ambiguity` is deliberately outside the pinned enumeration because the spine sentence opens with it capitalized while the partial carries it mid-sentence; both homes still name ambiguity, but only the case-stable tail is pinned.

Assert the dispatcher partial scopes its stop rule to review findings by requiring the phrase `A review finding stops the workflow only when` in all four skill outputs, so the reviewing-impl output cannot be read as overriding the implementation-side stop list that `templates/partials/implementation-autonomy.md` owns under the active `authority-guided-implementation-autonomy` claim.

Assert the replacement re-review-loop sentences positively, not only by absence of their retired predecessors. In the rendered `reviewing-plan`, `reviewing-adr`, and `reviewing-impl` outputs for both variants, assert the literal `Diagnose every residual finding under the authority-guided remediation boundary above`. Without this, deleting a re-review-loop sentence outright rather than replacing it would leave every other assertion green while `dod: residual-routing` went unmet, because the positive residual semantics otherwise all arrive from the shared partial.

Assert the widened resync return edge. In the rendered `reviewing-plan-resync` output for both variants, assert the phrases `a finding, initial or residual, implicates the ADR itself`, `the amendable decision text is wrong`, `while the implicated ADR remains amendable`, and `a new resync invocation follows under its own one-verify-pass bound`, and assert the revised notes bullet phrase `whether it surfaces on initial dispatch or in the verify pass`. This is the only proof of `dod: resync-edge`.

Assert the clauses the settled ADR requires but that no other assertion reaches. In all four skill outputs, assert the exact literal `sole exception to the same-artifact no-loop rule`, which is the third clause shared byte-identically between the two prose homes and carries the same drift guard as the other two. In the three reviewer-agent outputs, assert the ADR carve-out literal `is not an unauthorized deviation merely because its proposed future state differs from current state` and the reviewer-side new-authority trigger literal `would make a new load-bearing choice material outside approved durable boundaries`.

Render `adr-reviewer`, `plan-reviewer`, and `code-reviewer` through `renderAgentGolden` with the same data shapes those agents already use in `TestAdrReviewerAgent`, `TestPlanReviewerAgent`, and `TestCodeReviewerAgent`. Assert each carries the revised `user-decision` criterion and the non-trigger sentence, and that `mechanical`, `reasoned`, `user-decision`, and `suggested_fix` all survive. Render all three a second time with `prefix` `example`, empty `vars`, `testLayout()`, and empty `data`, and call `assertNoLeaks` on each of those outputs; without this second render the claim's empty-variable coherence clause would reach past what this marker exercises, since the populated shapes alone prove nothing about unset interpolation in the revised spine.

Assert these retired phrases are absent from every rendered skill and agent output above and from each of the four embedded skill sources: ``Escalate any residual structural findings as `user-decision` items``, `the step-2 return edge applies to initial-dispatch findings only`, `o not loop further without explicit user direction`, `a genuine design fork or unresolved ambiguity that should not be decided unilaterally`, ``present a genuine unresolved `user-decision` fork or consensus deviation and stop``, and `(return edge, step 2)`. The loop phrase is deliberately pinned from its second character: it occurs lowercase in the plan and ADR skills and capitalized in resync, and the case-stable tail catches both.

Add the same proof marker line, naming its own unit, to the doc comment of `TestConditionalVerifyPass`: `// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestConditionalVerifyPass)`. That test owns the retained single-verify-pass bound; do not extend it to clauses it does not exercise. Assign no third marker: the claim body in Task 1.4 must stay within what these two units actually exercise.

Run `go test ./internal/project -run 'TestAuthorityGuidedReviewRemediation|TestConditionalVerifyPass|TestAdrReviewerAgent|TestPlanReviewerAgent|TestCodeReviewerAgent|TestReviewingPlanTemplate|TestReviewingAdrTemplate|TestReviewingPlanResyncTemplate|TestReviewingImplTemplate'`; before Tasks 1.2 and 1.3 it fails on the missing partial, missing includes, and unchanged classification prose, and after them it passes.

### Task 1.2: Move the classification boundary into the shared spine
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:preserve-review-controls"]

In `templates/partials/review-spine-head.md`, under `## Classification rules`, keep the `mechanical` and `reasoned` bullets byte-for-byte and replace the `user-decision` bullet with:

```
- **user-decision**: every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim; cite the affected authority and name the deviation it would require.
```

Keep the existing `Severity is informational only; the dispatching skill routes by classification kind.` line unchanged and add one paragraph directly after it:

```
Ambiguity, competing clean options, severity, structural character, and the fact that a finding survived a prior correction do not by themselves make a finding a user decision. When no settled authority is affected, the finding is mechanical or reasoned. An ADR that intentionally declares an active-claim change is not an unauthorized deviation merely because its proposed future state differs from current state; check it against the settled design and its declared State changes instead. When a correction would make a new load-bearing choice material outside approved durable boundaries, say so in `suggested_fix` rather than classifying the finding as a user decision for that reason.
```

The second sentence of that paragraph is the carve-out ADR Decision 5 requires: without it, the `user-decision` bullet's reference to an unauthorized active-claim change would make every ADR declaring an `update` or `remove` operation a user decision by construction, which is exactly the reading the settled design rejects. The third is the reviewer-side trigger the dispatcher's new-authority routing rule depends on; it is a reporting obligation only and adds no apply, commit, or re-review directive.

Leave the finding schema, the specific-location requirement, and the whole `## Consensus adherence` section unchanged; a consensus deviation stays a `user-decision` finding under its own rule. Add no directive to apply fixes, commit, or re-review, so the `reviewers-report-only` claim stays true.

Create `templates/partials/review-remediation-autonomy.md` as the only full statement of the dispatcher-side obligation. Keep it target-neutral, variable-free, free of headings, free of the `awf:section`, `awf:end`, and `awf:include` tokens, and publication-safe. Open with a bold non-heading label in the style of `templates/partials/implementation-autonomy.md` so direct inclusion never interrupts a consumer's section structure or numbered procedure.

It must state that the dispatching workflow applies mechanical corrections directly and reasoned corrections with a concise rationale, autonomously; that the review spine is the single semantic home of the classification and this workflow routes it rather than redefining it; that a finding making a new load-bearing choice material outside approved durable boundaries routes through the existing grounded-design or ADR workflow and pauses only at that workflow's mandatory approval boundary before the new authority is adopted; that exactly one fresh verify-pass dispatch is retained after reasoned fixes or a user-approved ruling, every residual finding is diagnosed under the same boundary, authority-preserving mechanical and reasoned residual corrections are applied, the applicable verification runs, and its disposition is reported without dispatching another same-artifact review loop; and that a consensus deviation remains a user decision.

The no-loop rule must carry its one exception, or it renders directly above a resync body that legitimately ends and re-invokes over the same plan. State that plan resync's governed ADR amendment and review return edge is the `sole exception to the same-artifact no-loop rule`: that resync ends, the ADR is amended and independently reviewed, and a new resync invocation follows under its own one-verify-pass bound. The quoted phrase is the third verbatim clause below.

Three clauses must appear verbatim, because Task 1.1 pins them as exact literals. The first two are byte-identical to the spine wording above; the third is pinned across the four skill outputs:

- the non-trigger enumeration `competing clean options, severity, structural character, and the fact that a finding survived a prior correction`, used in a sentence that also names ambiguity and states that none of them ever transfers the choice to the user; and
- the stop criterion `every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim`; and
- the exception phrase `sole exception to the same-artifact no-loop rule`.

The stop sentence must open with the exact phrase `A review finding stops the workflow only when`, so the rule is visibly scoped to review findings. That scoping is load-bearing: this partial renders directly beside `implementation-autonomy`, whose own stop list includes a genuine unresolved design fork and is pinned by `TestAuthorityGuidedImplementationAutonomy` under an active claim this ADR does not change. Require the stop sentence to also direct that the affected authority be cited.

Scoping alone leaves the two stop lists readable as rivals, because the adjacent implementation stop list names an unresolved design fork without qualification while this partial denies that competing clean options transfer the choice. Add one reconciling clause stating that a review finding offering competing clean options inside approved durable boundaries is not the unresolved design fork of that adjacent list: it is delegated detail this workflow resolves, and a choice that is genuinely load-bearing instead routes through the grounded-design or ADR workflow, which supplies the pause. This restates the settled decisions rather than adding new policy, so it stays inside the ADR's single declared operation and changes nothing in `implementation-autonomy` or its claim.

Do not restate the `mechanical` and `reasoned` definitions the spine owns, and do not introduce a severity rule, classifier, or policy schema.

### Task 1.3: Project the boundary through the four reviewing skills
Latitude: exact
Applying: ["authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]

Add exactly one `<!-- awf:include review-remediation-autonomy -->` line to each of the four reviewing skills, placed outside every `awf:section` block so no section marker is disturbed:

- `templates/skills/reviewing-plan/SKILL.md.tmpl`: between the `<!-- awf:end -->` that closes the `when-fires` section and the `## Procedure` heading.
- `templates/skills/reviewing-adr/SKILL.md.tmpl`: between the `<!-- awf:end -->` that closes the `when-fires` section and the `## Procedure` heading.
- `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`: between the `<!-- awf:end -->` that closes the `when-fires` section and the `## Procedure` heading.
- `templates/skills/reviewing-impl/SKILL.md.tmpl`: immediately after the existing `<!-- awf:include implementation-autonomy -->` line, separated by one blank line, keeping both includes at that position and outside any section.

In the `classify-route-findings` section of `templates/skills/reviewing-plan/SKILL.md.tmpl`, `templates/skills/reviewing-adr/SKILL.md.tmpl`, and `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, replace the bullet `   - **user-decision**: present to the user and wait.` with:

```
   - **user-decision**: present to the user with the cited affected authority and wait.
```

In the `classify-route-findings` section of `templates/skills/reviewing-impl/SKILL.md.tmpl`, replace the trailing clause ``present a genuine unresolved `user-decision` fork or consensus deviation and stop.`` with:

```
present a `user-decision` finding with the cited affected authority, or a consensus deviation, and stop.
```

Leave the rest of that numbered step, including its diagnose-then-route-by-classification opening, unchanged.

In the `re-review-loop` section of `templates/skills/reviewing-plan/SKILL.md.tmpl` and `templates/skills/reviewing-adr/SKILL.md.tmpl`, replace the trailing sentence ``Escalate any residual structural findings as `user-decision` items; do not loop further without explicit user direction.`` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above, apply the authority-preserving mechanical and reasoned residual corrections, and report their disposition. Stop only for a residual finding that remains a true user decision, and dispatch no further same-artifact review pass.
```

Leave the preceding conditional-dispatch prose in both sections untouched so the `TestConditionalVerifyPass` ordered phrases stay green.

In `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, make the governed return edge cover residual findings. In the `classify-route-findings` section, change the return-edge opening from `when a finding implicates the ADR itself` to `when a finding, initial or residual, implicates the ADR itself` and restate its gating parenthetical from ``(the plan is right and the still-`Proposed` decision text is wrong)`` to `(the plan is right and the amendable decision text is wrong)`. ADR Decision 4 gates this edge on amendability, not on the `Proposed` status specifically, and an ADR at `Implementing` is still amendable; leaving the two mentions with different conditions would render one edge with two availability rules. No test pins the `still-Proposed` wording. Keep the rest of that paragraph, including the amend, re-review, re-resync loop, unchanged. In the `re-review-loop` section, replace the trailing sentence ``Escalate any residual structural findings as `user-decision` items, an ADR-implicating residual included: the step-2 return edge applies to initial-dispatch findings only. Do not loop further without explicit user direction.`` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above and apply the authority-preserving mechanical and reasoned residual corrections. An ADR-implicating residual takes the step-2 return edge while the implicated ADR remains amendable: this resync ends, the ADR is amended and independently reviewed, and a new resync invocation follows under its own one-verify-pass bound. Stop only for a residual finding that remains a true user decision, and dispatch no further same-artifact verify pass inside this invocation.
```

In the `notes` section of the same file, replace the first bullet `- Resync fixes never edit the repository beyond the plan file; ADR-implicating findings route through the ADR amendment + review skills instead (return edge, step 2).` with:

```
- Resync fixes never edit the repository beyond the plan file; an ADR-implicating finding routes through the ADR amendment and review skills instead, whether it surfaces on initial dispatch or in the verify pass.
```

In the `re-review-loop` section of `templates/skills/reviewing-impl/SKILL.md.tmpl`, replace the trailing sentence `Classify any residual finding, apply authority-determined residual fixes, rerun the gate and audit, and stop on any unresolved user decision. Do not add another review loop.` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above, apply the authority-preserving residual fixes, rerun the gate and audit, and stop only for a residual finding that remains a true user decision. Do not add another review loop.
```

In the repo-local part `.awf/skills/parts/reviewing-impl/run-audit.md`, replace the clause `so resolve it or escalate it as a user-decision item` with:

```
so resolve it, escalating only when its remedy would reach the authority-deviation boundary above
```

This part renders into the same `reviewing-impl` skill body a few sections below the newly expanded partial, and its unqualified user-decision escalation of an audit `Error` contradicts the boundary that body now states; a missing changelog entry is a mechanical fix. Change nothing else in that part: the repo-local tooling, the ADR-0073 reference, the `Warning` advisory line, and the no-gate note stay as they are.

Leave the report-only reviewer instructions, the consensus-adherence paste, the model-tier prose, the shipped audit step in the template, and every hand-off route unchanged in all four skills. Change no mandatory approval: `templates/skills/reviewing-adr/SKILL.md.tmpl` step 8 keeps the settled-ADR approval stop verbatim, and no skill gains or loses a checkpoint.

Run `go test ./internal/project -run 'TestAuthorityGuidedReviewRemediation|TestConditionalVerifyPass|TestAdrReviewerAgent|TestPlanReviewerAgent|TestCodeReviewerAgent|TestReviewingPlanTemplate|TestReviewingAdrTemplate|TestReviewingPlanResyncTemplate|TestReviewingImplTemplate|TestUnsetFallbackRenders|TestMaintainableCodeReviewLenses'`; it passes. `TestUnsetFallbackRenders` carries both the report-only and the unset-fallback proofs for the reviewer surfaces, and `TestMaintainableCodeReviewLenses` is the plan-review lens regression guard over the revised spine; there is no test named `TestReviewersReportOnly`.

### Task 1.4: Apply current authority, changelog, and ADR lifecycle events
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]

In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, add one claim placed directly after the existing `authority-guided-implementation-autonomy` claim so the two autonomy boundaries read together. Use the heading line, trailer lines, and blank-line spacing already used by the sibling claims in that file, with the claim id `authority-guided-review-remediation`, `Origin: ADR-authority-guided-review-remediation`, `Backing: test`, and no `Revised-by` line.

Its body must assert, in present tense, only what the two markers assigned in Task 1.1 actually exercise: the shared review spine is the single semantic classification home and reserves `user-decision` for a finding where every viable correct remediation would contradict a settled user-approved design or decision or require an unauthorized active-claim change, with the affected authority cited; ambiguity, competing clean options, severity, structural character, and survival of a prior correction do not transfer the choice; one variable-free shared prose partial is directly included exactly once by plan review, ADR review, plan resync, and implementation review, applies mechanical and reasoned corrections autonomously, and scopes its stop rule to review findings; a newly material load-bearing choice routes through the existing grounded-design or ADR workflow and pauses at that workflow's mandatory approval boundary; exactly one fresh verify pass is retained and every residual finding is diagnosed under the same boundary without another same-artifact review loop apart from the single governed exception; that exception is plan resync's ADR amendment and review return edge, which covers initial and residual findings while the implicated ADR remains amendable, ends the resync, and is followed by a new invocation under its own one-verify-pass bound; the shared spine also carries the carve-out that an ADR intentionally declaring an active-claim change is not an unauthorized deviation; and every affected skill and reviewer template renders coherently under empty variables. Word the no-loop clause and its exception in one breath: applied claim prose cannot be edited later without a new declared operation, so a universal whose own next clause contradicts it would be a durable defect. Do not assert that reviewers stay report-only, that consensus deviations stay user decisions, or that the mandatory approvals are unchanged: those clauses belong to the existing `reviewers-report-only`, `memory-log-consumer-coverage`, and `mandatory-approval-boundaries` claims and are not exercised by either assigned marker.

Change no other claim: the ADR declares one `add` operation and no `update`.

Update `changelog/CHANGELOG.md` under `## [Unreleased]` / `### Features` with one concise adopter-facing entry stating that plan, ADR, resync, and implementation review now apply authority-preserving corrections autonomously, including after the single verify pass, and ask the user only when a correction would deviate from settled design or an active current-state claim.

Transition `docs/decisions/authority-guided-review-remediation.md` from Proposed to Implementing in the same transaction as the claim addition. Append an Implementing status event dated on execution day with the current content digest, then append one Applied event whose operation list is exactly the single add operation the ADR's State changes section declares, written in the same backticked-claim-id form that section uses.

Obtain the digest through the governed placeholder workflow: insert 64 zeros, run `./x check`, read the reported computed digest for `docs/decisions/authority-guided-review-remediation.md`, replace the placeholder with it exactly, and rerun until `./x check` no longer reports a content-sha256 mismatch for that file. Do not precompute or guess the digest, and do not expect a whole-tree clean result at this point: the lock, topic doc, decision index, and every rendered skill and agent output are still stale until Task 1.5 renders them. Task 1.5 owns the clean whole-tree check.

Leave the ADR at `Implementing` and this plan at `status: Proposed`; `effort-workflow` owns both later status-only `Implemented` flips after terminal implementation review settles.

### Task 1.5: Render outputs, review generated prose, and reach green
Kind: batch
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]
Paths: [".awf/awf.lock", "docs/topics/rendering/workflow-skill-templates.md", "docs/decisions/INDEX.md", "glob:.claude/skills/awf-reviewing-*/SKILL.md", "glob:.pi/skills/awf-reviewing-*/SKILL.md", "glob:.claude/agents/*.md", "glob:.pi/agents/*.md"]
Representative: `templates/partials/review-remediation-autonomy.md` expands once into each of the eight rendered reviewing-skill outputs under `.claude/skills/awf-reviewing-*/SKILL.md` and `.pi/skills/awf-reviewing-*/SKILL.md`, while the revised `review-spine-head.md` classification bullet reaches `adr-reviewer.md`, `plan-reviewer.md`, and `code-reviewer.md` under both `.claude/agents/` and `.pi/agents/`.
Edge: `docs/decisions/INDEX.md` changes only its single in-flight status token from `(Proposed)` to `(Implementing)` for this ADR, and `.claude/agents/explorer.md`, `.pi/agents/explorer.md`, `.claude/agents/grounding-checker.md`, `.pi/agents/grounding-checker.md`, `.claude/agents/implementer.md`, and `.pi/agents/implementer.md` fall inside the declared agent globs but must show no diff, since neither the spine partial nor the new partial reaches them.
Post-check: After `./x render`, run `git diff --check` and require no output. Then run the absence probe `rg -n -i -e 'Escalate any residual structural findings' -e 'the step-2 return edge applies to initial-dispatch findings only' -e 'do not loop further without explicit user direction' -e 'a genuine design fork or unresolved ambiguity that should not be decided unilaterally' -e 'present a genuine unresolved .user-decision. fork or consensus deviation and stop' templates/partials/review-spine-head.md templates/skills/reviewing-plan/SKILL.md.tmpl templates/skills/reviewing-adr/SKILL.md.tmpl templates/skills/reviewing-plan-resync/SKILL.md.tmpl templates/skills/reviewing-impl/SKILL.md.tmpl .claude/skills/awf-reviewing-*/SKILL.md .pi/skills/awf-reviewing-*/SKILL.md .claude/agents/*.md .pi/agents/*.md` and require exit status 1 (no matches) rather than 2 (traversal or path error). Establish that the probe reached every intended file by rerunning the identical path list as `rg -l -e 'user-decision' <same paths>` and requiring the listed set to contain all five template sources, all eight `awf-reviewing-*/SKILL.md` outputs across both targets, and `adr-reviewer.md`, `plan-reviewer.md`, and `code-reviewer.md` under both `.claude/agents/` and `.pi/agents/`; a short list means a glob silently matched nothing and the absence result proves nothing. Finally run `git diff --name-only` and confirm every changed path resolves inside this task's declared `Paths` population or inside the authored set changed by Tasks 1.1 through 1.4, and that every changed generated output is attributable to a named source or current-state mutation.

Run `./x render`. Then perform the focused semantic rendering review at each affected output boundary. Read `.claude/skills/awf-reviewing-plan/SKILL.md`, `.pi/skills/awf-reviewing-plan/SKILL.md`, `.claude/skills/awf-reviewing-plan-resync/SKILL.md`, `.claude/skills/awf-reviewing-adr/SKILL.md`, `.claude/skills/awf-reviewing-impl/SKILL.md`, `.claude/agents/plan-reviewer.md`, and `.pi/agents/adr-reviewer.md` and confirm each reads as one coherent instruction rather than two: the expanded autonomy paragraph must not sit adjacent to a surviving fragment that still promotes residual findings automatically, the numbered procedure must remain correctly ordered around the inserted include, and the resync return edge must read consistently across all three of its mentions in that output: the expanded partial's exception clause, the step-2 edge, and the step-4 residual sentence. Confirm the partial's exception clause and the step-4 sentence read as one rule stated once and then applied, not as redundant restatement. In the reviewing-impl output specifically, read the two adjacent expanded partials together and confirm the implementation stop list and the review stop rule read as complementary scopes rather than as a contradiction about design forks, and read the repo-local audit step against the new stop rule: an audit `Error` must read as a tooling gate the review resolves, escalating only at the authority-deviation boundary. Confirm literals such as `<literal-placeholder>`, `<slug>`, `<name>`, and `<path> (missing)` remain intentional generic placeholders rather than unresolved template values, and that no output contains an unresolved no-value token.

Run `go test ./internal/project ./internal/evals`, `./x check`, and `./x gate`; each reaches a clean or passing terminal state.

### Phase close

Return every implementation deviation in the completed child report and do not edit this plan. Leave plan frontmatter `status: Proposed` and the ADR `status: Implementing`; the parent supplies the completed report to report-only phase review and owns any focused Notes-and-findings settlement commit before checkpointing. Stage the complete implementation transaction explicitly, including the new partial, the revised spine partial, the four reviewing-skill templates, the repo-local `.awf/skills/parts/reviewing-impl/run-audit.md` part, the test file, the current-state part, the ADR lifecycle events, the changelog, the lock, `docs/decisions/INDEX.md`, `docs/topics/rendering/workflow-skill-templates.md`, and every rendered skill and agent output, but excluding this parent-owned plan. Run `./awf check staged` and `./x gate`; both pass. Create the one phase-closing commit:

```commit
feat(rendering): add authority-guided review remediation
```

The ADR lifecycle skill's subject template for an explicit application transaction ends with `(applies NNNN batch)`. That suffix is deliberately omitted here: the ADR is still pending a number, so no `NNNN` exists yet, and the slug-based equivalent would push the subject past the 72-character bound the commit gate enforces. No check requires the suffix.

## Definition of done

- `dod: escalation-boundary` The shared review spine defines `user-decision` as a cited deviation from settled user-approved design or an active current-state claim, and explicitly denies that ambiguity, competing clean options, severity, structural character, or survival of a prior correction transfers the choice; all three reviewer agents render that definition, and both prose homes carry the two shared clauses byte-identically.
- `dod: residual-routing` Plan, ADR, resync, and implementation review each include the shared remediation partial exactly once, retain exactly one fresh verify pass, diagnose residual findings under the same boundary, and dispatch no second same-artifact review pass; the automatic residual-escalation sentences and the fork-based routing clause are gone from every source and rendered output.
- `dod: resync-edge` Plan resync's ADR amendment and review return edge is available to initial and residual findings while the implicated ADR remains amendable, its notes describe it that way, and `TestAuthorityGuidedReviewRemediation` asserts both.
- `dod: preserved-controls` Reviewers stay report-only, consensus deviations stay user decisions, required gates and verification are unchanged, the grounded-design and settled-ADR approvals stand verbatim, the shared review partial scopes its stop rule to review findings so the implementation-autonomy stop list is untouched, and every affected skill and agent renders coherently under empty variables with no unresolved token.
- `dod: authority-applied` The new current-state claim is added with test backing that matches its two assigned proof markers and no wider, its declared operation is Applied, the ADR is `Implementing` and this plan `Proposed` pending deferred terminal closure, and `./x render`, `./x check`, `go test ./internal/project ./internal/evals`, and `./x gate` all reach a clean or passing terminal state.

## Notes

Record inline deviations immediately. For this delegated phase, preserve the child report as phase-review input and reconcile reported deviations plus review findings in one focused parent settlement commit before checkpointing or later execution. The settled design prohibits a policy schema, automated classifier, severity-based routing, an additional verify pass, a new workflow stage, a weakened gate or check, and any change to the report-only reviewer boundary or the mandatory grounded-design and settled-ADR approvals.

Plan review settlement: fourteen findings, all mechanical or reasoned, none a user decision. The load-bearing corrections were the missing `docs/decisions/INDEX.md` in the staged and declared population, a `./x check` clean-tree expectation placed before the render that could make it true, a nonexistent `TestReviewersReportOnly` in a run filter that would have passed vacuously, an unproven `dod: resync-edge`, a claim body reaching past its two proof markers, an unresolved contradiction between the new stop rule and the adjacent `implementation-autonomy` stop list, the reviewing-impl routing clause that still used the retired fork framing, and a batch task whose fields covered only its render fan-out.

Implementation review settlement: seven findings, four mechanical and three reasoned, none a user decision. The reviewer mutation-tested `TestAuthorityGuidedReviewRemediation` and found five surviving mutants, so the corrections were mostly to the oracle rather than the prose: the ambiguity non-trigger, three exact routing replacements, the reconciling clause, and the spine's `mechanical` and `reasoned` bullets were all assertable-but-unasserted. The single verify pass mutation-tested again and found four residuals, including that the ambiguity pin had been added only to the skill renders while the claim predicates that clause on the spine, which reaches the reviewer agent bodies; a bare `ambiguity` token also fails to discriminate there because `code-reviewer` names ambiguity elsewhere, so the spine sentence's own opening is pinned instead. All residuals were authority-determined and applied without a further review loop.

Three deviations from `Latitude: exact` plan text, each recorded here rather than silently absorbed:

- Task 1.3's fenced run-audit clause said `authority-deviation boundary above`; the shipped text says `authority-guided remediation boundary above`. Ground: the rendered `reviewing-impl` body never contains the former phrase, so the back-reference resolved to nothing. The ADR keeps `authority-deviation boundary` as its own decision vocabulary.
- Task 1.2's fenced `user-decision` bullet gained the clause `a consensus deviation is a user-decision under the Consensus adherence rule below`. Ground: the bullet states a necessary condition a consensus deviation does not meet, so a reviewer classifying strictly from the Classification rules block could down-classify one, which the Consensus adherence section forbids. This restates the ADR's preserved control rather than adding policy.
- Task 1.2's reconciling-clause directive said the finding is not the unresolved design fork `of that adjacent list`; the shipped partial says `that an implementation stop list names`. Ground: `implementation-autonomy` renders only in `reviewing-impl`, so a positional reference dangles in the other six skill outputs.

Execution deviation: Phase 1 declares subagent-driven ownership, and a commit-capable implementer was dispatched, but the dispatch was interrupted before the child returned a report. Its complete transaction was present and staged in the managed worktree with a clean working tree. Rather than discard verified work or trust it unread, the parent inspected the entire staged diff against every task in the phase, confirmed each `Latitude: exact` replacement and every pinned literal, ran `go test ./internal/project ./internal/evals`, `./x check`, the Task 1.5 absence probe with its reach sentinel over all nineteen paths, `./x gate`, and `./awf check staged`, performed the semantic rendering review at each named output boundary, and created the phase-closing commit itself. No child report exists, so the usual completed-report inventory is replaced by this record. The test-first red state was not directly observed by the parent; it is structurally guaranteed, because `TestAuthorityGuidedReviewRemediation` opens by reading `partials/review-remediation-autonomy.md` through `fs.ReadFile` with `t.Fatal` on error, and that file did not exist before Task 1.2.

Semantic-review observation: the rendered resync skill now states the return edge three times, in the expanded partial's exception clause, in the step-2 edge, and in the step-4 residual sentence, with the one-verify-pass tail repeated between the first and third. This reads as one rule stated and then applied rather than as contradiction, and both the partial clause and the step-4 sentence were required by separate review findings, so the repetition was left as settled.

Resync settlement: eleven findings across the resync round and its verify pass, none implicating the ADR and none a user decision. The load-bearing corrections gave ADR Decision 5's active-claim carve-out executable content in the spine, added the reviewer-side trigger the dispatcher's new-authority routing depends on, carried the same-artifact no-loop rule's single governed exception into the partial and the durable claim body, restated the resync return-edge gate on amendability rather than `Proposed` status, pinned the exception phrase as the third cross-home literal, added positive assertions for the three non-resync replacement sentences, and tightened the repo-local audit fragment whose unqualified user-decision escalation contradicted the new boundary.
