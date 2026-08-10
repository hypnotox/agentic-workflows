---
format: current-state-v4
slug: verification-checkout-for-implementation-commit-policy
status: Proposed
date: 2026-08-10
---
# ADR-0260: Verification checkout for implementation commit policy


## Context

Pi deliberately keeps an effort-associated parent session at the repository root while exposing an
optional managed-worktree path for explicit operations. The implementation subagent process also
starts at the repository root. This preserves one stable runtime and leaves the parent able to
integrate work, but a child brief may direct file and Git operations into an existing managed
worktree.

The implementation tool's commit policy does not model that distinction. It snapshots `HEAD` and
status only in the repository-root checkout before and after the child runs. A commit-capable child
can therefore create and report the required commit in the managed worktree while the monitor sees
an unchanged root `HEAD`, marks the successful call failed, and appends the stopped-inventory demand.
The parent must inspect the intended worktree to distinguish that false finding from a real missing
commit.

Changing either Pi process's working directory would couple postcondition verification to runtime
routing and contradict the root-bound effort model. Inferring a path from effort association would
also turn advisory process metadata into routing authority. The postcondition instead needs its own
explicit checkout identity, separate from execution location and from the child's prose report.
This decision addresses commit verification only; preventing an ordinary mutation from targeting
the wrong checkout remains a separate roadmap problem.

## Decision

1. `decision: verification-identity` The Pi implementation tool accepts an optional
   `verificationCheckout` string as the invocation-owned identity for commit-policy verification.
   Omission selects the project-root checkout. The field does not change the parent session working
   directory, the spawned child process working directory, role-contract loading, effort
   association, or where a task directs file operations.

2. `decision: registered-worktree-boundary` An explicit verification checkout resolves relative to
   the project root, normalizes Pi's one-leading-`@` path convention, and canonicalizes filesystem
   aliases. It must be a checkout root whose Git common-directory identity equals the project
   root's, which admits the project root and its live registered linked worktrees while excluding
   subdirectories, unrelated clones, stale registrations, and non-Git paths. Empty, missing, and
   mismatched identities refuse before child dispatch with an actionable diagnostic. The extension
   validates only the selected checkout through Git identity queries; it does not parse or
   reimplement repository worktree topology, whose shared authored implementation remains in
   `internal/git`.

3. `decision: checkout-scoped-policy` The existing before-and-after Git snapshot and both commit
   permission directions apply to the resolved verification checkout. Structured result details and
   policy diagnostics name that checkout. An unchanged-HEAD owner failure explains how to retry with
   `verificationCheckout` when work occurred in another registered worktree, so the monitor provides
   complete recovery without trusting a child-reported branch, hash, or path.

4. `decision: explicit-caller-selection` Parent-facing Pi workflow guidance selects
   `verificationCheckout` whenever implementation is intentionally directed to a managed worktree
   and omits it for ordinary root work. Actual mutation paths remain explicit in the child task. The
   cross-runtime implementer role contract gains no Pi-only checkout metadata or routing duty. Every
   affected template remains coherent under empty variables and emits no unresolved or no-value
   token.

## State changes

- update `rendering/pi-runtime:pi-implementation-state-boundary`
- update `rendering/pi-workflows:pi-structured-exploration-contract`
- update `rendering/pi-workflows:pi-implement-role-artifact`
- update `rendering/workflow-skill-templates:phase-transaction-ownership`

## Consequences

A managed-worktree commit can satisfy the existing commit-capable-owner postcondition without moving
either Pi process away from the repository root. The same selected identity detects a forbidden
commit, and invalid explicit paths fail before an implementation child spends tokens or mutates a
tree. The public tool schema grows by one optional field, while callers that implement in the root
retain their existing invocation.

Checkout selection remains explicit. A caller that directs work elsewhere but omits the field still
verifies the root, but the resulting diagnostic now gives the complete retry path. The extension
must canonicalize the selected path and compare its Git checkout-root and common-directory identity
before explicit dispatch, adding Git and filesystem validation to the parent-side precondition
without adding a second topology parser. This decision does not bind ordinary mutations to an
effort worktree, prove the child's reported commit hash, or strengthen the existing clean-tree and
commit-parent policy.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Change the parent session or child process working directory | Conflates execution routing with postcondition identity and breaks the deliberate root-bound runtime model. |
| Parse the child's reported worktree, branch, or commit | Makes enforcement depend on untrusted prose and cannot distinguish a mistaken report from repository state. |
| Infer the checkout from attached effort metadata | Effort association is advisory and explicitly not routing authority; implementation can also run without an effort. |
| Accept any Git checkout path | Permits a typo or unrelated clone to become the monitored authority instead of constraining verification to this repository's Git identity. |
| Add a TypeScript parser for `git worktree list` | Duplicates the repository topology contract already owned by `internal/git`; single-checkout Git identity probes answer this extension's narrower question. |
| Keep root-only monitoring and document manual inspection | Preserves the recurring contradictory result and leaves recovery outside the tool that produced the false finding. |

## Status history

- 2026-08-10: Proposed
