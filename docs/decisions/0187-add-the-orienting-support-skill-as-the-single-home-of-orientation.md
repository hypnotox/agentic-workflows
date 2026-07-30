---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0187: Add the orienting support skill as the single home of orientation

## Context

Orientation, the act of grounding a session in a topic before acting, has no owner in the skill catalog. Its guidance is scattered: the brainstorming template carries an inline "explore project context" step, the agent guide carries the `awf context` call discipline as loose prose, and the workflow doc states that repository sources outrank checkpoint prose without any procedure that operationalizes the rule. The moment where that rule matters most, resuming an in-progress effort from working memory in a fresh session, has no procedural coverage at all: nothing directs a resuming agent to verify the checkpoint's claims (commits landed since, worktree topology, ADR statuses, cited file existence) against the repository before acting on them. Stale prose migrating forward as authority is a failure class the catalog already recognizes for roadmap entries (roadmap-graduation); the same class is unguarded at every topic entry point.

The same staleness risk applies mid-chain: ADR authoring and plan writing frequently run in a later session than the brainstorm that settled the design, and both write repository facts into durable records.

A grounding check against the repository established the mechanical constraints this decision honors: the include engine expands partials in a single non-recursive pass and hard-errors on a partial that itself contains an `awf:include` directive; workflow-profile validation rejects an empty `Purpose`; the skill-section-parity invariant requires the catalog `Sections` list to equal the template's `awf:section` marker set; `Core: true` affects only new-adopter pre-selection, and `awf upgrade` never auto-enables an artifact; the closure-migration primitive walks only structural requirement edges, and `requires-skills-exact` forces every standard skill's `RequiresSkills` empty, so no existing mechanism can backfill a skill that declares no agent or doc requirement.

User constraints, verbatim: "It shouldn't be a chain skill like brainstorming. It should be a utility skill used by the agent in fitting situations." "the grounding check subagent could probably also benefit from that spine." "it would probably be useful if the skill would enable to use one or more exploration subagents as they see fit."

## Decision

1. Add a standard support skill `orienting` to the compile-time catalog: a `SkillSpec` with `Core: true`, no `RequiresAgent` or `RequiresDoc`, and `Sections` exactly `when-to-invoke`, `guide-ladder`, `context-command`, `resume-revalidation`, `hand-off`; and a `WorkflowProfile` with `Kind: WorkflowSupport`, `Purpose` "Ground the session in a topic before starting, resuming, or widening work.", `Trigger` "Use when taking up a topic: before brainstorming fresh non-trivial work, when resuming an effort, or when taking over a handoff.", no `UsuallyFollows`, and `CommonFollowUps` brainstorming, debugging, writing-plans, executing-plans. No other skill's profile gains an edge to `orienting`; the profile graph stays advisory and the prose invocation is the router.
2. Author `templates/skills/orienting/SKILL.md.tmpl` with those five overridable sections. `when-to-invoke` names four moments: before brainstorming fresh non-trivial work; resuming an effort in a new session or after context summarization; taking over a handoff; and mid-chain re-orientation when the working set widens into unexamined files or domains, or a durable artifact is about to cite repository facts not verified in the current session. `guide-ladder` encodes the guide-first grounding order (agent guide, then relevant document-map docs, then domain docs, then recent history of the touched area) and explicitly permits dispatching one or more exploration subagents as fitting, one information need each, in parallel when independent, report-only. `context-command` encodes the managed `awf context` discipline (start bare, request only named facets, never prescribe `--full`) and carries the context-spill include. `resume-revalidation` directs the agent to read the effort memory header and handoff log (the file at the angle-bracket placeholder path `.awf/efforts/<slug>/memory.md`), then verify every load-bearing claim against repository truth before acting: commits landed since the checkpoint, worktree topology versus what memory describes, cited ADR statuses against the decision index, cited plan and file existence; a discrepancy resolves in favor of the repository, and only the one user-managed writer corrects the checkpoint. `hand-off` routes to the fitting next skill; the skill never commits, never creates an effort, is never a chain gate, and is single-pass.
3. Extract the orientation ladder (the guide-first grounding order and the `awf context` discipline) into one shared partial under `templates/partials/`, included via `awf:include` by both the orienting skill template and the grounding-checker agent contract in its verification-scope area. The partial itself contains no `awf:include` directive; the context-spill include stays a sibling directive in each consuming template. Resume-revalidation is not part of the partial: subagents never touch effort memory.
4. Reference-and-shrink the existing inline copies: the brainstorming template's inline explore-project-context step shrinks to an invocation of the rendered orienting skill; the proposing-adr template gains one advisory pointer sentence in its when-to-invoke area and the writing-plans template gains one in its confirm-scope step, each directing to the orienting skill when grounding is stale. Debugging and refactor-coupling-audit keep their domain-specific grounding untouched.
5. Add a schema generation whose migration enables `orienting` in every config that has `brainstorming` enabled, written as a bespoke migration (the closure primitive cannot express it), idempotent and atomic per the existing migration conventions. Configs without `brainstorming` are untouched.

## State changes

- add `rendering/workflow-skill-templates:orienting-single-home`
- update `rendering/workflow-skill-templates:explorer-and-grounding-role-contracts`
- update `rendering/workflow-skill-templates:implementer-context-grounding`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- add `config/migrations-and-locks:orienting-skill-backfill`

## Consequences

- Orientation gains one procedural home. The brainstorming template shortens, ADR and plan writing gain an explicit staleness guard, and the effort-resume moment is covered for the first time. The grounding-checker contract inherits the same ladder its parent skills teach, so its verification method and the main thread's orientation method stop drifting independently.
- Existing adopters with brainstorming enabled are backfilled by the migration, at the cost of a schema generation bump: every adopter must run `awf upgrade` before gated commands run again (binary-version gate). An adopter who deliberately disables `orienting` afterwards accepts a dangling prose reference in brainstorming's rendered body, the same acceptance that already exists for the exploring reference there.
- The render surface grows by one skill for every enabled target and by one shared partial; Pi picks the skill up with no extra wiring (pi-native-workflow-skills).
- Explicitly ruled out: `orienting` as a chain gate or prerequisite; effort creation inside `orienting`; inbound workflow-profile edges to it.
- Risk: the mid-chain re-orientation moment is judgment-based and may be over- or under-invoked; mitigated by keeping the skill single-pass and cheap, and by the advisory pointer sentences anchoring the two highest-value call sites.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Strengthen the existing homes (brainstorming step, workflow doc) without a new skill | Leaves the resume moment uncovered and the duplicated guidance free to drift |
| A resume/handoff-only skill | Misses the pre-work and mid-chain moments where durable records inherit stale facts |
| Ship without a backfill migration, ADR note only | Project precedent (the exploring closure generation) chose backfill over dangling references |
| Backfill via the closure primitive | Impossible: `requires-skills-exact` forces `RequiresSkills` empty and `orienting` has no agent or doc requirement to hang closure on |
| Nest the context-spill include inside the shared partial | Impossible: the include engine is single-pass and hard-errors on nested includes |

## Status history

- 2026-07-30: Proposed
