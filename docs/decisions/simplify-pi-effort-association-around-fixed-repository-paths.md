---
format: current-state-v4
slug: simplify-pi-effort-association-around-fixed-repository-paths
status: Proposed
date: 2026-08-03
---
# ADR-simplify-pi-effort-association-around-fixed-repository-paths: Simplify Pi effort association around fixed repository paths

## Context

ADR-0218 introduced an explicit Pi effort association, but coupled that association to live
checkout replacement. Its `using_effort` tool resolves either a managed or receiving checkout,
terminates the active agent run, queues a private command, calls Pi's command-only `changeCwd`,
and transfers immutable association state through a process-global coordinator into the
replacement extension instance. The activity protocol consequently records absolute CWD,
receiving-checkout CWD, and checkout role, exposes resolve and checkout operations, and reports
a CWD mutation axis that the awf binary does not itself own.

That coupling makes a supportive identity operation cross a full runtime replacement boundary.
The source extension is invalidated during replacement, the destination extension must recover
transient coordinator state, and failures split across pre-teardown, replacement, commit, and
recovery phases. The same mechanism writes warnings into Pi's local status bar and notification
surface even though Remote Pi's temporary effort-slug name already provides the desired visible
attachment indicator. Repairing autonomous continuation after every replacement outcome would
add another delivery protocol without removing the underlying source of failure.

The repository structure already supplies the paths an associated agent needs. Effort memory is
always `.awf/efforts/<slug>/memory.md`, and an optional managed worktree, when present, is always
`.awf/worktrees/<slug>`. Pi is intentionally run from the repository primary root. The extension
can therefore give the model fixed relative paths without selecting, validating, or moving to a
checkout. Directory presence is sufficient to decide whether to mention the optional managed
worktree; Git registration and branch validity remain the concern of ordinary worktree commands,
not session association.

Activity remains advisory process coordination, not tracked authority, topology, a lock, or
permission. Its useful state is only the current owner and attachment/heartbeat time. Existing
protocol-1 activity files may contain obsolete checkout fields or malformed advisory data, but an
explicit new attachment is a natural recovery boundary. Safe resident handling and conditional
replacement still matter because symlink, size, and concurrent-writer boundaries are real even
when checkout topology is not validated.

This changes terminal ADR-0218 and ADR-0219 forward through current-state claims. Their history is
not rewritten.

## Decision

1. `decision: associate-without-checkout-replacement` Make Pi effort association independent of
   checkout selection and runtime CWD mutation. Pi remains in the repository primary root for the
   association lifecycle. `using_effort` never resolves a receiving or managed destination, calls
   `changeCwd`, creates a conversation, queues a command, terminates an agent run, or transfers
   state between extension instances. Work against an effort's managed worktree uses the fixed
   relative path supplied in model context and ordinary file or command-local path selection.

2. `decision: activity-protocol-v2` Replace the extension-facing activity protocol with JSON-only
   protocol v2:

   - `awf effort activity attach <slug> --owner <uuid> --json`
   - `awf effort activity heartbeat <slug> --owner <uuid> --json`
   - `awf effort activity detach <slug> --owner <uuid> --json`

   Remove `resolve` and `checkout`, their destination/CWD/role/receiving-checkout flags, checkout
   facts, prior checkout facts, the `ready`, `checkout-updated`, and `repository-mismatch`
   conditions, and the CWD mutation axis. A protocol-v2 activity fact contains exactly
   `schemaVersion: 2`, owner, `attachedAt`, and `heartbeatAt`. A protocol-v2 reply carries
   `schemaVersion: 2`, a stable condition, and an exact condition-specific fact set. `attached`,
   `taken-over`, and `heartbeat` require effort, validated memory, and activity facts and forbid an
   outcome. `detached` carries no effort, memory, activity, or outcome fact. Each refusal condition
   requires exactly one outcome and forbids effort, memory, and activity facts. Handled refusal
   conditions are `not-owner`, `missing`, `invalid-memory`, and `unsafe-resident`; no reply field is
   optional except the outcome cause described below. A refusal outcome carries its matching
   condition, `changedActivity`, `changedMemory`, ordered executable next actions, and a cause
   exactly when a mechanism call failed. Malformed invocation and failures that observe no managed
   state retain the existing nonzero, empty-stdout, bounded-stderr CLI convention.

3. `decision: recovery-attachment` Make explicit protocol-v2 attach the recovery boundary for
   advisory activity. Under the per-effort activity lock, attach reads only the bounded regular
   file identity needed for safe conditional publication. If any safe activity file exists,
   regardless of its protocol version or decodability, attach conditionally replaces it with the
   new v2 owner/timestamps and reports `taken-over`; it does not preserve or interpret obsolete
   checkout details. A symlink, non-regular file, oversized file, storage fault, or identity race
   remains an `unsafe-resident` refusal. Heartbeat and detach accept only a valid v2 record owned
   by their supplied owner. Activity remains optional and advisory, and show, list, memory,
   worktree, integrate, finish, render, check, and unrelated commands do not acquire an activity
   validation gate.

4. `decision: direct-using-effort-tool` Narrow the generated Pi tool to two exclusive public
   forms: attachment with `{ effort: "<canonical-slug>" }`, and detachment with
   `{ detach: true }`. The closed schema contains only optional `effort` and `detach` properties;
   runtime validation requires exactly one form. Execute attachment and detachment directly from
   the tool context through the generated binary client, propagate cancellation, and serialize
   operations through one process-local promise chain. Tool results are non-terminating. A switch
   first detaches the old claim; a failed detach preserves the old association, while a failed new
   attach after successful detach leaves the session detached without uncertain rollback. Repeat
   attachment directly replaces the claim. Validation failures throw before binary invocation;
   expected protocol refusals return bounded actionable model-visible results.

5. `decision: fixed-relative-effort-context` While attached, append one hidden, non-persisted
   custom message in every Pi `context` event. It reports
   `[awf effort] active=<slug> memory=.awf/efforts/<slug>/memory.md` and appends
   `managedWorktree=.awf/worktrees/<slug>` only when a direct directory stat from the repository
   root says that fixed path is a directory. Absence or inspection failure omits the worktree
   field and never blocks association. Refresh validated memory metadata and optional directory
   presence after heartbeat. Inject nothing while detached or after restart. Do not duplicate
   Phase or Next in the transient line; the owned memory remains their workflow authority.

6. `decision: advisory-process-lifecycle` Retain one process-local association with an ephemeral
   owner UUID, immutable snapshots, owner-checked heartbeat after completed turns, explicit
   detach, shutdown detach attempt, restart-detached behavior, and ownership-loss cleanup.
   Heartbeat `not-owner` or `missing` clears local association and Remote publication. Other
   heartbeat failures preserve advisory attachment while clearing unverified published memory and
   managed-worktree facts. Remote publication and shutdown cleanup failures remain silent and do
   not change tool success, authority, or workflow progress.

7. `decision: remote-name-without-local-tui` Retain Remote Pi's complete `awf` metadata
   replacement, capability negotiation, replay, and temporary effort-slug name override. Publish
   effort identity, validated memory metadata when available, and the activity heartbeat, but no
   CWD or checkout role. Detach and ownership loss clear metadata and the name override. Remove all
   direct `ctx.ui` status, notification, and cleanup calls, and remove assigned-name diagnostics
   that existed only to drive local presentation. The Remote Pi name override remains the visible
   attachment indicator; metadata and names remain advisory and never become routing authority or
   a lock.

8. `decision: fixed-layout-dependency-boundary` Keep responsibility cohesive. The Go effort
   package owns safe activity residents, memory validation, owner/race semantics, and protocol-v2
   outcomes. Command code owns the three-operation grammar and JSON transport. The generated
   client strictly decodes v2 and invokes `./awf`; it does not write residents. The generated Pi
   extension owns process-local association, direct directory presence, transient context, and
   optional Remote Pi translation. The target-neutral effort workflow continues to direct runtimes
   without native association context into the exact managed worktree, while permitting a runtime
   that supplies explicit effort paths to remain at the repository root and target that worktree
   by path. It names no Pi-only tool. The Pi companion states that Pi starts at the repository root
   and uses the supplied fixed paths. Neither layer infers an effort, inspects Git topology, or
   validates unsupported checkout layouts.

9. `decision: compatibility-and-verification` Keep on-disk v1 compatibility one-way and bounded:
   new attach recovers by replacing a safe v1 file, while older binaries need not understand a v2
   activity resident or reply. No upgrade migration is required. Update the complete behavior
   surface, including CLI and effort authority, Pi runtime and workflow claims, the companion
   skill, README, architecture, working-with-awf, testing guidance, generated outputs, and
   changelog. Tests prove the exact v2 grammar/envelopes and removed checkout surface, recovery
   attachment, owner/race/safe-file behavior, direct non-terminating Pi state machine, cancellation,
   fixed relative transient context with optional worktree presence, restart and heartbeat
   behavior, absence of queued/CWD/TUI machinery, retained Remote Pi metadata/name replay, target
   render/prune behavior, strict TypeScript coverage, and the complete repository gate. Render
   tests exercise every changed template with empty optional values and require coherent generic
   output with no `<no value>` or other unresolved-value token. The direct Implemented transaction
   updates every behavior-stating authoring artifact, `AGENTS.md`, all declared claims with
   preserved provenance, adopter-facing changelog text, and generated outputs in the same commit;
   `./x render` regenerates `docs/decisions/INDEX.md` and the lock before that transaction is
   staged.

## State changes

- update `tooling/cli:effort-command-contract`
- update `tooling/effort-management:effort-record-authority`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-workflows:pi-effort-session-association`
- update `rendering/pi-workflows:using-effort-skill`
- update `rendering/workflow-skill-templates:unified-effort-workflow-coverage`
- update `rendering/workflow-skill-templates:effort-workflow`

## Consequences

Effort association becomes a small direct operation rather than a runtime-navigation protocol.
The model receives the two fixed relative paths it needs on every call, including tool follow-ups,
without persisting duplicate workflow state. Efforts created with `--no-worktree` remain useful:
their context simply omits `managedWorktree`. The existing Remote Pi name override continues to
show which effort a session uses, while local footer and notification failures disappear.

The activity binary, generated client, and extension lose destination resolution, checkout
mutation, absolute path facts, role state, command queueing, global transfer coordination,
replacement timeouts, `changeCwd` capability detection, and multi-phase recovery. The smaller
state machine has fewer partial outcomes and no foreign runtime-replacement dependency. Protocol
v2 also aligns outcome ownership with reality: awf reports only activity and memory axes it can
observe.

The design deliberately trusts awf's fixed layout and the operational rule that Pi starts at the
repository root. A Pi process started elsewhere may receive relative paths that are not meaningful;
the extension does not add topology discovery to compensate for an unsupported invocation.
Directory presence does not prove Git registration, cleanliness, branch identity, or usability.
Those properties remain validated by the commands that mutate managed-worktree topology.

Explicit attachment can overwrite malformed but safely bounded advisory activity. This improves
recovery and discards prior owner/timestamp diagnostics, which are no longer presented locally.
Symlink, non-regular, oversized, storage, and concurrent-identity failures remain refused. Older
binaries may reject v2 activity; downgrade compatibility was already outside the activity
contract, while a new binary can recover directly from v1 without a repository migration.

The implementation is cross-cutting despite deleting complexity. A forward ADR is necessary
because eight implemented current-state claims change, and a reviewed plan is necessary to sequence
the protocol, generated runtime, claims, documentation, rendering, and coverage work within the
single application transaction.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Repair autonomous continuation after every `changeCwd` outcome | It adds another delivery protocol while retaining the runtime replacement boundary that causes the failures. |
| Keep a queued post-turn command but remove CWD replacement | Without runtime mutation, termination and command queueing add latency and failure states without providing a safety barrier. |
| Keep protocol v1 and force `changedCwd` to false | It preserves checkout operations and facts that no longer belong to association, and mutates a closed protocol without a clean version boundary. |
| Add primary-checkout validation to receiving resolution | Fixed repository-relative paths make destination resolution unnecessary; another topology check would preserve the over-modeled checkout abstraction. |
| Continue validating Git registration before reporting the worktree | Association only needs a useful optional path. Commands that mutate topology already own the stronger Git checks. |
| Maintain dual v1/v2 activity decoders | Obsolete checkout details have no remaining consumer; explicit safe replacement is a smaller recovery contract. |
| Replace the model tool with only a user command | Agents would lose explicit autonomous attachment to an already-confirmed effort. |
| Remove effort association entirely | The transient paths, advisory metadata, and Remote Pi effort name remain useful once detached from CWD mutation. |
| Keep local footer and notification integration | The Remote Pi name already communicates attachment, and background presentation errors disrupt unrelated work. |

## Status history

- 2026-08-03: Proposed
