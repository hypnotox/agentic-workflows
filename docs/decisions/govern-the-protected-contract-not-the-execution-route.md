---
format: current-state-v4
slug: govern-the-protected-contract-not-the-execution-route
status: Proposed
date: 2026-08-19
---
# ADR-govern-the-protected-contract-not-the-execution-route: Govern the protected contract not the execution route


## Context

An external source-level audit of this repository at commit `50bd6412f` found that awf has capable
local autonomy rules but no single high-precedence statement of what the workflow protects and what
an implementation owner may change. Its central finding was governance inversion: mechanisms
introduced to protect correct outcomes had become outcomes that must themselves be protected.

The substance is already present and already correct, but scattered and subordinate. The shared
implementation-autonomy partial requires a correction to preserve "the approved outcome, material
scope, settled durable boundaries, and required verification", and states that an omitted path alone
is not a reason to stop. The shared review spine states that competing clean options inside approved
boundaries are delegated detail rather than a user decision. Neither is positioned as an authority
that lower-level rules answer to, so a cautious agent can read a rule about phase shape, task order,
path inventory, or artifact transaction as outranking the requested outcome and a clean design.

One rule contradicts that substance outright. Explicit outline approval is currently required before
every hand-authored production-code mutation, including a mechanical production refactor and a test
that prepares a production change. The trigger is the act of touching production code, not the
presence of an unresolved decision, so a fully specified routine change with no open question still
stops for a ceremonial approval. That rule is carried in the workflow document's chain section and
in a shared approval partial, and is backed by two current-state claims.

The two changes are recorded together because separating them would leave the doctrine standing
beside a rule that contradicts it.

## Decision

1. `decision: protected-contract-governs` The workflow governs a change's protected contract. The
   protected contract is the requested outcome, the explicitly settled durable choices, the material
   scope, the externally observable behaviour, the compatibility and safety constraints, the
   applicable active project rules, the required verification strength, and the prohibited
   shortcuts.

2. `decision: route-is-implementation-detail` Everything outside the protected contract that a
   settled decision has not explicitly made load-bearing is the execution route, which an
   implementation owner may choose and revise while the protected contract holds: phase and task
   boundaries, their order, local names, file and symbol inventories, helper allocation, execution
   mode, exact command sequence, commit decomposition, and non-load-bearing mechanism choice.

3. `decision: doctrine-single-home` The doctrine has one semantic home that every workflow surface
   consumes by reference. No surface restates or redefines it, so no surface can drift from it.

4. `decision: material-decision-approval` A user approval boundary is triggered by an unresolved
   material decision, never by the act of mutating production code. A material decision exists when
   the requested outcome is materially ambiguous, when viable approaches carry meaningfully
   different durable consequences, when externally observable behaviour or compatibility or safety
   or material scope would change, when repository authority contradicts the request, when a
   required verification oracle would have to be weakened, when an irreversible or destructive
   action is not already authorized, or when the clean implementation exposes a separate
   load-bearing decision. Routine implementation detail creates no approval boundary.

## State changes

- add `rendering/workflow-skill-templates:protected-contract-over-route`
- update `rendering/workflow-skill-templates:independent-workflow-escalation`
- update `rendering/workflow-skill-templates:mandatory-approval-boundaries`

## Consequences

A routine, fully specified production change now has a complete legal path with no approval stop,
which removes the most frequent source of ceremony without removing any protection. Brainstorming
remains the sole owner of the approval interaction and still fires for every genuine material
decision, so the boundary moves from an artifact class to an actual decision.

The doctrine gives lower-level rules something to answer to. A rule that constrains only the route
is now visibly subordinate, which is what makes later corrections to plan authority, review lenses,
and verification-oracle rules ordinary rather than contested. Those corrections remain separate
decisions; this record does not make them.

Nothing here weakens generated-source ownership, drift detection, path confinement, worktree
ownership, destructive-operation safety, compatibility inside the declared support window, required
verification, or current-state authority. The protected contract names each of them, so a change
that would weaken one is a protected-contract change and stops.

The delegated-owner contract is unchanged and now reads as a consequence of the doctrine rather than
a separate rule: a delegate consumes the parent-supplied protected contract, never recreates the
approval interaction, and stops to report when that contract is absent or must change.

The accepted risk is that removing a mandatory stop makes an agent's own judgment of materiality
load-bearing. A misjudgement now proceeds where it previously paused. The mitigation is that the
material-decision triggers are enumerated rather than left to taste, and that every existing stop
condition for authority conflict, safety, unreachable verification, and unresolved design forks
remains in force.

Both governance footprints carry the same doctrine, and both supported runtimes render the same
semantics; the doctrine describes no runtime-specific protocol.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Record the doctrine and leave the universal approval gate for a later decision | Ships a doctrine standing beside a rule that contradicts it, leaving which one governs genuinely ambiguous. |
| Express the doctrine as a workflow profile, rigor mode, or depth knob | awf ships one workflow with no profiles, routers, or runtime policy knobs; a mode would make behaviour harder to reason about, not easier. |
| Restate the doctrine in each consuming skill | Duplicated policy in independently changeable places is the divergence the doctrine exists to prevent. |
| Keep the approval trigger and narrow it by carving out exceptions per work type | Exception lists grow with every new work shape and never state the actual rule, which is that an unresolved decision is what warrants asking. |

## Status history

- 2026-08-19: Proposed
