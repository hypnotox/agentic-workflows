---
format: current-state-v4
slug: bound-agent-guides-as-native-skill-routers
status: Implementing
date: 2026-08-06
---
# ADR-bound-agent-guides-as-native-skill-routers: Bound agent guides as native-skill routers


## Context

The repository's rendered `AGENTS.md` is loaded into every agent session and has regrown to 22,074 bytes. A fresh adopter render is about 12,393 bytes. The guide's own authoring standard says this surface pays its word cost on every task and must remain extra terse, yet the two largest sections duplicate information available elsewhere: Workflow reproduces every enabled skill's name, purpose, trigger, and relationships, while Invariants and Working memory carry mechanism and protocol detail already owned by current-state topics, skills, and workflow documentation.

ADR-0157 reduced an earlier 23,737-byte guide by making it an entry-point router, but deliberately required a catalog-derived trigger roster. That choice predates awf's current target posture. Both retained targets render native `SKILL.md` artifacts with required name and description frontmatter; Pi's pinned runtime smoke proves native skill discovery. Claude Code's native skill discovery is an external harness capability rather than an awf runtime contract, but awf already renders its selected skills at Claude's native paths. Repeating those descriptions in a target-neutral guide makes the harness and guide competing catalogs and charges every session for the copy.

The same accumulation has weakened progressive disclosure elsewhere. Full subagent model-tier definitions render in the guide even though governed skills carry their operative selection rule and `docs/working-with-awf.md` already explains routing. This repository's guide overrides repeat merge authorization, working-memory, command, and invariant mechanisms that already have canonical homes. The result contradicts the intended ownership boundary rather than merely exceeding an arbitrary length preference.

A durable bound is needed because prose-only review allowed the guide to regress after ADR-0157. The bound must remain advisory for adopters: an adopter may intentionally carry more project-specific context, and guide size alone is not a correctness failure. awf itself needs a stricter regression proof because its self-hosted guide models the standard it ships. No configuration surface or origin accounting is justified for a fixed progressive-disclosure opinion.

## Decision

1. `decision: native-skill-discovery-owns-catalog` Native harness skill discovery is the sole runtime catalog of enabled skill names and descriptions. The rendered guide carries no enabled-skill roster, trigger table, relationship list, or fallback catalog. A supported target that cannot expose its native skill artifacts must solve that capability at the adapter boundary rather than duplicating the catalog in `AGENTS.md`.
2. `decision: guide-is-dispatch-layer` The rendered guide is an always-loaded dispatch layer. It retains only project identity, genuinely cross-cutting imperatives, essential command entry points, the instruction to select native skills whose exposed descriptions fit, and concise links to canonical authority. Procedure, rationale, lifecycle detail, mechanism explanation, and subsystem-scoped rules remain in skills, current-state topics, ADR history, or documents reached through the guide's map. The revised templates preserve coherent missingkey=zero rendering and emit no unresolved-value token when data is missing or an optional string is empty.
3. `decision: guide-authoring-cost-test` The agent-guide authoring standard treats every sentence as an every-session cost: content belongs only when it is needed before native skill selection or routes the agent to canonical authority. It explicitly rejects native skill inventories and duplicated procedure, and treats a size budget as a regression signal rather than a target to fill.
4. `decision: guide-definition-ownership` Operative governed-dispatch rules remain with the skills that perform dispatch, while full reusable model-tier definitions live in `docs/working-with-awf.md`. The guide does not duplicate those definitions. Working-memory procedure remains canonical in `docs/workflow.md`; the guide carries only the minimum effort and resume routing needed before skill selection.
5. `decision: fixed-guide-budgets` awf adopts three fixed, non-configurable byte bounds: the direct default guide render remains at or below 8 KiB, this repository's self-hosted guide remains at or below 10 KiB, and ordinary aggregate `awf check` emits a structured advisory when an adopter's deterministic expected managed `AGENTS.md` exceeds 12 KiB. The advisory reports observed and allowed bytes, does not change a warning-only zero exit, includes adopter overrides and data, and is absent when the agents document is local because awf neither renders nor owns it.
6. `decision: simple-budget-observation` Guide size is observed on the already-produced expected output and the adopter warning is attached only to aggregate `CheckReport.Notes`, excluding initialization, direct drift-only checking, and other non-aggregate advisory consumers. awf does not add configurable thresholds, section or provenance attribution, generic document metrics, a new diagnostics framework, or compatibility plumbing for the removed roster; those mechanisms would cost more conceptual surface than this bounded prose signal warrants.

## State changes

- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/guide-and-doc-templates:working-memory-single-home`
- update `rendering/workflow-skill-templates:deliberate-subagent-model-selection`
- add `rendering/guide-and-doc-templates:agent-guide-size-budgets`
- add `rendering/sync-and-drift:agent-guide-size-advisory`

## Consequences

- Every adopter stops paying for a second skill catalog. Adding, removing, or editing a skill no longer changes the neutral guide merely to mirror frontmatter already rendered for each target.
- An agent must rely on its harness's native skill discovery. There is intentionally no fallback for a consumer that reads `AGENTS.md` without supporting the project's selected target artifacts. Pi discovery is exercised locally; Claude discovery remains part of the external harness contract.
- The guide ceases to be a standalone workflow or protocol summary. Necessary detail remains reachable through native skills and the document map, with one authoritative home instead of synchronized copies.
- The 8 KiB and 10 KiB bounds force substantial redistribution rather than roster deletion alone. In particular, this repository must compress mechanism-heavy invariants and local overrides while preserving every genuinely cross-cutting rule and moving any guide-unique fact before deletion.
- Adopters receive a visible 12 KiB warning without a failed check. Intentional larger guides remain possible, but the fixed warning cannot be disabled or raised and therefore continues to communicate awf's authoring posture.
- Measuring deterministic expected output makes the advisory stable across missing, stale, or hand-edited resident files and keeps ordinary drift reporting independent. A locally owned guide is outside this contract.
- Existing roster derivation and completeness proofs are retired rather than preserved as unused seams. Structural proofs instead require terse native-skill routing, coherent degraded rendering, canonical pointers, the three bounds, and absence of evicted prose classes.
- ADR-0157 remains unchanged history. This decision revises the current claims that encoded its roster and guide-content choices; no predecessor status changes.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep names and triggers but remove purpose and relationships | Native skill frontmatter still owns both identity and selection description, so even a smaller table remains duplicate catalog state. |
| Delete only the roster and add a warning | This repository would remain far above the self-hosted bound because working-memory, invariant, setup, command, and model-selection detail has also accumulated outside its canonical homes. |
| Attribute bytes to base, adopter data, overrides, and individual sections | Precise attribution requires renderer provenance and a new diagnostic model for a problem better solved by simple ownership rules and one total-size signal. |
| Make budgets configurable or blocking | Configuration weakens the shipped opinion, while a blocking adopter check mistakes contextual prose size for correctness. |
| Rely only on prose review | ADR-0157 already established a terse-router intent and the guide regrew, demonstrating that an unmeasured standard is insufficient. |

## Status history

- 2026-08-06: Proposed
- 2026-08-06: Implementing; content-sha256: 979a8685564b1e3db06e61efdb21090ea5341744e13cf676a7ad6e8b1c8b030b
- 2026-08-06: Applied; operations: update `rendering/guide-and-doc-templates:guide-entry-point-routing`, update `rendering/workflow-skill-templates:deliberate-subagent-model-selection`
- 2026-08-06: Applied; operations: update `rendering/guide-and-doc-templates:working-memory-single-home`, add `rendering/guide-and-doc-templates:agent-guide-size-budgets`
- 2026-08-06: Applied; operations: add `rendering/sync-and-drift:agent-guide-size-advisory`
