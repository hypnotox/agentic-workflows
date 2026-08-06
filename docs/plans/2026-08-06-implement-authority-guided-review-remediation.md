---
format: plan-v2
date: 2026-08-06
adrs: [authority-guided-review-remediation]
status: Proposed
---
# Plan: Implement Authority-Guided Review Remediation

## Goal

Render one authority-guided remediation boundary across the shared review spine and all four reviewing skills, so plan, ADR, resync, and implementation review apply authority-preserving corrections autonomously, including after the single verify pass, and reserve `user-decision` for a cited deviation from settled user-approved design or an active current-state claim. Do not add a policy schema, automated classifier, severity router, extra verify pass, new workflow stage, or weaker verification path, and do not change the report-only reviewer boundary, consensus adherence, required gates, or the mandatory grounded-design and settled-ADR approvals.

## Architecture summary

Two prose homes divide the contract without duplicating it. `templates/partials/review-spine-head.md` stays the single semantic home for finding classification and is the only place the authority-deviation criterion is defined; it is spliced into the three reviewer agents. A new variable-free partial `templates/partials/review-remediation-autonomy.md` owns the dispatcher-side routing obligation and refers to that classification rather than restating its definition; it is included exactly once by each of the four reviewing skills. The include mechanism rejects nested includes, so the spine partial carries its own classification prose and never includes the new partial.

Each reviewing skill then loses only its automatic residual promotion sentence; plan resync additionally widens its governed ADR amendment and review return edge to cover residual findings. One focused rendering test in `internal/project/spine_test.go` proves single direct inclusion, the reviewer-side classification change across all three reviewer agents, authority-guided initial and residual routing in all four skill projections, absence of the retired escalation sentences, and coherent empty-variable rendering.

The ADR declares one `add` operation. This plan uses the incremental lifecycle path rather than the direct one-transaction path a one-operation ADR normally takes, because the new claim's proof markers must exist alongside the test that carries them for `awf check` to stay clean inside the implementation phase; the deferred status-only `Implemented` flip stays with `effort-workflow` after terminal review settles.

Before dispatch, the parent establishes a clean and green managed-worktree baseline: `git status --short` prints no output, `./x check` reports clean, and `./x gate` passes. The phase owner receives that verified baseline and works only in `/home/hypno/Projects/agentic-workflows/.awf/worktrees/review-remediation-autonomy` until returning its completed report.

## Phase 1: Ship authority-guided review remediation

**Execution mode: subagent-driven.**

Completes: ["escalation-boundary", "residual-routing", "resync-edge", "preserved-controls", "authority-applied"]

### Task 1.1: Pin the review-remediation contract before production changes
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]

In `internal/project/spine_test.go`, add `TestAuthorityGuidedReviewRemediation` carrying the proof marker `// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestAuthorityGuidedReviewRemediation)` in its doc comment block. Write it to fail before Tasks 1.2 and 1.3 land.

Read `partials/review-remediation-autonomy.md` from `templates.FS` and assert it contains no `{{`, no line whose trimmed form starts with `#`, and no `awf:section` or `awf:include` substring, mirroring the structural assertions `TestAuthorityGuidedImplementationAutonomy` makes on the implementation-autonomy partial.

For each of `skills/reviewing-plan/SKILL.md.tmpl`, `skills/reviewing-adr/SKILL.md.tmpl`, `skills/reviewing-plan-resync/SKILL.md.tmpl`, and `skills/reviewing-impl/SKILL.md.tmpl`, read the embedded source and assert `strings.Count(raw, "<!-- awf:include review-remediation-autonomy -->")` equals 1.

Render all four skills through `renderSkillGolden` in two variants: a configured variant with `prefix` `example`, `vars` carrying `gateCmd` `./x gate`, `layout` from `testLayout()`, `commitScopes` `` `docs(plans)` ``, `skills` enabling `effort-workflow`, `adr-lifecycle`, and `reviewing-impl`, and `targetSubagentTools` true; and an empty variant with `prefix` `example`, empty `vars`, `testLayout()`, empty `data`, and empty `skills`. Call `assertNoLeaks` on every output. Assert each output carries the dispatcher semantics: mechanical and reasoned corrections are applied autonomously; the review spine defines the classification and the dispatcher routes rather than redefines it; ambiguity, competing clean options, severity, structural character, and survival of a prior correction do not transfer the choice to the user; a newly material load-bearing choice routes through the existing grounded-design or ADR workflow and pauses only at that workflow's mandatory approval boundary; exactly one fresh verify-pass dispatch is retained; every residual finding is diagnosed under the same boundary and corrected without another same-artifact review loop; and a stop requires that every viable correct remediation would contradict a settled user-approved design or decision or require an unauthorized change to an active current-state claim, with that authority cited. Assert a consensus deviation remains a user decision.

Render `adr-reviewer`, `plan-reviewer`, and `code-reviewer` through `renderAgentGolden` with the same data shapes those agents already use in `TestAdrReviewerAgent`, `TestPlanReviewerAgent`, and `TestCodeReviewerAgent`. Assert each carries the revised `user-decision` criterion and the explicit non-triggers sentence, and that `mechanical`, `reasoned`, `user-decision`, and `suggested_fix` all survive.

Assert these retired phrases are absent from every rendered skill and agent output above and from each of the four embedded skill sources: ``Escalate any residual structural findings as `user-decision` items``, `the step-2 return edge applies to initial-dispatch findings only`, `do not loop further without explicit user direction`, and `a genuine design fork or unresolved ambiguity that should not be decided unilaterally`.

Add the same proof marker line, naming its own unit, to the doc comment of `TestConditionalVerifyPass`: `// invariant: rendering/workflow-skill-templates:authority-guided-review-remediation (TestConditionalVerifyPass)`. That test owns the retained single-verify-pass bound; do not extend it to clauses it does not exercise.

Run `go test ./internal/project -run 'TestAuthorityGuidedReviewRemediation|TestConditionalVerifyPass|TestAdrReviewerAgent|TestPlanReviewerAgent|TestCodeReviewerAgent|TestReviewingPlanTemplate|TestReviewingAdrTemplate|TestReviewingPlanResyncTemplate'`; before Tasks 1.2 and 1.3 it fails on the missing partial, missing includes, and unchanged classification prose, and after them it passes.

### Task 1.2: Move the classification boundary into the shared spine
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:preserve-review-controls"]

In `templates/partials/review-spine-head.md`, under `## Classification rules`, keep the `mechanical` and `reasoned` bullets byte-for-byte and replace the `user-decision` bullet with:

```
- **user-decision**: every viable correct remediation would contradict or change a settled user-approved design or decision, or would require an unauthorized change to an active current-state claim; cite the affected authority and name the deviation it would require.
```

Keep the existing `Severity is informational only; the dispatching skill routes by classification kind.` line unchanged and add one paragraph directly after it:

```
Ambiguity, competing clean options, severity, structural character, and the fact that a finding survived a prior correction do not by themselves make a finding a user decision. When no settled authority is affected, the finding is mechanical or reasoned.
```

Leave the finding schema, the specific-location requirement, and the whole `## Consensus adherence` section unchanged; a consensus deviation stays a `user-decision` finding under its own rule. Add no directive to apply fixes, commit, or re-review, so the `reviewers-report-only` claim stays true.

Create `templates/partials/review-remediation-autonomy.md` as the only full statement of the dispatcher-side obligation. Keep it target-neutral, variable-free, free of headings and section markers, free of nested includes, and publication-safe. Open with a bold non-heading label in the style of `templates/partials/implementation-autonomy.md` so direct inclusion never interrupts a consumer's section structure or numbered procedure. It must state that the dispatching workflow applies mechanical corrections directly and reasoned corrections with a concise rationale, autonomously; that the review spine is the single semantic home of the classification and this workflow routes it rather than redefining it; that ambiguity, competing clean options, severity, structural character, and survival of a prior correction never transfer the choice to the user; that a finding making a new load-bearing choice material outside approved durable boundaries routes through the existing grounded-design or ADR workflow and pauses only at that workflow's mandatory approval boundary before the new authority is adopted; that exactly one fresh verify-pass dispatch is retained after reasoned fixes or a user-approved ruling, every residual finding is diagnosed under the same boundary, authority-preserving mechanical and reasoned residual corrections are applied, the applicable verification runs, and its disposition is reported without dispatching another same-artifact review loop; and that the workflow stops and presents a user decision only when every viable correct remediation would contradict or change a settled user-approved design or decision or would require an unauthorized change to an active current-state claim, citing that authority, with a consensus deviation remaining a user decision. Do not restate the mechanical and reasoned definitions the spine owns, and do not introduce a severity rule, classifier, or policy schema.

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

In the `re-review-loop` section of `templates/skills/reviewing-plan/SKILL.md.tmpl` and `templates/skills/reviewing-adr/SKILL.md.tmpl`, replace the trailing sentence ``Escalate any residual structural findings as `user-decision` items; do not loop further without explicit user direction.`` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above, apply the authority-preserving mechanical and reasoned residual corrections, and report their disposition. Stop only for a residual finding that remains a true user decision, and dispatch no further same-artifact review pass.
```

Leave the preceding conditional-dispatch prose in both sections untouched so the `TestConditionalVerifyPass` ordered phrases stay green.

In `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, make the governed return edge cover residual findings. In the `classify-route-findings` section, change the return-edge opening from `when a finding implicates the ADR itself` to `when a finding, initial or residual, implicates the ADR itself` and keep the rest of that paragraph, including the amend, re-review, re-resync loop, unchanged. In the `re-review-loop` section, replace the trailing sentence ``Escalate any residual structural findings as `user-decision` items, an ADR-implicating residual included: the step-2 return edge applies to initial-dispatch findings only. Do not loop further without explicit user direction.`` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above and apply the authority-preserving mechanical and reasoned residual corrections. An ADR-implicating residual takes the step-2 return edge while the implicated ADR remains amendable: this resync ends, the ADR is amended and independently reviewed, and a new resync invocation follows under its own one-verify-pass bound. Stop only for a residual finding that remains a true user decision, and dispatch no further same-artifact verify pass inside this invocation.
```

In the `notes` section of the same file, update the first note so the return edge is no longer described as step-2-only: state that ADR-implicating findings route through the ADR amendment and review skills whether they surface on initial dispatch or in the verify pass.

In the `re-review-loop` section of `templates/skills/reviewing-impl/SKILL.md.tmpl`, replace the trailing sentence `Classify any residual finding, apply authority-determined residual fixes, rerun the gate and audit, and stop on any unresolved user decision. Do not add another review loop.` with:

```
Diagnose every residual finding under the authority-guided remediation boundary above, apply the authority-preserving residual fixes, rerun the gate and audit, and stop only for a residual finding that remains a true user decision. Do not add another review loop.
```

Leave the report-only reviewer instructions, the consensus-adherence paste, the model-tier prose, the audit step, and every hand-off route unchanged in all four skills. Change no mandatory approval: `templates/skills/reviewing-adr/SKILL.md.tmpl` step 8 keeps the settled-ADR approval stop verbatim, and no skill gains or loses a checkpoint.

Run `go test ./internal/project -run 'TestAuthorityGuidedReviewRemediation|TestConditionalVerifyPass|TestAdrReviewerAgent|TestPlanReviewerAgent|TestCodeReviewerAgent|TestReviewingPlanTemplate|TestReviewingAdrTemplate|TestReviewingPlanResyncTemplate|TestReviewersReportOnly'`; it passes.

### Task 1.4: Apply current authority, lifecycle events, render outputs, and close green
Kind: batch
Latitude: exact
Applying: ["authority-guided-review-remediation:authority-deviation-boundary", "authority-guided-review-remediation:autonomous-review-judgment", "authority-guided-review-remediation:uniform-residual-routing", "authority-guided-review-remediation:resync-return-edge", "authority-guided-review-remediation:preserve-review-controls"]
Paths: ["templates/partials/review-remediation-autonomy.md", "templates/partials/review-spine-head.md", "templates/skills/reviewing-plan/SKILL.md.tmpl", "templates/skills/reviewing-adr/SKILL.md.tmpl", "templates/skills/reviewing-plan-resync/SKILL.md.tmpl", "templates/skills/reviewing-impl/SKILL.md.tmpl", "internal/project/spine_test.go", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "changelog/CHANGELOG.md", "docs/decisions/authority-guided-review-remediation.md", ".awf/awf.lock", "docs/topics/rendering/workflow-skill-templates.md", "glob:.claude/skills/awf-reviewing-*/SKILL.md", "glob:.pi/skills/awf-reviewing-*/SKILL.md", "glob:.claude/agents/*.md", "glob:.pi/agents/*.md"]
Representative: `templates/partials/review-remediation-autonomy.md` expands once into each of the eight rendered reviewing-skill outputs under `.claude/skills/awf-reviewing-*/SKILL.md` and `.pi/skills/awf-reviewing-*/SKILL.md`, while the revised `review-spine-head.md` classification bullet reaches `adr-reviewer.md`, `plan-reviewer.md`, and `code-reviewer.md` under both `.claude/agents/` and `.pi/agents/`.
Edge: The empty-variant render of every affected skill and agent carries the same boundary with no unresolved no-value token; `.claude/agents/explorer.md`, `.pi/agents/explorer.md`, `.claude/agents/grounding-checker.md`, `.pi/agents/grounding-checker.md`, and `.claude/agents/implementer.md`, `.pi/agents/implementer.md` fall inside the declared agent globs but must show no diff, since neither the spine partial nor the new partial reaches them.
Post-check: After `./x render`, run `git diff --check` and require no output. Then run `rg -n -e 'Escalate any residual structural findings' -e 'the step-2 return edge applies to initial-dispatch findings only' -e 'do not loop further without explicit user direction' -e 'a genuine design fork or unresolved ambiguity that should not be decided unilaterally' templates/partials/review-spine-head.md templates/skills/reviewing-plan/SKILL.md.tmpl templates/skills/reviewing-adr/SKILL.md.tmpl templates/skills/reviewing-plan-resync/SKILL.md.tmpl templates/skills/reviewing-impl/SKILL.md.tmpl .claude/skills/awf-reviewing-*/SKILL.md .pi/skills/awf-reviewing-*/SKILL.md .claude/agents/*.md .pi/agents/*.md` and require exit status 1 (no matches) rather than 2 (traversal or path error); establish that the probe actually reached those files by rerunning the identical command with `-e 'user-decision'` substituted for the four patterns and confirming it returns matches and exit status 0. Finally run `git diff --name-only` and confirm every changed path resolves inside the declared `Paths` population and every changed generated output is attributable to a named source or current-state mutation.

In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, add one claim placed directly after the existing `authority-guided-implementation-autonomy` claim so the two autonomy boundaries read together. Use the heading line, trailer lines, and blank-line spacing already used by the sibling claims in that file, with the claim id `authority-guided-review-remediation`, `Origin: ADR-authority-guided-review-remediation`, `Backing: test`, and no `Revised-by` line. Its body must assert, in present tense: the shared review spine is the single semantic classification home and reserves `user-decision` for a finding where every viable correct remediation would contradict a settled user-approved design or decision or require an unauthorized active-claim change, with the affected authority cited; ambiguity, competing clean options, severity, structural character, and survival of a prior correction do not transfer the choice; one variable-free shared prose partial is directly included exactly once by plan review, ADR review, plan resync, and implementation review and applies mechanical and reasoned corrections autonomously; a newly material load-bearing choice routes through the existing grounded-design or ADR workflow and pauses at that workflow's mandatory approval boundary; exactly one fresh verify pass is retained and every residual finding is diagnosed under the same boundary without another same-artifact review loop; plan resync's ADR amendment and review return edge covers initial and residual findings while the implicated ADR remains amendable; and reviewers stay report-only, consensus deviations stay user decisions, and required gates, verification, and the mandatory grounded-design and settled-ADR approvals are unchanged, with every affected template rendering coherently under empty variables. Change no other claim: the ADR declares one `add` operation and no `update`.

Update `changelog/CHANGELOG.md` under `## [Unreleased]` / `### Features` with one concise adopter-facing entry stating that plan, ADR, resync, and implementation review now apply authority-preserving corrections autonomously, including after the single verify pass, and ask the user only when a correction would deviate from settled design or an active current-state claim.

Transition `docs/decisions/authority-guided-review-remediation.md` from Proposed to Implementing in the same transaction as the claim addition. Append an Implementing status event dated on execution day with the current content digest, then append one Applied event whose operation list is exactly the single add operation the ADR's State changes section declares, written in the same backticked-claim-id form that section uses. Obtain the digest through the governed placeholder workflow: insert 64 zeros, run `./x check`, copy the reported computed digest exactly, replace the placeholder, and rerun until clean; do not precompute or guess it. Leave the ADR at `Implementing` and this plan at `status: Proposed`; `effort-workflow` owns both later status-only `Implemented` flips after terminal implementation review settles.

Run `./x render`. Then perform the focused semantic rendering review at each affected output boundary. Read `.claude/skills/awf-reviewing-plan/SKILL.md`, `.pi/skills/awf-reviewing-plan/SKILL.md`, `.claude/skills/awf-reviewing-plan-resync/SKILL.md`, `.claude/skills/awf-reviewing-adr/SKILL.md`, `.claude/skills/awf-reviewing-impl/SKILL.md`, `.claude/agents/plan-reviewer.md`, and `.pi/agents/adr-reviewer.md` and confirm each reads as one coherent instruction rather than two: the expanded autonomy paragraph must not sit adjacent to a surviving fragment that still promotes residual findings automatically, the numbered procedure must remain correctly ordered around the inserted include, and the resync return edge must read consistently in both its step-2 and verify-pass mentions. Confirm the reviewing-impl output carries both expanded partials without duplicated or contradictory escalation policy. Confirm literals such as `<literal-placeholder>`, `<slug>`, `<name>`, and `<path> (missing)` remain intentional generic placeholders rather than unresolved template values, and that no output contains an unresolved no-value token.

Run `go test ./internal/project ./internal/evals`, `./x check`, and `./x gate`; each reaches a clean or passing terminal state.

### Phase close

Return every implementation deviation in the completed child report and do not edit this plan. Leave plan frontmatter `status: Proposed` and the ADR `status: Implementing`; the parent supplies the completed report to report-only phase review and owns any focused Notes-and-findings settlement commit before checkpointing. Stage the complete implementation transaction explicitly, including the new partial, the revised spine partial, the four reviewing-skill templates, the test file, the current-state part, the ADR lifecycle events, the changelog, the lock, and every rendered output, but excluding this parent-owned plan. Run `./awf check staged` and `./x gate`; both pass. Create the one phase-closing commit:

```commit
feat(rendering): add authority-guided review remediation
```

## Definition of done

- `dod: escalation-boundary` The shared review spine defines `user-decision` as a cited deviation from settled user-approved design or an active current-state claim, and explicitly denies that ambiguity, competing clean options, severity, structural character, or survival of a prior correction transfers the choice; all three reviewer agents render that definition.
- `dod: residual-routing` Plan, ADR, resync, and implementation review each include the shared remediation partial exactly once, retain exactly one fresh verify pass, diagnose residual findings under the same boundary, and dispatch no second same-artifact review pass; the automatic residual-escalation sentences are gone from every source and rendered output.
- `dod: resync-edge` Plan resync's ADR amendment and review return edge is available to initial and residual findings while the implicated ADR remains amendable, and its notes describe it that way.
- `dod: preserved-controls` Reviewers stay report-only, consensus deviations stay user decisions, required gates and verification are unchanged, the grounded-design and settled-ADR approvals stand verbatim, and every affected skill and agent renders coherently under empty variables with no unresolved token.
- `dod: authority-applied` The new current-state claim is added with test backing and its declared operation Applied, the ADR is `Implementing` and this plan `Proposed` pending deferred terminal closure, and `./x render`, `./x check`, `go test ./internal/project ./internal/evals`, and `./x gate` all reach a clean or passing terminal state.

## Notes

Record inline deviations immediately. For this delegated phase, preserve the child report as phase-review input and reconcile reported deviations plus review findings in one focused parent settlement commit before checkpointing or later execution. The settled design prohibits a policy schema, automated classifier, severity-based routing, an additional verify pass, a new workflow stage, a weakened gate or check, and any change to the report-only reviewer boundary or the mandatory grounded-design and settled-ADR approvals.
