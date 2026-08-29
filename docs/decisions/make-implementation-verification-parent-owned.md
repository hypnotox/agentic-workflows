---
format: current-state-v4
slug: make-implementation-verification-parent-owned
status: Proposed
date: 2026-08-29
---
# ADR-make-implementation-verification-parent-owned: Make implementation verification parent-owned


## Context

ADR-0166 assigns a subagent-driven plan phase to a commit-capable implementation child from a clean
baseline through staging, the commit gate, and the closing commit. ADR-0177 projects that authority
into a two-mode implementer contract and the Pi implementation profile exposes `allowCommits` to
select it. Inline execution instead keeps transaction ownership with the parent.

That split duplicates assurance and fragments responsibility. A child may run broad checks to reach
its closing commit, after which the parent must still inventory the result, settle review findings,
and own terminal assurance. Commit authority also makes the child's wired pre-commit gate
unavoidable even when the parent is the intended verification owner. Helper returns carry useful
prose but no complete state identity with which the parent can decide whether focused evidence
remains fresh.

The approved boundary keeps delegated implementation but makes verification and transaction
ownership uniform. Children still need focused feedback while editing, and the parent needs enough
structured evidence to avoid blindly repeating it. The Pi profile can enforce unchanged HEAD at its
selected checkout, but its unrestricted child shell exposes no awf-owned command filter. The design
therefore must not claim mechanical detection of gate commands that the runtime cannot provide.

This decision preserves sequential implementation dispatch, exclusive parent-batch execution,
managed-checkout selection, focused tests, report-only review, the fast and full gate semantics, and
unrelated-defect routing. It does not introduce host-wide verification serialization,
external-prerequisite preflights, or serialized integration windows. The separately approved
semantic-owner assurance decomposition is a successor decision. Approved batching of independent
cheap operations remains implementation-route guidance for the plan rather than a durable
transaction-ownership commitment.

## Decision

1. `decision: parent-owned-transaction` The parent owns every implementation transaction across
   inline and delegated execution: integration, explicit staging, the staged check, every commit, the
   fast commit gate, and terminal exhaustive verification. An implementation child never stages,
   commits, or runs either gate; it runs only focused tests and checks relevant to its bounded task
   and reports every command and result.

2. `decision: one-child-authority` Replace the commit-capable phase-owner and commit-disabled helper
   modes with one commit-disabled implementation-child authority. `subagent-driven` remains a plan
   execution mode for delegated implementation, not delegated transaction ownership. Remove
   `allowCommits` from the implementation profile because the approved model has no valid
   commit-capable child state; retain selected-checkout validation, sequential dispatch, and
   exclusive parent-batch execution.

3. `decision: focused-evidence-receipt` Every implementation child returns a closed completed or
   stopped receipt that identifies its assigned scope, canonical checkout, starting and ending Git
   and worktree state, changed paths, exact focused commands and actual results, completed and
   remaining work, deviations, separately routed blockers, and applicable generated-output or
   fixture evidence. A completed receipt means the assigned implementation and focused checks
   finished; it does not imply a commit or terminal verification.

4. `decision: parent-validates-freshness` The parent independently inventories the checkout after
   return, rejects writes outside the delegated boundary, confirms unchanged HEAD and the reported
   end state, and reuses focused evidence only while the checkout identity, relevant authority, and
   overlapping paths remain unchanged. Lost or unverifiable evidence and overlapping later mutation
   invalidate the affected receipt. No receipt substitutes for a parent-owned gate.

5. `decision: honest-enforcement-boundary` Pi compares the selected HEAD before and after the child,
   rejects a mismatch, and fails closed when that unchanged-HEAD comparison cannot be established.
   The child no-commit and no-gate rules are enforced through the rendered implementer and dispatch
   contracts with deterministic contract tests, not described as shell-command or intervening-history
   interception. Other runtimes retain the same parent-owned semantic contract through their rendered
   roles and skills. Every affected template renders coherently with empty variables under
   missingkey-zero behavior and emits no unresolved-value token, backed by deterministic tests.

## State changes

- update `rendering/workflow-skill-templates:phase-transaction-ownership`
- update `rendering/workflow-skill-templates:plan-task-detail-modes`
- update `rendering/workflow-skill-templates:maintainable-code-subagent-contract`
- update `rendering/workflow-skill-templates:implementer-role-contract`
- update `rendering/pi-runtime:pi-implementation-state-boundary`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- update `rendering/pi-workflows:pi-implement-role-artifact`

## Consequences

The parent becomes the single owner of mutable integration and verification evidence. Children can
focus on bounded implementation without paying commit-gate cost, and their focused results can be
reused when the parent proves that relevant state stayed fresh. Delegated execution remains useful
without creating a second transaction owner.

Parent work increases: it must integrate every child result, validate the receipt against the real
checkout, stage deliberately, and create the commit. The structured receipt is more detailed than
today's helper report, and a stale or incomplete receipt causes focused checks to be rerun. This cost
is accepted because it replaces blind repetition with explicit evidence rather than weakening any
oracle.

Removing `allowCommits` is a breaking change to the pre-1.0 Pi profile schema. Existing callers that
send it must stop doing so. The runtime can enforce unchanged HEAD but cannot prove that a child did
not invoke a gate through an alias or equivalent shell command; rendered contracts and regression
tests are the narrow truthful enforcement boundary until the runtime exposes a suitable command
policy.

The decision does not serialize independent efforts or agents. Test and fixture isolation remains
the remedy for unsafe concurrent verification.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep commit-capable children and ask them not to run gates | A child commit invokes the wired commit gate and preserves split transaction ownership. |
| Retain `allowCommits` but accept only `false` | A compatibility switch with no valid alternate state preserves obsolete conceptual surface. |
| Let children run gates and reuse those results | The approved verification owner is the parent, and child state may change before integration and review settlement. |
| Add shell-command filtering in awf | The current profile exposes no awf-owned argv interception boundary; claiming one would invent a runtime capability. |
| Add a host-wide verification lock | Independent efforts and agents should remain concurrent; fixture or test isolation should fix contention defects. |
| Add prerequisite preflights or integration serialization | The observed incidents were accidental setup and slow-verification effects, not permanent workflow requirements. |

## Status history

- 2026-08-29: Proposed
