---
format: current-state-v2
status: Proposed
date: 2026-07-29
---
# ADR-0177: Rendered implementer role contract

## Context

Dispatched implementation subagents return a no-op "blocked" with no reason on plan phases that end
in the green gate. The parent then has nothing to act on: no inventory of what was done, no failing
check, no output, no statement of what was already tried. The root cause is a missing instruction,
not a bad one.

The implementation role is the only dispatched role in awf with no child-facing contract. The four
dispatched roles divide cleanly:

- The two reviewer roles load a rendered agent file from disk. `loadReviewer` in
  `.pi/extensions/awf-subagents/index.ts:235` reads `REVIEWER_PATHS[kind]` (index.ts:93), strips
  frontmatter, prepends a per-role report-only line, and errors on an empty body. Their behaviour is
  authored in `templates/agents/*.md.tmpl` and governed by `rendering/workflow-skill-templates:reviewers-report-only`.
- The explore and grounding roles carry literal persona prose in `rolePrompt()`
  (`templates/pi/awf-subagents/index.ts.tmpl:172-197`), several paragraphs each.
- The implement role gets one sentence (`templates/pi/awf-subagents/index.ts.tmpl:198`, rendered at
  `.pi/extensions/awf-subagents/index.ts:232`): follow AGENTS.md and the task exactly, commits are
  allowed or forbidden, "Report changed files, verification, and blockers." That sentence invites
  "blockers" and specifies nothing about what a blocker report must contain.

Three further facts shape the fix.

Children are spawned `-p --no-session` (`templates/pi/awf-subagents/runner.ts.tmpl:188-197`), so a
child has no interactive channel: the absence is mechanical, not a matter of degree. Parent-facing
prose nonetheless reaches the child unqualified. `templates/partials/checkpoint-routine.md` says to
"stop and wait", and subject-less imperatives such as subagent-driven-development step 2 ("Stop for
missing phase context, paths, boundaries, checks, or closing subject.") read as addressed to
whoever is running the skill. A child that obeys them stops and waits for a user who cannot answer.

ADR-0166 made a plan phase one all-or-nothing green transaction, and ADR-0173 applied
smallest-reliable-model pressure to every governed dispatch. Both are correct alone. Together they
concentrate risk on the heaviest role: the one dispatch that must reach green in a single shot is
also the one under pressure to run on the smallest model, and it is the only one without a contract.

One premise from the original diagnosis was wrong and is recorded here so it is not reintroduced:
the implement child *can* read its own skill file. `IMPLEMENT_TOOLS` includes read and bash, and Pi
renders skills to `.pi/skills/<prefix>-<name>/SKILL.md`. The defect is not that the child is denied
its instructions; it is that nothing tells it which instructions are its own, and AGENTS.md routes
it into parent-facing chain procedures it must not run.

Scope is split. This ADR takes the implementation role only (Part A). A successor takes the explorer
and grounding-checker roles, deletes the remaining `rolePrompt` branches, and extracts a shared
dispatch spine (Part B). The split keeps this fix off two blockers that Part B genuinely owns:
revising `rendering/workflow-skill-templates:bounded-exploration-reporting`, whose text explicitly
names "Pi's fixed prompt" and whose literal strings are pinned by `TestBoundedExplorationReporting`
(`internal/project/target_test.go:454`), and settling whether non-Pi dispatch prose may name those
agents at all, which `rendering/workflow-skill-templates:cross-runtime-exploration-dispatch`
deliberately constrains.

That constraint is exploration-specific and does not reach the implementation role: no claim
protects the genericity of the subagent-driven-development dispatch branch, and the non-Pi branch of
`reviewing-impl` already names the `code-reviewer` agent literally
(`templates/skills/reviewing-impl/SKILL.md.tmpl:34`). Naming `implementer` in generic prose follows
established precedent.

## Decision

1. Add one `implementer` agent to the standard catalog: an `AgentSpec` in
   `internal/catalog/standard.go` plus `templates/agents/implementer.md.tmpl`. It is one agent with
   two authority modes, a commit-capable phase owner and a commit-disabled path-confined helper,
   selected by the dispatch, not two agents. Its `RequiresSkills` stays empty: the contract
   deliberately does not route the child into any skill.

2. The rendered contract body carries exactly seven clauses:

   1. **Identity and authority mode.** The child states which mode it is operating in: commit-capable
      phase owner, or commit-disabled helper confined to the paths it was given.
   2. **The task is the complete scope.** The brief is the whole job. The child does not replan,
      broaden scope, or perform unrelated cleanup.
   3. **Selective AGENTS.md authority.** The invariants, conventions, and commands in AGENTS.md bind
      the child. Its skill catalog and chain routing do not: the child runs no workflow skill, opens
      no effort, and writes no memory.
   4. **The green obligation.** Reaching green is the job. Iterating on failures is expected work,
      not a blocker. The child never weakens an assertion, relaxes a golden, or edits a test in
      order to make a failure disappear; updating a golden that the phase legitimately changes is
      expected work and is reported as such.
   5. **No user exists.** The child has no interactive channel. Escalation means returning an
      inventory and finishing the turn, never stopping to wait for an answer.
   6. **The owner transaction procedure.** A commit-capable owner stages the complete transaction,
      runs `awf check --staged`, runs the gate, and commits once, only after both pass, leaving a
      clean tree.
   7. **The structured return.** A closed two-outcome return schema, so the parent distinguishes
      outcomes without parsing free text. `completed` requires the closing commit subject and sha in
      owner mode or the changed-file list in helper mode, plus each verification actually run and its
      result. `stopped` is invalid without all five of: `git status --short` output, work completed,
      work remaining, the failing check with its actual output, and what was already tried. There is
      no third outcome; an aborted or crashed child is the runtime's failure to report, not the
      child's.

3. Pi loads the implementer contract from the rendered artifact rather than from a literal string.
   `subagent_implement` reads `.pi/agents/implementer.md` through a loader that preserves
   `loadReviewer`'s three behaviours: frontmatter strip, per-role prepend, and a hard error on an
   empty instruction body (`templates/pi/awf-subagents/index.ts.tmpl:208-210`). Its missing-file error
   carries the same actionable repair shape as the reviewer loader's ("Enable the matching ... agent
   and run awf render"). Only the implement branch of `rolePrompt` is deleted; the explore and
   grounding branches stay, so neither exploration claim is revised by this ADR.

4. A commit-capable owner that produces no commit fails the call. The existing before/after git
   snapshot around the implement run (`templates/pi/awf-subagents/index.ts.tmpl:741-752`) currently
   fails only the mirror case, `allowCommits: false` with a changed HEAD; it gains the case
   `allowCommits: true` with HEAD unchanged, whose failure message demands the clause-7 `stopped`
   inventory. Be precise about what this buys: the check is a no-commit detector, so it makes the
   *existence* of a silent non-completion mechanical, and it deliberately also fails a child that
   stopped correctly with a full inventory, because that phase did not complete and a failure result
   is the honest report. It cannot inspect the five clause-7 fields, which stay prose-enforced. That
   is still the difference between a parent receiving an unexplained "blocked" as success and
   receiving a failed call, which prose alone never achieved.

   `rendering/pi-workflows:pi-implement-role-artifact` owns both the disk load in item 3 and this
   check. `rendering/pi-workflows:pi-subagent-failure-details` needs no revision: the new result is
   itself a marked error result carrying bounded progress and diagnostics through the same
   `tool_result` middleware, and the implementation-commit-policy behaviour that claim names is
   retained unchanged.

5. Rendered skill prose is corrected in the same change. `subagent-driven-development` and
   `executing-plans` state the brief contract and name the `implementer` agent in both the Pi and
   non-Pi dispatch branches. Within those two skill bodies and any partial they include, every
   subject-less imperative that a child could read as addressed to itself gains an explicit subject,
   including subagent-driven-development step 2; the obligation is bounded to those two artifacts and
   does not reach the wider skill corpus. The `checkpoint-routine` partial stays parent-only and is
   not included in any child-facing artifact.

   `implementer-role-contract` covers these prose corrections.
   `rendering/workflow-skill-templates:maintainable-code-subagent-contract` and
   `rendering/workflow-skill-templates:phase-transaction-ownership` need no revision even though
   clauses 2 and 6 restate obligations they already pin: both claims constrain what the dispatching
   skills must say, and neither asserts that only a skill may carry the obligation, so a second
   artifact repeating it leaves both true.

6. Catalog wiring lands in one commit with its configuration. `subagent-driven-development` and
   `executing-plans` each gain `RequiresAgent: "implementer"`, declared on both because both dispatch
   implementation children: the phase owner and the commit-disabled batch helpers
   (`templates/skills/executing-plans/SKILL.md.tmpl:31`). Relying on the transitive edge through
   `executing-plans`'s `RequiresSkills` would leave the declared structural edge not matching the
   declared dispatch. Both `.awf/config.yaml` and `examples/sundial/.awf/config.yaml` enable the agent
   in that same commit. Splitting them would leave
   `rendering/catalog-and-targets:enabled-set-closed` unmet, failing every gated command at project
   open, including the gate meant to prove the change. This is the first non-reviewer value of
   `RequiresAgent`, so the field's doc comment at `internal/catalog/catalog.go:57-58` and the
   reviewer-only framing around it are corrected in the same commit. No
   `rendering/catalog-and-targets` operation is needed: `catalog-go-single-source`,
   `skill-section-parity`, `structured-agent-encoding`, `target-dialect-render`, `enabled-set-closed`,
   and `requires-skills-exact` are all generic over agents, and item 1's empty `RequiresSkills`
   conforms to the last of them.

7. Adopter trees migrate through one registry entry, `{To: 23, Name: "implementer-agent-closure",
   Apply: applyCloseEnabledSet}`, reusing the existing enabled-set closure (precedent: `{To: 13,
   "exploring-skill-closure"}`), sequenced after ADR-0175's generation 22.

8. Three surfaces describe awf's catalog agents as review agents and are falsified by item 1. They
   are corrected in the same commit: `README.md:45-48` (the "**Review agents**" bullet listing exactly
   the three reviewers), `README.md:245-246` ("the three review agents" in the `awf init` default
   description, which becomes four), and `.awf/parts/agents-doc/identity.md:3` ("multi-runtime skills,
   review agents, docs"), whose correction reaches `AGENTS.md` only through `./x render`. Every status
   transition on this ADR runs `./x render` and commits the regenerated `docs/decisions/INDEX.md` and
   lock alongside the status change.

9. No shared dispatch spine is extracted here. One new agent does not justify the abstraction; Part B
   extracts it when the second and third arrive.

## State changes

- add `rendering/workflow-skill-templates:implementer-role-contract`
- add `rendering/pi-workflows:pi-implement-role-artifact`

## Consequences

The implementation role gains what the reviewer roles have had since ADR-0148: a rendered,
drift-checked, review-visible contract that can be changed by editing a template instead of a
TypeScript string literal. A reasonless "blocked" stops being a silent parent-side puzzle and becomes
a failed call with a named cause.

The commit-disabled helper mode is prose-only outside Pi. Agent encoders emit a literal name plus a
rendered description and the rendered instruction body, and the Codex TOML profile is a closed
three-key schema (`internal/project/agent.go:22-26`,
`rendering/catalog-and-targets:structured-agent-encoding`), so the seven-clause body does reach every
target but no non-Pi target can express an authority mode structurally. Off Pi the mode is carried by
the dispatch brief and honoured by the child; only the Pi snapshot check enforces it. This is accepted
rather than mitigated: forcing the distinction into every encoder would be a much larger change for a
guarantee only one runtime can currently keep.

`subagent_implement` starts failing closed. Adopting the reviewer loader's empty-body and missing-file
errors means a Pi adopter who enables the pi target without `subagent-driven-development` or
`executing-plans` has no rendered implementer agent and loses a tool that works today off the literal
string; generation 23 closes the enabled set only for adopters who already enable one of those skills.
The precedent is exact and accepted: `subagent_review` already fails identically, and item 3 commits
the implementer loader to the same actionable repair hint
(`.pi/extensions/awf-subagents/index.ts:240`).

Adding an `AgentSpec` triggers six machine-forced authoring obligations, each of which fails the
gate if skipped: a golden test in `internal/project/spine_test.go` (required by
`TestEveryCatalogArtifactHasGoldenTest` in `internal/project/catalog_sweep_test.go:139-152`),
`dataKeys` entries in `internal/configspec` for every data key
(`internal/configspec/spec_test.go:207`), `awf:section` marker parity between template and spec, a
hand-authored unset-data fallback for every conditional, a leak-free empty-data render, and fixups to
the two existing test scaffolds that enable a now-paired skill with `agents: []` and would therefore
fail `enabled-set-closed` at `Open()`: `internal/project/project_test.go:1415` and
`internal/project/target_test.go:659`, the latter being `TestMaintainableCodeMultiTargetParity`, itself
a backing test for `maintainable-code-subagent-contract`. The tool-agnostic scan
(`rendering/workflow-skill-templates:skill-prose-tool-agnostic`) additionally bans backticked
`write`, `edit`, and `read` and phrases such as "read tool", so the contract body must be authored
around it. These are known costs of the chosen home, not surprises.

Part B is unblocked but not started, and its two revisions remain owed:
`bounded-exploration-reporting` and `cross-runtime-exploration-dispatch`. Until it lands, awf carries
one role loaded from a rendered artifact and two still carrying literal prose, an asymmetry that is
deliberate and temporary.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Author the contract in the dispatching skill instead of a rendered agent | The contract would be duplicated across `subagent-driven-development` and `executing-plans`, and it would live in a parent-facing artifact the child is told not to run. |
| Expand the implement branch of `rolePrompt` in place | Keeps the heaviest contract in a TypeScript string literal: invisible to review of rendered output, unreachable by drift checking, and Pi-only by construction. |
| Two agents, one per authority mode | The two modes share every clause but one. Two agents would double the five machine-forced obligations to express a single boolean. |
| Extract the shared dispatch spine now | One new agent does not justify the abstraction. Part B extracts it against three call sites instead of guessing at one. |
| Fix the reasonless stop with prose alone | Prose is what already failed, and the child is under model pressure and reads a lot of text. Clause 7's five fields do remain prose-enforced, but item 4 makes the one thing that matters most, a silent non-completion reported as success, mechanical instead. |

## Status history

- 2026-07-29: Proposed
