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
- The implement role gets one sentence (`templates/pi/awf-subagents/index.ts.tmpl:199`, rendered at
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
(`internal/project/target_test.go:435`), and settling whether non-Pi dispatch prose may name those
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
      not a blocker. The child never weakens an assertion, relaxes a golden, or edits a test to make
      a failure disappear.
   5. **No user exists.** The child has no interactive channel. Escalation means returning an
      inventory and finishing the turn, never stopping to wait for an answer.
   6. **The owner transaction procedure.** A commit-capable owner stages the complete transaction,
      runs `awf check --staged`, runs the gate, and commits once, only after both pass, leaving a
      clean tree.
   7. **The structured return.** A closed return schema whose `stopped` outcome is invalid without
      all five of: `git status --short` output, work completed, work remaining, the failing check
      with its actual output, and what was already tried.

3. Pi loads the implementer contract from the rendered artifact rather than from a literal string.
   `subagent_implement` reads `.pi/agents/implementer.md` through a loader that preserves
   `loadReviewer`'s three behaviours: frontmatter strip, per-role prepend, and a hard error on an
   empty instruction body (`templates/pi/awf-subagents/index.ts.tmpl:243-244`). Only the implement
   branch of `rolePrompt` is deleted; the explore and grounding branches stay, so no backed claim is
   revised by this ADR.

4. The stop-report rule is enforced mechanically, not by prose. The existing before/after git
   snapshot around the implement run (`templates/pi/awf-subagents/index.ts.tmpl:741-752`) is extended
   so that `allowCommits: true` with HEAD unchanged and no reported commit produces a failure result
   naming the missing inventory. Prose alone cannot stop a reasonless "blocked"; a check that fails
   the call can.

5. Rendered skill prose is corrected in the same change. `subagent-driven-development` and
   `executing-plans` state the brief contract and name the `implementer` agent in both the Pi and
   non-Pi dispatch branches. Every subject-less imperative that a child could read as addressed to
   itself gains an explicit subject, including subagent-driven-development step 2. The
   `checkpoint-routine` partial stays parent-only and is not included in any child-facing artifact.

6. Catalog wiring lands in one commit with its configuration. `subagent-driven-development` gains
   `RequiresAgent: "implementer"`, and both `.awf/config.yaml` and
   `examples/sundial/.awf/config.yaml` enable the agent in that same commit. Splitting them would
   leave `rendering/catalog-and-targets:enabled-set-closed` unmet, failing every gated command at
   project open, including the gate meant to prove the change.

7. Adopter trees migrate through one registry entry, `{To: 23, Name: "implementer-agent-closure",
   Apply: applyCloseEnabledSet}`, reusing the existing enabled-set closure (precedent: `{To: 13,
   "exploring-skill-closure"}`), sequenced after ADR-0175's generation 22.

8. No shared dispatch spine is extracted here. One new agent does not justify the abstraction; Part B
   extracts it when the second and third arrive.

## State changes

- add `rendering/workflow-skill-templates:implementer-role-contract`
- add `rendering/pi-workflows:pi-implement-role-artifact`

## Consequences

The implementation role gains what the reviewer roles have had since ADR-0148: a rendered,
drift-checked, review-visible contract that can be changed by editing a template instead of a
TypeScript string literal. A reasonless "blocked" stops being a silent parent-side puzzle and becomes
a failed call with a named cause.

The commit-disabled helper mode is prose-only outside Pi. Agent encoders emit name and description
only, and the Codex TOML profile is a closed three-key schema
(`rendering/catalog-and-targets:structured-agent-encoding`), so no non-Pi target can express an
authority mode structurally. Off Pi the mode is carried by the dispatch brief and honoured by the
child; only the Pi snapshot check enforces it. This is accepted rather than mitigated: forcing the
distinction into every encoder would be a much larger change for a guarantee only one runtime can
currently keep.

Adding an `AgentSpec` triggers five machine-forced authoring obligations, each of which fails the
gate if skipped: a golden test in `internal/project/spine_test.go` (required by
`TestCatalogSweep` in `internal/project/catalog_sweep_test.go:137-152`), `dataKeys` entries in
`internal/configspec` for every data key (`internal/configspec/spec_test.go:207`), `awf:section`
marker parity between template and spec, a hand-authored unset-data fallback for every conditional,
and a leak-free empty-data render. The tool-agnostic scan
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
| Fix the reasonless stop with prose alone | Prose is what already failed. The child is under model pressure and reads a lot of text; a check that fails the call is the only enforcement that cannot be skimmed past. |

## Status history

- 2026-07-29: Proposed
