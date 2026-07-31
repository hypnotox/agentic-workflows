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

## D. Corrections to sections A and B (2026-07-31 grounding pass)

A grounding check against HEAD revised the findings above. Read these before
acting on any finding.

D1. **A8 is withdrawn.** It would regress an Implemented decision.
`templates/skills/adr-lifecycle/SKILL.md.tmpl` at HEAD already states that the
body stays amendable through `Accepted` and `Implementing` and freezes at
`Implemented` or `Abandoned`, matching the live claim
`adr-system/adr-lifecycle:adr-amendable-until-terminal` (Origin ADR-0188,
Backing: test). Do not widen the freeze rule.

D2. **The corpus moved during the audit, so line numbers above are not
trustworthy.** The amendment-until-terminal prose landed at 2026-07-30 22:25;
this report committed at 23:48; ADR-0189 was proposed at 2026-07-31 00:11.
Concurrent sessions work in this checkout. Any plan derived from this report
must re-verify each finding against HEAD before freezing.

D3. **A5 refined.** Unnumbering the visible refactor-coupling-audit headings
and uncounting the prose is safe and needs no catalog or test change. Renaming
the `category-N-*` section ids would break `skill-section-parity` (ADR-0054)
and orphan adopter override parts, so ids stay stable. The mechanism that
leaves headings sparse is a convention part overriding a section with empty
content, not "a sidecar dropping categories".

D4. **A2 refined.** `templates/skills/bugfix/SKILL.md.tmpl:8` also says "task
skill" and is correctly `WorkflowTask`; leave it. The four to reword are tdd,
adr-lifecycle, refactor-coupling-audit and roadmap-graduation.

D5. **A4 is wider than stated.** Unsetting `gateCmdFull` also rewrites the
rendered `.awf/hooks/pre-push.sh` and falsifies four hand-written convention
parts (`.awf/parts/workflow/local-hooks.md`,
`.awf/parts/workflow/composing-the-gate.md`, `.awf/docs/parts/testing/tiers.md`,
`.awf/docs/parts/development/command-runner.md`) that render into
`docs/workflow.md`, `docs/testing.md` and `docs/development.md`. Same commit.

D6. **A3 costs three test edits.** Moving `allowCommits: true` inside the
`targetSubagentTools` conditional fails `internal/project/spine_test.go:866`,
`internal/project/spine_test.go:551` and
`internal/project/phase_transaction_ownership_test.go:80`, two of which are
invariant proofs (ADR-0168, ADR-0166). Neither claim text mentions
`allowCommits`, so removing it from generic-render expectations is a scope
correction; the plan must quote both claim texts and say so, or terminal review
will read it as an assertion weakened to hide a failure.

D7. **A6 costs a catalog edit.** Merging the path-identification section pair
requires dropping the matching entry from the `Sections` list in
`internal/catalog/standard.go` in the same commit. Nothing else keys off those
names.

D8. **A5/A9 additions.** The resync single-invoker claim is false in three
places, not one: the skill body, its frontmatter description, and
`templates/agents/plan-reviewer.md.tmpl`. `reviewing-plan`'s when-fires prose
enumerates five lens names rather than counting them, which is the same
staleness in a different shape.

D9. **B1 is smaller and less separable than stated.** Include directives are
line-anchored, and every model-selection block sits mid-sentence inside a
numbered list item, so extraction forces a rendered change: there is no
"source-only, rendered-identical" tier. Roughly half the block is the Pi and
non-Pi branch rule, which is claim-load-bearing and test-pinned verbatim; only
the tier glosses compress. The claim update, template compression, test
literals, ADR flip and render output must land in one commit, because
`awf check` validates claim provenance against the declaring ADR's operations.

## E. Settled decisions

E1. **Prose economy is compress-with-reference.** Each dispatch site keeps a
one-line rule naming the small, standard and large tiers plus the escalation
trigger; the definitions live in the agent guide and a shared partial. Not full
deferral. The governing reason is that a dispatched subagent may not have the
agent guide in context (Pi loads only the agent artifact, per ADR-0179); it is
NOT that an adopter could disable the guide, which is impossible because
`agents-doc` is mandatory and ADR-0061 keeps mandatory docs out of the
toggleable pool.

E2. **`plan-reviewer.yaml` `step-exactness` reduces to a single sentence**
carrying the consolidated Reject list. Both originally nominated deltas are
already covered by the universal executability lens; the surviving delta is the
imperative framing, which is a real steer for an LLM reviewer. Its sibling
focus items carry incident narrative found nowhere in the universal lenses and
are not swept into this trim.

E3. **Skill-kind vocabulary: the catalog wins.** The four bodies are reworded
to "support skill"; catalog `Kind` values are unchanged, so no adopter's
rendered guide changes. The adjacent layer that uses "task skill" to mean "any
non-chain skill" (the `taskSkillRows` render key, the glossary line, and the
test comments) is renamed in the same plan so one vocabulary holds inside and
out.

E4. **roadmap-graduation's commit reason lives in the body**, with the subject
staying `docs(roadmap): drop <item>`.

E5. **An ADR still gates the rendered prose economy**, but its justification is
that it authorises a durable policy (compress-with-reference plus the template
partials convention, subsuming the deferred reviewer-spine-dedup item), not
that the current claim text forbids the compression. The claim names the tiers
without mandating the definitional glosses, so a narrower reading would make
this a test-literal refresh; the ADR route is chosen deliberately.

## F. Status: parked pending ADR-0187

Execution is deliberately not started. ADR-0187 (Proposed, plan written at
`docs/plans/2026-07-30-orienting-support-skill-adr-0187.md`) creates
`templates/partials/orientation-ladder.md`, which is the same partial this work
would introduce, and also edits `internal/catalog/standard.go`,
`templates/skills/writing-plans/SKILL.md.tmpl`, the brainstorming and
proposing-adr templates, `.awf/config.yaml` and the lock, and the same
`rendering/workflow-skill-templates` current-state file this work must touch.
It additionally raises the schema generation and the project version. Running
both concurrently would conflict on every one of those files.

Resume trigger: ADR-0187 reaches `Implemented` and its plan is complete.

Resume checklist:
1. Re-verify every finding in sections A and B against HEAD; treat all line
   numbers as stale and expect some findings to have been overtaken, as A8 was.
2. Re-check whether ADR-0187 already absorbed the orientation partial, the
   catalog edits, and the writing-plans and brainstorming prose trims, and drop
   those items from scope if so.
3. Confirm the `rendering/workflow-skill-templates` claim budget still permits
   the planned operations; the topic was at its 20-claim ceiling and ADR-0187
   consumed a granted exception.
4. Then propose the ADR described in E5 and write the plan.
