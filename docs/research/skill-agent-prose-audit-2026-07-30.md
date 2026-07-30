# Skill and subagent prose audit - 2026-07-30

Full audit of the skill and subagent prose surface: the adopter-facing template
sources under `templates/skills/` (19 files), `templates/agents/` (7),
`templates/partials/` (6), the project-local overlays under `.awf/skills/parts/`
and the agent/skill sidecar yamls, and the rendered outputs under
`.claude/skills/` and `.claude/agents/`. Goal per the commissioning request:
verify each artifact is well written, as terse as possible without losing
needed instruction, and correctly steers what it intends.

Method: five independent report-only audit subagents (chain-entry, review
cluster, execution cluster, task/support, cross-cutting), shared rubric
(concision, steering accuracy against current commands/paths/contracts,
clarity of actor and imperative, frontmatter/trigger quality, punctuation and
publication-safety invariants). Every high-severity finding below was
independently re-verified in the main thread against the sources; catalog and
vocabulary claims were confirmed against `internal/catalog/standard.go` and the
skill bodies. `./x check` is clean for the repo and the sundial example; no
unresolved tokens or banned codepoints anywhere on the surface.

Overall verdict: the surface is healthy - no stale `.awf/memory/` references,
the routing triple (mechanical / reasoned / user-decision) is identical
everywhere, contracts are mostly symmetric, and `./x check` passes. The two
systematic problems are (1) a class of hard-coded enumerations and target
literals that have drifted from reality and now steer incorrectly, and (2)
roughly 150+ template lines of restatement: the same contract prose rendered
two or three times per file, or duplicated across sibling files that a partial
should own.

## A. Steering defects (fix first; each verified)

A1. **Broken adopter command.** `templates/skills/writing-plans/SKILL.md.tmpl:56`
interpolates `{{ .prefix }} new plan` - `.prefix` is the skill-name prefix, not
the binary. The sundial example renders `sundial new plan`, a nonexistent
command; only the prefix=awf coincidence makes this repo's render correct. Fix
with the `runnerEnabled` conditional already used at
`templates/skills/proposing-adr/SKILL.md.tmpl:63`. Same line also sanctions
copying `docs/plans/template.md`, which carries the generated-file banner; drop
the copy path (proposing-adr already bans the equivalent for ADRs).

A2. **Stale lens enumerations steer reviewers to skip lenses.** The rendered
plan-reviewer has seven universal lenses; `reviewing-plan/SKILL.md.tmpl:16,38`
says "all five lenses" and `plan-reviewer.md.tmpl:48` plus
`reviewing-plan-resync/SKILL.md.tmpl:29,65` say "the other three lenses",
omitting `application-batches` and `maintainable-design`. The catalog
description of code-reviewer (`internal/catalog/standard.go:161`) enumerates
five of its seven lenses. Because the brief is what a dispatched reviewer
obeys, these enumerations license skipping two lenses. Fix: replace every
count/list with "all universal lenses" / "the remaining lenses" so the text
cannot drift again.

A3. **Pi-only dispatch literal leaks to every target.**
`subagent-driven-development/SKILL.md.tmpl:33` instructs dispatch "with
`allowCommits: true`" outside the `targetSubagentTools` conditional. That knob
exists only in the Pi subagent extension (ADR-0123/0177). The implementer
contract defaults an unspecified mode to commit-disabled helper, so a Claude
dispatcher following the skill literally gets a phase owner that refuses to
commit. Fix: move the literal into the Pi branch; the generic branch says to
state phase-owner mode in the brief.

A4. **Fictitious gate fast/full split (repo-local).** `.awf/config.yaml:200`
sets `gateCmdFull: ./x gate full`, but `x:19-22` documents that the `full` arg
exists only for hook compatibility and runs the identical gate. Rendered
debugging and bugfix skills therefore instruct choosing between two tiers that
are the same command. Fix: unset `gateCmdFull` (or implement a real fast tier).

A5. **Resync skill contradictions.** The Pi branch of
`reviewing-plan-resync/SKILL.md.tmpl:25` omits the plan-path identification
instruction (it lives only in the else branch), so the Pi render never says how
to find the plan. Frontmatter and when-fires claim reviewing-adr is the sole
invoker, but `reviewing-plan` step 7 also invokes resync; the catalog trigger
inherits the false claim.

A6. **Memory-writer contradiction.** sdd step 4 offers a user-transfer
exception letting the child edit parent memory; the implementer contract
(`implementer.md.tmpl:43-45`) unconditionally forbids it. Drop the exception
(consistent with the agent guide). Same sentence has subject drift: "the hook
repeats the staged check as defense in depth, then creates the declared
phase-closing commit" - grammatically the hook commits. Split the sentence.

A7. **Category-count desync in refactor-coupling-audit.** The template
hard-codes "6-category" / "Preserve all six categories", but this repo's
sidecar drops categories 4 and 5: the rendered skill numbers its headings
1, 2, 3, 6, its Output block demands "Codegen sites:" and "Constructor paths:"
lines the reader was never taught, and the note pointing at "the `customise:`
hints above" dangles at nothing. Fix in the template: uncount the prose,
unnumber the headings, make the Output block track the surviving categories.

A8. **adr-lifecycle freeze rule too narrow.** Line 91 freezes the body "once
`Accepted` or `Implemented`", omitting Implementing and Abandoned; the Notes at
line 98 state it correctly ("any live state"). Fix line 91 and cut the overlap.

A9. **Catalog relationship drift** (`internal/catalog/standard.go:260-277`,
renders into every adopter's guide):
- debugging lists `executing-direct` as a follow-up, but no body routes that
  hand-off and executing-direct's own entry condition ("only after
  brainstorming") contradicts it.
- tdd's actual invokers (bugfix, debugging) appear nowhere in its
  relationships, and its listed follow-ups never reference tdd back
  (executing-plans' tdd-opt-in section renders empty).
- refactor-coupling-audit's follow-ups omit `proposing-adr`, the one skill its
  output exists to feed; its `UsuallyFollows: exploring` is inverted (exploring
  is invoked from inside the audit).
- roadmap-graduation's "shipped roadmap item" trigger covers one of the body's
  three cases (graduating to ADR, graduating to PR, explicit drop), and the
  frontmatter description contradicts the catalog trigger. Its commit
  instruction also puts the reason in the subject and then says the reason
  goes in the body.
- executing-plans and subagent-driven-development tell mixed plans to hand
  phases to each other, but neither lists the other in the catalog.

A10. **Kind vocabulary contradiction.** Four skills self-describe as "a task
skill: it sits off the workflow chain" (tdd, adr-lifecycle,
refactor-coupling-audit, roadmap-graduation) while the catalog classifies all
four `WorkflowSupport` and the guide renders "(support)"; the actual
`WorkflowTask` skills are bugfix and debugging. One vocabulary must win across
bodies, catalog, and glossary (the glossary's `taskSkillRows` description is
also stale: the rendered guide lists all skill kinds).

A11. **Degraded-prose defect.** `templates/partials/review-spine-tail.md:19`
renders "Review review complete" when `digestLabel` is unset - no unresolved
token, but not coherent generic prose. Move "review complete" inside the
with-branch.

A12. Smaller accuracy items: bugfix step 5 never names `reviewing-impl`
despite the frontmatter promise (add the skill-conditional);
`.awf/skills/parts/debugging/debugging-surfaces.md` says "after any sync" (no
such command; say render) and "the target-native governed exploration loader"
where this repo can just say `awf-exploring`; tdd says "validate" an effort
where siblings say "create or resume"; grounding-checker uses "the effort"
generically where effort is a reserved term; the dirty-stop inventory in both
execution skills asks for "prior concerns", a field the implementer's stopped
report does not define.

## B. Concision (the commissioning ask)

Roughly 150+ template lines are removable or dedupable with no instruction
loss. The recurring patterns:

B1. **Model-selection boilerplate.** The ~75-word "Choose the smallest model
expected to complete reliably: small / standard / large" paragraph appears 16
times across the four review-skill templates (2 sections x 2 branches each),
plus executing-plans, sdd (twice), exploring (both branches, with an internal
self-repeat), and brainstorming (twice) - and the same paragraph is already in
every adopter's AGENTS.md. Constraint: the text is invariant-backed
(`rendering/workflow-skill-templates:deliberate-subagent-model-selection`,
`internal/project/subagent_model_selection_test.go`). Two-tier fix: (a) now,
extract a model-selection partial - rendered output unchanged, test stays
green, ~20 template copies become 1-2; (b) rendered-level dedup (state once
per skill or point at the guide) requires an ADR updating that claim. This is
the concrete scope for the long-deferred reviewer-spine-dedup ADR.

B2. **Effort/working-memory triple statement.** Nearly every skill restates
the working-memory rules (one effort, owned path, one writer, standalone
forbidden) in its procedure preamble, while the checkpoint partial included
later in the same file restates them again, and AGENTS.md carries them a third
time. Trim every preamble to the operative novelty (carry the slug and exact
owned path; reviewer receives them read-only) and let the checkpoint partials
own the protocol. Affects brainstorming, proposing-adr, writing-plans, all
four reviewing skills, executing-direct, executing-plans, sdd, bugfix,
debugging, tdd, roadmap-graduation, plus the two checkpoint partials whose own
items 1-2 overlap each other.

B3. **Notes sections that restate the file.** proposing-adr Notes restate
lines 63/33/12; adr-lifecycle Notes restate the intro and step 4;
writing-plans Notes restate its positioning line; reviewing-impl states its
independence three times and its docs-only skip rule twice; roadmap-graduation
states each of its two rules three times in 67 lines (cut the failure-modes
section and step 5); refactor-coupling-audit's "Test-coupling planning rule"
section restates category 2 including the same bolded sentence. debugging's
symptom-list default and When-to-invoke render back-to-back near-verbatim.
executing-plans contains the literal sequence "and do not drift from the plan.
No drift from the plan."

B4. **Un-extracted cross-file spines** (beyond B1): the execution spine shared
by executing-plans and sdd (~12-15 lines: dirty-stop recovery, staged
transaction, V2 ownership - already micro-drifting: "restart" vs
"redispatch"); the exploration breadth/detail/outcome ladder duplicated
between the exploring skill and the explorer agent (~15 lines; the skill also
instructs restating contracts the agent already carries, against the idiom the
other pairings use); the `awf context`/`awf topic` orientation sentence x5
files; the staged-transaction prose x3 skill sites (implementer keeps its
numbered child form); the larger-work escalation menu x3; the testing-oracle
and coverage-never-regresses blocks x3; the INDEX-regen block twice per file
in proposing-adr and adr-lifecycle; agent intro sentences duplicated verbatim
across the three reviewer templates (move into review-spine-head); explorer
and grounding-checker each repeat their identity sentence six lines apart.

B5. **Repo-local sidecar duplication.** `.awf/agents/plan-reviewer.yaml`'s
`step-exactness` focus item (~200 words) is a near-verbatim reorder of the
universal executability lens rendered in the same agent; reduce to its genuine
deltas or delete. `.awf/agents/adr-reviewer.yaml` has two focus items
restating universal lenses 1 and 4 (unlike code-reviewer.yaml, no "kept
deliberately" note) and an INDEX-regen item the template tail already covers.

B6. Filler and trivia: bugfix's default test-tier section steers nothing;
exploring's "Pi is deeply integrated" architecture note steers nothing;
"larger authority batch" and "audit shape" are undefined jargon at their use
sites; "V2 operation batches" appears in two skills with no referent.

## C. What is fine

The implementer agent template is the tightest prose on the surface and its
contract matches both dispatch skills end to end (modes, green obligation,
closing-subject flow, report shapes) apart from A3/A6. The not-found sentinel
matches exactly on both sides of the exploring pair. adr-lifecycle's contract
content matches docs/decisions/README.md. The checkpoint/context-spill
partials match `awf context --help` exactly. Publication safety holds
everywhere except A11. Frontmatter is accurate and terse except
reviewing-impl's (no "Use ..." trigger, no mention of its conformance-audit
and worktree-routing duties) and the roadmap-graduation/resync cases above.

## D. Proposed fix routing

1. **Mechanical template batch** (no rendered-behavior change beyond corrected
   words): A1, A2, A5, A6, A8, A11, A12, B3, B6, and the B2 preamble trims.
   Template edits + `./x render` + `./x check`; goldens update with them.
2. **Repo-local config batch**: A4 (`gateCmdFull`), debugging-surfaces part
   refresh, adr-reviewer.yaml trims.
3. **Structural partial extraction** (rendered output identical or
   near-identical): B1 tier (a), B4 partial set. Mechanical but touches many
   files; a small plan is warranted.
4. **Catalog/code changes**: A3, A7 (template restructure), A9, A10 - touch
   `internal/catalog/standard.go` and template structure; direct execution
   with review.
5. **Needs an ADR**: B1 tier (b) - rendered-level model-selection dedup and
   any once-per-skill spine statement, because the invariant claim
   `deliberate-subagent-model-selection` must change with it. This subsumes
   the deferred reviewer-spine-dedup backlog item.
6. **User decisions**: delete vs trim `plan-reviewer.yaml step-exactness`
   (B5); which kind vocabulary wins in A10 (reword bodies to "support" vs
   reclassify); whether roadmap-graduation's commit reason lives in subject or
   body (A9).
