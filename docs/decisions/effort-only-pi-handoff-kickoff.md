---
format: current-state-v4
slug: effort-only-pi-handoff-kickoff
status: Proposed
date: 2026-08-13
---
# ADR-effort-only-pi-handoff-kickoff: Effort-only Pi handoff kickoff

## Context

Pi handoff guidance currently tells the outgoing agent to put checkpoint-reading and handoff-log
procedures into the replacement kickoff. Agents can also add phase-local restrictions such as
"continue phase 4" or "do not start phase 5." Those instructions duplicate the effort's durable
memory and workflow authority, and a replacement agent can treat them as a stopping boundary after
the named phase instead of continuing the effort.

The replacement session receives the catalog skills. The Pi-derived `using-effort` skill already
owns the Pi-specific association tool instructions, while orienting and effort-workflow own resume
validation and autonomous lifecycle continuation. Effort memory and repository state carry the
current phase, immediate next action, decisions, observations, plan, and worktree identity.

## Decision

1. `decision: identify-effort-only` An effort-backed Pi session handoff kickoff identifies only the
   effort with `Continue with effort <slug>.` It does not restate phase or task limits, association
   mechanics, checkpoint-reading procedure, handoff-log procedure, or other continuation scope.
   The receiving agent resumes through the applicable skills and the effort's durable authority.

## State changes

- update `rendering/pi-workflows:pi-session-handoff-workflow`

## Consequences

Replacement agents receive one stable continuation intent instead of a transient duplicate of
workflow state. They can continue autonomously across phase boundaries until the effort finishes or
a governed stop condition occurs. Pi-specific association remains in the `using-effort` skill, and
resume validation remains in orienting and effort-workflow.

The kickoff no longer carries a recovery checklist. Correct continuation therefore depends on the
receiving agent following the exposed skill routing and reading the effort's authority, which is
already required for effort resumption.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Include association, memory-read, and handoff-log commands in every kickoff | It duplicates skill-owned procedure and encourages kickoff prose to become a second workflow authority. |
| Name the immediate phase but omit an explicit stopping prohibition | A phase-scoped kickoff can still be interpreted as the complete assignment and stall later progression. |
| Permit arbitrary concise continuation prose | It leaves room for procedural or scope duplication and does not establish one predictable handoff contract. |

## Status history

- 2026-08-13: Proposed
