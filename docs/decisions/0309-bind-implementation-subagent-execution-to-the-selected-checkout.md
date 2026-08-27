---
format: current-state-v4
slug: bind-implementation-subagent-execution-to-the-selected-checkout
status: Implemented
date: 2026-08-26
---
# ADR-0309: Bind Implementation Subagent Execution to the Selected Checkout


## Context

Effort-backed implementation normally occurs in a managed linked worktree while the parent Pi session stays rooted at the primary checkout. That stable parent root is required by extensions, native skills, and other session integrations. Changing it for an effort would break runtime assumptions.

The implementation profile already accepts an optional `verificationCheckout`. Awf validates that identity as an exact live checkout and snapshots it before and after child execution, but prepares the child with the project root as its working directory. Verification can therefore inspect the managed worktree while the child mutates the primary checkout. The name and postcondition check do not route execution.

Pi-tools already accepts a profile-supplied child working directory beneath the parent session root. Managed effort worktrees live beneath that root, so the profile can align child execution with the validated checkout without changing the parent session. The adopter pi-tools runtime remains revision-independent and available through the protocol handshake.

Parent-session commands and generic file operations remain a separate boundary. Effort association supplies the managed-worktree path but is advisory and does not authenticate or route an ordinary command. The existing wrong-checkout issue therefore requires prominent explicit-path mitigation even after child execution is corrected.

## Decision

1. `decision: selected-checkout-governs-child-execution` When an implementation invocation supplies a verification checkout, the same validated checkout is both the child base working directory and the before-and-after commit-policy snapshot identity. This aligns relative execution with verification; it does not confine deliberately targeted paths.
2. `decision: omitted-checkout-retains-root-default` An invocation that omits the checkout continues to execute and verify at the project root. The profile never infers an effort or worktree from activity, branch names, modification time, or repository topology.
3. `decision: effort-workflows-supply-managed-checkout` Effort-backed pre-integration implementation owners supply the managed-worktree checkout explicitly. Lifecycle operations that intentionally belong in the primary checkout retain their governed primary-root transition.
4. `decision: parent-session-root-remains-stable` Awf does not change or globally reroute the main Pi session working directory. Parent-session pre-integration mutation uses the supplied managed-worktree path explicitly, and adopter-facing guidance states this limitation and mitigation while it remains open.

## State changes

- update `rendering/pi-runtime:pi-implementation-state-boundary`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- update `rendering/pi-workflows:pi-implement-role-artifact`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`

## Consequences

Implementation children with an explicit checkout begin relative execution in the same checkout whose commit policy is verified. The existing checkout validation becomes the single identity for base CWD and postcondition evidence, without claiming confinement against deliberately targeted paths.

This changes existing explicit-checkout calls whose tasks rely on root-relative execution while verifying another checkout. Those callers must revise their invocation or task paths; omitted-checkout calls retain their current project-root behavior.

The parent session remains stable, and no effort identity becomes hidden routing authority. Callers must continue to name the managed worktree. Awf preparation explicitly refuses a checkout outside the canonical parent-session checkout or one that is not live and accessible before dispatch. This preserves the descendant boundary without attributing refusal to pi-tools runtime confinement.

The parent-session half of the wrong-checkout issue is mitigated, not closed. Documentation must not claim session-wide confinement, and the known issue remains until its existing completion criteria are satisfied or deliberately revised by a later decision.

Pi-tools can continue independent development and patching. Awf tests its profile contract and pinned test-support dependency, but does not claim revision-reproducible adopter execution. Every affected rendered template preserves missingkey-zero behavior and emits no no-value token when optional values are empty.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Change the main Pi session CWD to the effort worktree | It breaks extensions, skills, and other integrations that rely on a stable session root. |
| Keep execution at root and use the managed worktree only for verification | It can approve commit policy for a checkout the child did not mutate. |
| Infer the active effort inside the implementation profile | Advisory activity is not routing authority, and implicit selection would obscure the caller's mutation target. |
| Add a separate execution-checkout field | Two independently supplied checkout identities could diverge; requiring equality would create redundant public inputs. |
| Claim complete wrong-checkout closure from child routing | Parent-session shell and file operations remain explicitly path-targeted rather than confined. |

## Status history

- 2026-08-26: Proposed
- 2026-08-26: Accepted; content-sha256: bf244f2900e3565356a4ab9e493ff63f6751f6351c0cc1e69dc80f4c419264a4
- 2026-08-26: Implementing; content-sha256: bf244f2900e3565356a4ab9e493ff63f6751f6351c0cc1e69dc80f4c419264a4
- 2026-08-26: Applied; operations: update `rendering/pi-runtime:pi-implementation-state-boundary`, update `rendering/pi-workflows:pi-structured-exploration-contract`, update `rendering/pi-workflows:pi-implement-role-artifact`, update `rendering/workflow-skill-templates:phase-transaction-ownership`
- 2026-08-27: Implemented; content-sha256: bf244f2900e3565356a4ab9e493ff63f6751f6351c0cc1e69dc80f4c419264a4
