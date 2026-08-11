---
format: current-state-v4
slug: restore-autonomous-effort-management
status: Proposed
date: 2026-08-11
---
# ADR-restore-autonomous-effort-management: Restore Autonomous Effort Management


## Context

ADR-0222 introduced mandatory later-turn confirmation of an effort outcome and title before first
creation. ADR-0226 extended that boundary to the explicit short slug. The confirmation protected
immutable identity while agents were creating efforts too early or naming discovery rather than a
settled outcome.

Observed behavior has changed. Effort-creation timing is now generally sound and can instead be late
or omitted when apparently easy work takes long enough on the primary checkout to block integration.
The mandatory round trip no longer provides proportionate protection: it delays the managed
worktree exactly when continuity has already become useful, and it can discourage creating the
effort at all.

The useful parts of the existing model remain. Discovery is effort-free until durable continuity
materially helps. One effort-workflow owns creation through finish, explicit titles and short slugs
remain immutable, managed worktrees isolate implementation, and repository authority outranks
memory. The policy gap is conversational rather than mechanical. The CLI already accepts the
explicit identity and cannot observe whether natural-language authorization preceded its command.

A session may also discover a distinct outcome while another effort is active. Silently reusing or
abandoning the old effort would lose continuity, but requiring a user round trip for every switch
would recreate the ceremony this decision removes. The transition instead needs a deliberate,
reasoned disposition of the old effort. ADR-0222 and ADR-0226 are terminal history, so this record
changes their active claims without editing either record.

## Decision

1. `decision: create-efforts-autonomously` When independent judgment determines that durable
   continuity materially helps for a concrete outcome with no owning effort, the effort-workflow
   autonomously chooses a faithful outcome title and canonical short slug, creates the effort, reports
   the allocated identity, and continues in its managed worktree. First creation has no mandatory
   proposal, user-confirmation, later-response, or reconfirmation boundary.

2. `decision: preserve-continuity-threshold` Keep discovery effort-free and preserve the existing
   independent continuity trigger. Reaching a workflow stage, changing code, or producing an artifact
   does not itself require an effort. The effort-workflow remains the sole owner of creation,
   checkpoints, integration, topology removal, retrospective routing, and archival finish.

3. `decision: switch-deliberately` When a session already owns a different unfinished effort, it may
   switch autonomously only after reasoning about why the new effort should own the work and whether
   the old effort should be kept for later continuation or discontinued. A kept effort receives a
   resumable checkpoint before the switch. A discontinued effort transfers necessary context to the
   new owner before its old managed topology is explicitly discarded and its resident is finished
   into the ordinary archive.

4. `decision: reuse-existing-lifecycle` Add no abandoned lifecycle state, effort schema, CLI command,
   conversational authorization state, or runtime policy knob. A discontinued effort uses the same
   topology-removal and archival-finish mechanics as any other finished effort; the reasoned
   disposition and context transfer are workflow responsibilities.

5. `decision: prove-autonomous-boundary` Deterministic workflow projections must positively prove
   autonomous explicit-slug creation, identity reporting, managed-worktree continuation, and
   deliberate active-effort disposition while rejecting obsolete ask, wait, later-response, and
   reconfirmation instructions. Existing sole-owner, immutable-identity, effort-free discovery, and
   active-signature protections remain.

## State changes

- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:effort-workflow`

## Consequences

Continuity ownership can begin as soon as the agent judges it materially useful, so work is less
likely to remain on the primary checkout long enough to block integration. Precise immutable titles
and slugs remain required, but choosing them becomes an autonomous responsibility whose quality is
reviewed on its merits rather than authorized by ceremony.

The user loses a routine opportunity to correct identity before allocation. Faithful naming,
effort-free discovery, explicit reporting, and immutable ownership discipline mitigate that risk.
If the chosen identity is materially wrong, correction uses an intentional new effort and ordinary
archival cleanup rather than silent rename or reuse.

Switching between efforts becomes possible without a mandatory check-in, including when priorities
change mid-conversation. The agent must make the transition legible: preserve a resumable old effort
or transfer its necessary context and archive it. This adds judgment to lifecycle management but does
not add persistent state or destructive automation to the CLI.

Failure recovery no longer depends on retained confirmation evidence. Ordinary diagnosis and
authority-preserving retry rules apply, while unresolved safety or correctness blockers still stop
progression under the general workflow contract.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Retain mandatory three-field confirmation | Observed creation timing is now sound, while the extra round trip can delay or suppress useful isolation and continuity. |
| Make detached creation autonomous but prohibit switching from an active effort | It avoids one transition risk but cannot handle deliberate reprioritization or replacement of an obsolete outcome without restoring user ceremony. |
| Add an `abandon` command or lifecycle state | Ordinary topology removal and archival finish already represent a discontinued effort safely; new persistent machinery would duplicate that lifecycle. |

## Status history

- 2026-08-11: Proposed
