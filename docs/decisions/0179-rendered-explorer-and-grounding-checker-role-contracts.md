---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0179: Rendered explorer and grounding-checker role contracts

## Context

ADR-0177 established the thesis and took one of its three roles: a dispatched role's child-facing
contract is a rendered agent artifact, not a literal string inside the generated Pi extension. It
scoped itself to the implementation role and named the two claim revisions its successor would owe.
This is that successor.

Two of Pi's four dispatch roles (`Role` is `grounding | explore | review | implement`,
`templates/pi/awf-subagents/runner.ts.tmpl:7`) still carry their persona prose as TypeScript literals
in `rolePrompt()` (`templates/pi/awf-subagents/index.ts.tmpl:174-201`). The function now has exactly
two callers, the grounding dispatch at line 695 and the explore dispatch at line 723, so removing both
branches deletes the function rather than leaving a stub. The asymmetry ADR-0177 called "deliberate
and temporary" is the whole of what remains.

The consequences of the literal home are the three ADR-0177 recorded. The prose is invisible to review
of rendered output, unreachable by drift checking, and Pi-only by construction: a Codex or Claude
adopter dispatching exploration gets a skill-authored brief with no equivalent of the contract the Pi
child receives. The explore literal is the largest of the three at thirteen array entries
(`index.ts.tmpl:187-199`), which makes it the worst-placed.

Three facts about the explore role shape how far the artifact can go.

The explore prompt is parameterised. `Selected breadth maximum: ${options.breadth}` and
`Selected report detail: ${options.detail}` interpolate per call, while a rendered agent file is
static. `loadImplementer` already solved exactly this shape by appending the per-call authority mode
to the loaded body, so the explorer appends breadth and detail the same way. This is precedent, not
novelty.

Part of the explore prose is Pi-specific and must not move. Entry 189 carries two sentences with
different homes: the parent may run independent information needs concurrently while refinement stays
sequential, which is a runtime-neutral statement about dispatch discipline, and "Pi admits at most ten
active exploration children and queues the rest FIFO with abort-aware removal", which describes one
runtime's limiter. An agent body renders for every target and must stay runtime-neutral, as
`implementer.md.tmpl` is, so only the second sentence stays Pi-side. That split keeps
`internal/project/target_test.go:435`'s ban on `ten exploration children` and `queues the rest FIFO`
in non-Pi output satisfied, and keeps the `ExplorationBreadth` and `ExplorationDetail` aliases in use
rather than orphaned by `rolePrompt`'s deletion. Nothing would flag them if they were orphaned:
`tsconfig` sets no `noUnusedLocals`.

Both revisions ADR-0177 named are genuinely forced, not stylistic.

`rendering/workflow-skill-templates:bounded-exploration-reporting` says "The rendered exploration
guidance and Pi's fixed prompt define adaptive breadth and grounded reporting". Its proof
`TestBoundedExplorationReporting` (`internal/project/target_test.go:454-490`) checks two bodies, the
rendered exploring skill guidance and the `rolePrompt("explore")` output labelled "Pi fixed prompt",
each against its own `wants` slice with target-specific spellings: the skill list wants
`Ground every material claim with file/line evidence` where the Pi list wants the `file:line`
spelling. Moving that prose into an agent mechanically falsifies the phrase "Pi's fixed prompt",
because after this change Pi's fixed contribution is three appended lines and the contract proper is a
rendered artifact shared with every target.

`rendering/workflow-skill-templates:cross-runtime-exploration-dispatch` constrains the non-Pi branch
to "a generic target-native fresh-context exploration subagent". Once the explorer renders for every
target, that genericity has no remaining rationale: exploring and grounding are the last two dispatch
branches whose non-Pi prose names no agent. `reviewing-impl` names `code-reviewer` in its non-Pi
branch (`templates/skills/reviewing-impl/SKILL.md.tmpl:34`), and `executing-plans` and
`subagent-driven-development` both name `implementer`. A grounding check confirmed that
`rendering/pi-workflows:pi-dedicated-grounding-dispatch` bans Pi *tool* names from non-Pi output, not
agent names: every enforcement site is a literal token list and none mentions an agent name
(`internal/project/subagent_model_selection_test.go:75`, `target_test.go:435`,
`internal/project/spine_test.go:939` and `:1461`).

A grounding check also refuted one premise this design started from and found pins it had missed.

`RequiresAgent` is single-valued, which is sufficient only because pairing is restricted to direct
dispatchers. There is no transitive closure to lean on: `init()` at
`internal/catalog/standard.go:248` empties `RequiresSkills` for every profiled skill at `:272`, so the
edge brainstorming to exploring to explorer does not exist, and `target_test.go:441` enumerates three
exploration consumers (`brainstorming`, `debugging`, `refactor-coupling-audit`). Only `exploring` and
`brainstorming` dispatch an agent directly, and both currently carry an empty `RequiresAgent`.

The phrase `queues the rest FIFO with abort-aware removal` occurs exactly once in `index.ts.tmpl`
(line 189) and `target_test.go:298` pins it as proof for
`rendering/pi-workflows:pi-structured-exploration-contract`. Deleting the explore branch reddens that
test. The claim sentence itself survives, because `MAX_EXPLORATION_CONCURRENCY` and `createLimiter`
remain and the limiter prose merely relocates to a per-call suffix, so this is a scheduled test edit,
not a claim operation.

The generic phrase `target-native fresh-context exploration subagent` is pinned in four places
including the `unsetFallbackCases` table (`target_test.go:429` and `:491`, `spine_test.go:939` and
`:1459`). Adding the agent name beside it rather than substituting for it leaves all four untouched.

One further correction is folded in here because this ADR already revises claims in its topic.
ADR-0177 authored `implementer-role-contract`'s second sentence as "The subagent-driven-development
and executing-plans skills name that agent in every dispatch branch and address their own imperatives
to an explicit subject." The second half is broader than its proof. `TestImplementerAgent`
(`spine_test.go:277`) checks the agent name across both capability shapes for both skills, which does
establish "every dispatch branch", but for subjects it pins five specific literals: the raise-concerns
imperative in each skill (`spine_test.go:350` and `:357`), plus design preservation and the context
call (`:360-361`) and batch inventory (`:365`) in `executing-plans`. Nothing verifies that every
imperative in either skill carries a subject. This is the failure mode the ADR-0177 retrospective
promoted as a lesson, caught in that ADR's own prose.

The shared loader this work would have justified extracting has already landed as a standalone
no-behaviour-change refactor (`732c060e`), ahead of this ADR and outside it. `loadAgentContract` owns
the read, the missing-file repair, the frontmatter strip, and the empty-body error; each role supplies
a `ContractSource` of noun, repair clause, and prepend (`index.ts.tmpl:210-216`). That sequencing was
chosen because a green-throughout refactor does not belong inside a schema-bumping transaction, and
because the behaviour it preserves is already named by `pi-implement-role-artifact` without reference
to any function or message text, so it needed no claim operation of its own.

## Decision

1. Add two agents to the standard catalog, `explorer` and `grounding-checker`: an `AgentSpec` each in
   `internal/catalog/standard.go` plus `templates/agents/explorer.md.tmpl` and
   `templates/agents/grounding-checker.md.tmpl`. Both keep `RequiresSkills` empty, for the same reason
   `implementer` does: the contract deliberately routes the child into no skill.

2. The rendered explorer body carries the eleven runtime-neutral clauses now in the explore branch of
   `rolePrompt`: report-only identity with read and evidence-producing commands only; exactly one
   information need with no bundling and no recursive delegation; the dispatch-discipline sentence that
   independent information needs may run concurrently as separate calls while refinement of an earlier
   result stays sequential; breadth ordered targeted, bounded, broad, with its per-tier definitions;
   breadth as an adaptive maximum to be widened only on evidence and never past the selection; the
   project search universe definition for broad searches; report detail ordered paths, summary,
   analysis, with its per-tier definitions and its independence from breadth; file:line grounding for
   every material claim; the three-way not-found, inconclusive, and unverified distinction with the
   exact not-found opening and the broad-absence obligation; final report only, no search narrative;
   and statelessness across calls. The dispatch-discipline sentence is named explicitly because
   `bounded-exploration-reporting` requires it and `target_test.go:472` pins both of its halves.

3. The rendered grounding-checker body carries the current grounding literal plus the five
   child-obligation bullets that migrate out of the brainstorming skill
   (`templates/skills/brainstorming/SKILL.md.tmpl:49-53`): verify the design's factual premises against
   source, surface unstated assumptions and edge cases, flag altitude and scope, check fit against
   current-state claims and Accepted or Implemented ADRs, and return findings in the closed
   `{kind, topic, detail, grounding, confidence}` shape with the three confidence values defined.

4. Neither body carries per-call or Pi-specific text. The Pi extension deletes `rolePrompt` entirely
   and both call sites load their contract through `loadAgentContract`, supplying all three
   `ContractSource` fields. Each role gets its own noun and repair clause, so both tools fail closed
   with an actionable enable-and-render hint exactly as `subagent_review` and `subagent_implement`
   already do, and its own `prepend`, which is required and interpolated unconditionally: the explorer
   and grounding-checker prepends carry the report-only identity line, as the reviewer prepend does.
   The per-call suffix is separate from the prepend: the explorer appends breadth, detail, and the
   Pi-only limiter sentence about ten active children and abort-aware FIFO queueing, while the
   grounding call appends no suffix at all.

5. Both dispatching skills name their agent, symmetrically. The `exploring` skill's non-Pi branch adds
   the `explorer` name and the `brainstorming` skill's non-Pi grounding branch adds the
   `grounding-checker` name, each keeping its existing generic phrase, so both are additions rather
   than substitutions and all four sites pinned on `target-native fresh-context exploration subagent`
   stand. Treating grounding asymmetrically would render a contract to disk that no rendered prose
   directs anyone to dispatch, and would strip the five obligations from the only surface that carries
   them today for non-Pi adopters. The `brainstorming` skill keeps its
   `grounding-check-output-format` section name and its parent-facing paragraphs, the brief synthesis,
   the surface-findings rule, and the advisory single-pass rule; the `:48` lead-in "Ask the subagent
   specifically to:" is rewritten to point at the grounding-checker contract rather than left dangling
   above removed bullets.

6. Pair the two direct dispatchers: `exploring` gains `RequiresAgent: "explorer"` and `brainstorming`
   gains `RequiresAgent: "grounding-checker"`. Two mechanical consequences land in the same commit.
   `nonReviewingDispatchers` (`internal/catalog/catalog_test.go:107-110`) is a closed allowlist and
   `TestReviewingSkillSpecsArePaired` errors at `:123` on any non-`reviewing-` skill carrying
   `RequiresAgent`, so both new pairings fail until that map names them. Nine fixtures enable
   `exploring` or `brainstorming` without the paired agent and fail project open until they add it:
   `internal/project/render_tree_test.go:115` and `:141`,
   `internal/project/skillrefs_test.go:88`, `internal/project/context_artifacts_test.go:260`,
   `internal/project/unused_test.go:63` and `:175`, `internal/project/spine_test.go:1704`, and
   `internal/project/target_test.go:367` and `:455`. Eight carry a bare `agents: []`; the exception is
   `explorationFixtureConfig` at `target_test.go:367`, whose list is populated but predates both new
   agents.

   The discriminator is whether a site reaches `Open` with a non-local dispatching skill, not whether its
   config names one, because `checkNodeRequirements` only errors while the `applyCloseEnabledSet`
   migration path self-heals. Three sites that look affected are therefore not:
   `internal/project/project_test.go:1458` and `internal/project/skillrefs_test.go:102` each pair
   `brainstorming` with a `local: true` sidecar, which `checkKindAgainstCatalog` skips outright
   (`internal/project/validate.go:105`), the first of them precisely to prove that exemption;
   `internal/migrate/closeenabledset_test.go:82` never opens a project at all, since `closeFixture` only
   writes config and sidecars. That last fixture instead gains an assertion that the migration closes the
   new pairing edge, which its currently-empty positive-want loop asserts nothing about today. The
   `target_test.go:455` fixture must gain `agents: [explorer]` specifically, or item 8's revised
   `TestBoundedExplorationReporting` cannot render the body it asserts against. The `RequiresAgent`
   doc comment (`internal/catalog/catalog.go:57-61`) enumerates the field's users as reviewers plus
   the implementer and is falsified by this item; it is reworded generically over dispatchers rather
   than re-enumerated, so the next pairing does not have to touch it.

   `debugging` and `refactor-coupling-audit` route through the `exploring` skill and cannot be closed,
   because the emptied `RequiresSkills` provides no transitive edge. That gap is pre-existing and
   unchanged by this work; it is accepted and recorded, the same shape ADR-0177 accepted for a Pi tree
   enabling neither dispatching skill.

7. One migration to config-schema generation 24, reusing `applyCloseEnabledSet` as generation 23 did
   (`internal/migrate/migrate.go:57`), with both config trees enabling the two agents in the same
   commit. Two parts of the version handling are distinct. The `minVersionBySchema[24]` entry is
   forced: ADR-0177 proved that omitting it makes every gated command refuse. The target version is a
   choice, because `internal/project/version_test.go` only requires the highest mapped generation to
   equal `Version`, so mapping 24 onto the existing 0.27.0 would also pass. This ADR chooses `0.28.0`
   and a matching `project.Version` bump anyway: 0.27.0 is already the declared floor for generation
   23, and reusing one version as the floor for two generations would make the refusal message
   unable to distinguish them. That 0.23.0 through 0.27.0 have not yet been released
   (`changelog/CHANGELOG.md`'s newest release heading is 0.22.0) is a pre-existing release-cadence
   matter this ADR does not address.

8. Seven test-edit obligations land with the change rather than being discovered by a red gate. Three
   follow from the moved prose: `TestBoundedExplorationReporting` is revised so its second body is the
   rendered explorer agent, its label no longer claims a Pi fixed prompt, and its single Pi `wants`
   slice splits three ways into an explorer-body list, a retained Pi-body list for the limiter and the
   per-call breadth and detail lines, and the existing skill-body list which keeps its own spellings;
   `target_test.go:298`'s pin of `queues the rest FIFO with abort-aware removal` moves to the Pi
   per-call suffix, keeping `pi-structured-exploration-contract` proven without a claim operation; and
   both new roles get the TypeScript behaviour seam through `h.requests[0].systemPrompt` in addition to
   Go source pins, because that seam caught four real mutations in Part A that source pins alone
   missed. Four follow from item 7's generation bump, exactly as generation 23's own commit had to
   make them: `internal/project/version_test.go:15` (`minVersionBySchema[23] != Version` breaks on the
   version bump), `version_test.go:29` (`ValidateSchemaMinimumVersion(24, Version)` asserts "no
   minimum" and breaks once 24 is mapped), `internal/migrate/dropworkflowtelemetry_test.go:11`
   (`Current() != 23`), and `internal/migrate/workflowtelemetry_test.go:64` (the joined
   applied-migration-name list gains the new migration).

9. Narrow `implementer-role-contract`'s second sentence to what its proof establishes: both skills
   name the agent in every dispatch branch, and their parent-facing imperatives for raising concerns,
   preserving the plan's settled design, running the context command, and inventorying batch returns
   carry an explicit subject. Naming those four categories covers all five pinned literals without
   asserting anything about imperatives no test checks. No test changes; the claim stops overreaching
   its evidence in one direction without under-reaching in the other.

10. Update `pi-implement-role-artifact` to shed the loader mechanics the new
    `pi-role-contract-loader` claim now owns. The read, the frontmatter strip, the enable-and-render
    repair, and the empty-body error become cross-role behaviour stated once; the implementer claim
    retains what is implement-specific, its `.pi/agents/` artifact path, its commit-authority prepend,
    and the before-and-after snapshot check that fails a commit-capable call whose HEAD is unchanged.
    This is a division of one claim's current content between two claims, not a weakening: no
    behaviour stops being asserted. It is also why `732c060e` needed no operation of its own, since
    the behaviour it preserved was already claimed and merely awaited a home that names every role.

11. Correct in the same commit the descriptive surfaces that two new `AgentSpec` entries falsify, the
    obligation ADR-0177 item 8 set the precedent for: `README.md:46` (the enumeration "Three review
    agents ... One `implementer` agent", which no longer covers the catalog), `README.md:249` ("and
    four agents", which becomes six), and `README.md:12` (the "independent review agents" framing of
    the agent set). A bare count bump would reintroduce ADR-0177's own error, so each enumeration is
    reworded rather than incremented. Two surfaces checked and found sound need no edit, recorded so
    they are not touched reflexively: `internal/configspec`'s agents-key description is generic with no
    count or enumeration, and its only numeric rendering in `docs/config-reference.md` is a generated
    enabled-count column that item 12's `./x render` already refreshes; `cmd/awf/main.go`'s package
    comment names the artifact kinds without counting them.

12. Every status transition on this ADR runs `./x render` and commits the regenerated
    `docs/decisions/INDEX.md` and lock alongside the status change. The implementing commit also adds a
    `changelog/CHANGELOG.md` `[Unreleased]` entry covering generation 24 and the two tools that begin
    failing closed, as `docs/releasing.md:66` requires for adopter-facing change and as generation
    23's commit did.

13. This ADR scopes to the contract artifacts and their loading, not to the wider "dispatch spine"
    ADR-0177's prose anticipated. The `run`, `toolResult`, and metadata boilerplate around the four
    dispatches stays as it is: it is genuine per-tool wiring with differing tool sets, model routing,
    and result shapes, and collapsing it would trade a real abstraction boundary for line count. The
    shared loader that Part B did justify is already extracted (`732c060e`).

## State changes

- add `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`
- add `rendering/pi-workflows:pi-role-contract-loader`
- update `rendering/pi-workflows:pi-implement-role-artifact`
- update `rendering/workflow-skill-templates:bounded-exploration-reporting`
- update `rendering/workflow-skill-templates:cross-runtime-exploration-dispatch`
- update `rendering/workflow-skill-templates:implementer-role-contract`

## Consequences

The thesis is complete. All six rendered role contracts then exist as artifacts, `rolePrompt` is gone,
and no role prose remains inline in the generated extension. Changing what an explorer or
grounding-checker child is told becomes a template edit under `awf render` and `awf check` rather than
a TypeScript string edit invisible to both.

Non-Pi adopters gain the most. The explorer's eleven clauses and the grounding-checker's five obligations
currently reach only Pi children; as agent artifacts they render for every target, all six of which
have an `AgentDir`. This is the first time a Codex or Claude adopter's exploration and grounding
children receive the same contract the Pi ones do, which is why item 5 names both agents in both
non-Pi branches rather than only the explorer: naming one and not the other would render a contract
nobody is told to dispatch while deleting the obligations that skill previously carried inline.

Two new `AgentSpec` entries trigger the same six machine-forced authoring obligations ADR-0177
inventoried, twice over: a golden test each in `internal/project/spine_test.go`, `dataKeys` entries in
`internal/configspec` for every data key, `awf:section` marker parity between template and spec, a
hand-authored unset-data fallback for every conditional, a leak-free empty-data render, and
conformance to `rendering/catalog-and-targets:catalog-defaults-generic-denylist`. Both bodies must
also be authored around `rendering/workflow-skill-templates:skill-prose-tool-agnostic`, which bans
backticked `read`, `write`, and `edit` and phrases such as "read tool". The explorer body is the
longest contract yet written under that constraint, since its subject matter is searching and reading.

`subagent_explore` and `subagent_grounding` start failing closed, which is a behaviour regression for
one adopter shape: a Pi tree that enables the pi target but not the dispatching skill loses a tool
that works today off the literal string. Generation 24 closes the enabled set only for adopters who
already enable `exploring` or `brainstorming`, and the precedent is exact, since `subagent_review` and
`subagent_implement` both already fail this way.

`workflow-skill-templates` goes from eighteen claims to nineteen, which is why both new agent bodies
share one combined claim instead of taking one each: the advisory ceiling is twenty, and two separate
claims would leave the topic one addition from it. The loader claim lands in `pi-workflows` (sixteen
claims, so seventeen) instead, where it belongs on subject matter anyway, since it describes extension
behaviour across every disk-loaded role rather than template content.

The pairing is cheap to declare and not cheap to land. Item 6's two field assignments force an
allowlist edit, nine fixture edits, and a doc-comment rewording; that fallout is inventoried rather
than discovered, but it is the bulk of the diff's file count.

The `debugging` and `refactor-coupling-audit` pairing gap stays open. Enabling either without
`exploring` still yields a skill that instructs a dispatch whose agent may be absent. Closing it needs
either a multi-valued `RequiresAgent` or a live `RequiresSkills` closure, both of which are larger
changes than this ADR should carry, and neither is made worse by it.

Narrowing `implementer-role-contract` sets a precedent worth naming: an over-broad claim sentence is
corrected by a `State changes` update on the claim, not by editing ADR-0177, whose meaning is frozen.
The retrospective lesson that caught it is thereby exercised once against real prose.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Two separate claims, one per new agent body | Would push `workflow-skill-templates` to twenty claims, exactly at the advisory ceiling, to express one commitment that reads identically for both roles. |
| Split Part B into two ADRs, one per role | The two roles share every structural question: the loader, the deletion of `rolePrompt`, the pairing mechanism, and the migration. Two records would duplicate all of it to separate two template files. |
| Name only the explorer in non-Pi prose, leaving grounding generic | Renders a grounding contract to disk that no rendered prose directs anyone to dispatch, and strips the five obligations from the only surface non-Pi adopters have for them today. |
| Move the Pi limiter prose into the explorer body | The body renders for every target and must stay runtime-neutral. It would also leak `ten exploration children` into non-Pi output, which `target_test.go:435` bans. |
| Substitute the agent name for the generic phrase in the non-Pi branches | Redundant churn against four pinned sites, including an `unsetFallbackCases` entry, for no gain; adding the name keeps the generic description that still reads correctly. |
| Rename `grounding-check-output-format` to match its narrowed content | Section names are catalog-declared and are the key adopters use for a sidecar override, so renaming is a breaking config-surface change to fix a slight misnomer. |
| Map generation 24 onto the existing 0.27.0 instead of bumping | Passes the gate, but makes one version the declared floor for two generations, so the refusal message can no longer distinguish which generation a stale binary is behind. |
| Extract the full dispatch spine as ADR-0177's prose anticipated | The four dispatches differ in tool set, model routing, and result shape. Only the contract loader was genuinely duplicated, and it is already extracted. |
| Leave the two roles as literals and only add the pairing | Preserves all three defects the thesis exists to fix, and leaves non-Pi adopters with no contract at all. |

## Status history

- 2026-07-30: Proposed
