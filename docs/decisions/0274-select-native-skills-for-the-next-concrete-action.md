---
format: current-state-v4
slug: select-native-skills-for-the-next-concrete-action
status: Implementing
date: 2026-08-13
---
# ADR-0274: Select Native Skills for the Next Concrete Action


## Context

Native skill descriptions are exposed before their bodies and serve as routing metadata. The existing guide prohibited preloading likely follow-up skills, but agents could still interpret a task's likely workflow chain as concurrently applicable and load orientation, generated-tree, and documentation bodies before beginning any of those actions. This spends context and weakens the intended progressive disclosure boundary. The affected templates must retain missingkey=zero behavior and token-free output when interpolation values are empty.

## Decision

1. `decision: route-by-next-action` Select native skill bodies against the next concrete action, not the full anticipated workflow chain.
2. `decision: defer-later-owners` A possible later edit, render, documentation update, review, or commit does not justify loading its owning skill before that action begins.
3. `decision: constrain-multiple-loads` Load multiple skill bodies only when each independently governs the same next action before another routing decision can occur.
4. `decision: expose-timing-boundaries` The orienting, using-awf, and writing-docs descriptions state when their owned work begins and exclude the approved investigation, planning, or known-file inspection that precedes it.

## State changes

- update `rendering/guide-and-doc-templates:guide-entry-point-routing`
- update `rendering/workflow-skill-templates:orienting-single-home`
- update `rendering/workflow-skill-templates:using-awf-transaction-home`
- update `rendering/workflow-skill-templates:writing-docs-delegation`

## Consequences

Agents receive a concrete routing test before loading any body and defer likely follow-up owners until their work actually begins. Legitimately concurrent governance remains possible, but anticipation alone no longer qualifies. Agents reassess routing between concrete actions and may load related bodies serially, trading some convenience or latency for lower context consumption. The three descriptions become slightly more explicit because routing timing and exclusions must be visible without loading the body.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Strengthen only the root guide | Commonly over-selected descriptions would still invite premature loading. |
| Change only skill descriptions | Each description would lack one consistent project-wide selection rule. |

## Status history

- 2026-08-13: Proposed
- 2026-08-13: Accepted; content-sha256: 559f0fc3e2f3e2a5a5c277319ee29f375ea33d76cc6f874eb2b5a53d2f9b202d
- 2026-08-13: Implementing; content-sha256: 559f0fc3e2f3e2a5a5c277319ee29f375ea33d76cc6f874eb2b5a53d2f9b202d
- 2026-08-13: Applied; operations: update `rendering/guide-and-doc-templates:guide-entry-point-routing`, update `rendering/workflow-skill-templates:orienting-single-home`, update `rendering/workflow-skill-templates:using-awf-transaction-home`, update `rendering/workflow-skill-templates:writing-docs-delegation`
