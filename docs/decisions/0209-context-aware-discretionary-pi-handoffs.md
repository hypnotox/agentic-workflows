---
format: current-state-v3
slug: context-aware-discretionary-pi-handoffs
status: Implemented
date: 2026-08-02
---
# ADR-0209: Context-aware discretionary Pi handoffs


## Context

awf currently treats a durable workflow checkpoint and a Pi fresh-session handoff as one
routine sequence. The generated checkpoint partials persist effort memory and then instruct Pi
to invoke `handoff_session`; the extension accepts the effort memory path plus a kickoff and
revalidates the memory file before replacement. This keeps continuation durable, but it also
makes every eligible boundary replace the active session even when the accumulated context is
still relevant to the next phase.

The two operations solve different problems. A checkpoint protects resumability and memory
hygiene. A handoff controls conversational context, model-input cost, and the effects of prior
compactions. Making replacement automatic is not required to keep the checkpoint durable, and
making replacement discretionary must not make checkpointing discretionary in turn.

Pi already exposes the facts needed for informed judgment through public extension APIs.
`ctx.getContextUsage()` reports current tokens against the active model's context window, and
`ctx.sessionManager.getBranch()` exposes compaction entries on the active branch. The extension
`context` event runs before every model call, including tool-follow-up calls, and can add a
non-persisted model-facing message. Pi does not expose the configured auto-compaction threshold
through that context API, so awf cannot honestly present or enforce a distance-to-threshold
policy.

The current handoff extension owns two unrelated concerns. Its single-use request, queued
continuation command, countdown, cancellation, parent-linked replacement, cleanup, and recovery
are session-replacement mechanics. Its repository-root discovery and memory path, ownership,
link, size, UTF-8, effort-header, and stable-identity validation duplicate workflow policy at a
runtime boundary that does not need to understand efforts. Workflow instructions can require a
current durable checkpoint without making the replacement mechanism parse or police it.

The generated Pi target currently governs four extension files: the handoff entrypoint and the
three subagent files. A standalone context-usage extension adds a fifth target output and must
travel through the same output plan, render, drift, cleanup, adopter example, minimum-runtime,
and pinned-runtime test surfaces. It remains separate from both subagent process orchestration
and handoff mechanics so observation, child execution, and session replacement each have one
cohesive owner.

## Decision

1. Keep durable checkpointing mandatory at the existing formal workflow boundaries. A concrete
   non-minimal effort still persists its owned memory before continuation, preserves the
   one-writer and repository-authority rules, and may add a checkpoint at any other safe point
   whose next action is independently resumable. A minimal simple fix still creates no effort
   merely to satisfy a boundary.

2. Make Pi session replacement discretionary after a completed formal phase checkpoint, after
   an approval has been persisted, or after an additional safe resumable checkpoint. At each
   eligible point the agent decides whether to continue in the active session or hand off by
   considering the context facts, compaction history, relevance of accumulated context, and the
   work about to begin. No token percentage, compaction count, phase type, or other fixed
   threshold mandates or forbids a handoff.

3. Keep mandatory approval semantics unchanged. Brainstorming and ADR review still stop for
   explicit approval, and no handoff occurs before that approval. After approval and its next
   action are persisted, handoff becomes one permitted continuation choice rather than the
   default continuation path. Routine clear boundaries continue autonomously in either the
   current or a replacement session; a decision not to hand off is not a user check-in.

4. Record a `## Handoff log` entry only after a fresh-session boundary actually exists.
   Ordinary checkpoints that continue in the active session update phase, next action, time,
   decisions, and observations without fabricating a handoff event. A replacement session appends
   the boundary as its first memory update before substantive continuation; if automatic kickoff
   fails, the prepared recovery prose carries the same instruction for manual continuation.
   Cancellation or a failure that leaves the old session active creates no handoff-log entry.

5. Add a standalone generated Pi extension at
   `.pi/extensions/awf-context-usage/index.ts`, authored from
   `templates/pi/awf-context-usage/index.ts.tmpl`. It owns only transient context-fact
   construction and injection. The handoff extension continues to own main-session replacement,
   and the subagent extension continues to own isolated child processes and routing.

6. The context-usage extension subscribes to Pi's `context` event so it refreshes its facts before
   every model call, including calls after tool results. It appends exactly one non-persisted,
   model-facing context message to the copied request messages and never writes a session message,
   custom entry, resident file, telemetry record, widget, status item, or handoff request.

7. The injected text is one neutral line with the form
   `[session context] 118.2k/272k (43%); compactions=0`. The token count and percentage come from
   `ctx.getContextUsage()`, the denominator is explicitly the active model window rather than an
   auto-compaction threshold, and the compaction count is the number of `compaction` entries on
   `ctx.sessionManager.getBranch()`, never the count from abandoned branches or the whole session
   tree. A finite token or window value below 1,000 renders as its rounded integer; a value from
   1,000 renders in base-1,000 `k` units, and a value from 1,000,000 renders in base-1,000,000 `m`
   units, in both cases with one decimal rounded by JavaScript `toFixed(1)` and a trailing `.0`
   removed. Percentage is `Math.round(tokens / contextWindow * 100)`. When tokens are unavailable
   but the window is known, the complete line is
   `[session context] unknown/272k; compactions=0`; when no positive window is available it is
   `[session context] unavailable; compactions=0`. The local formatter intentionally differs from
   the subagent display formatter, which coarsens values from 100,000; sharing it would violate the
   approved one-decimal context example and couple independent extension presentation contracts.

8. Context facts are observational only. The context-usage extension never triggers compaction,
   handoff, a warning, or a model turn and never recommends a pressure threshold. Supported
   operation is silent except for the injected model-facing line. Each generated Pi entrypoint,
   including the new one, retains the shared minimum-runtime compatibility guard and its single
   actionable incompatibility notice.

9. Simplify the public `handoff_session` tool schema to exactly one required string property,
   `kickoff`, with no additional properties. The kickoff is trimmed only to establish that it is
   non-empty, retains the existing public `maxLength: 1000` schema bound and execution-time
   1,000-UTF-16-code-unit check, and otherwise crosses the replacement boundary unchanged as the
   prose submitted to the new session.

10. Remove `memoryPath` and all repository, filesystem, effort-slug, ownership, hard-link, size,
    UTF-8, header, and stable-identity validation from the handoff runtime. The extension does not
    infer, read, validate, mutate, or mention effort memory. Workflow guidance remains the sole
    owner of the requirement to checkpoint before a non-minimal effort is handed off and of the
    instruction that a replacement session re-orient from effort memory and repository authority
    when applicable; callers express that continuation instruction in the bounded kickoff prose.

11. Preserve the handoff runtime's model-tool batch exclusivity, supported persisted-TUI check,
    single-use pending request, private FIFO queued command, terminating tool result, five-second
    countdown, cancellation, parent-linked session creation, old-history preservation,
    prepared-child cleanup, pre- and post-replacement failure boundary, automatic kickoff,
    editor fallback, visible recovery notice, and no-silent-retry behavior. Revalidation after the
    countdown now covers only the request and active persisted-session state that replacement still
    depends on, because no filesystem input remains.

12. Add the context-usage entrypoint to the Pi target's governed outputs and to every output-plan,
    descriptor, manifest, cleanup, drift, target-sensitive hash, generated checkout, Sundial,
    editor-quiet strip, and container-coverage assertion that enumerates Pi extension files. A
    target set without Pi renders none of these outputs. The new template remains coherent under
    missingkey=zero rendering with empty project variables and cannot emit a no-value token.

13. Add `rendering/pi-runtime:pi-context-usage-injection` as an invariant with `Backing: test`.
    Deterministic TypeScript tests cover per-model-call injection, tool-follow-up refresh, exact
    formatting including unavailable usage, active-branch-only compaction counting, model-window
    changes, non-persistence, absence of warnings and side effects, and the shared compatibility
    guard. Its implementation batch adds the required proof annotation in an `internal/...` test
    file and names the test unit that proves the claim. Handoff tests replace memory-validation
    coverage with the one-property schema, exact prose propagation, UTF-16 bound, and retained
    queue, exclusivity, countdown, cancellation, lineage, cleanup, and recovery behavior. The
    pinned real-runtime smoke proves transient delivery into actual model requests and refresh
    after compaction without a persisted context-usage message.

14. Update the workflow and checkpoint partials, AGENTS.md, architecture, workflow,
    working-with-awf, glossary, testing, rendering-domain, output-plan, pruning, release-smoke, and
    generated adopter documentation in the same checked implementation batches as the behavior
    they describe. Every lifecycle transition runs `./x render` and stages the regenerated
    `docs/decisions/INDEX.md` and lock output in the same commit. Historical ADRs remain frozen;
    this record corrects their current-state claims forward.

## State changes

- update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- update `rendering/pi-workflows:pi-session-handoff-workflow`
- update `rendering/pi-workflows:pi-session-handoff-lifecycle`
- update `rendering/pi-workflows:pi-session-handoff-public-contract`
- add `rendering/pi-runtime:pi-context-usage-injection`
- update `rendering/pi-runtime:pi-extension-target-render`
- update `rendering/pi-runtime:pi-minimum-runtime`
- update `rendering/pi-runtime:pi-real-runtime-smoke`

## Consequences

Checkpoint durability no longer forces session churn. An agent can retain useful accumulated
context for adjacent work while still creating a fresh parent-linked session before a large or
context-sensitive successor. The concise facts stay current during long tool-driven turns, so the
choice can account for actual model-window use and compaction history without reconstructing them
from stale conversation.

The policy deliberately relies on agent judgment. Two agents can make different handoff choices at
the same usage level, and a poor choice can retain noise too long or discard useful conversational
context early. Mandatory checkpoints, explicit safe-point eligibility, neutral facts, preserved
parent history, and the absence of automatic pressure actions bound that risk without inventing a
threshold the runtime cannot observe.

The handoff tool becomes smaller and independent of repository layout, effort identity, and
filesystem races. That also removes runtime enforcement of a useful discipline: a caller can submit
prose without first updating memory. The workflow's mandatory checkpoint contract and its tests are
therefore the enforcement surface; the runtime intentionally does not provide a second, partial
policy implementation.

The new extension adds one generated executable output and one short transient message to every
model request. It does not grow persisted sessions or telemetry state, but it slightly increases
request tokens and widens target, example, container, documentation, and runtime-smoke coverage.
Keeping the line bounded and the extension single-purpose limits both costs.

Handoff logging moves from the checkpoint before replacement to the replacement session's first
memory update. That keeps the log truthful but creates a short window in which an actual replacement
has occurred and its log line is not yet durable; editor recovery prose preserves the instruction
when automatic kickoff submission fails.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep automatic handoff after every checkpoint | Preserves the current simple rule but discards relevant context and creates session churn even when replacement has no benefit. |
| Make checkpointing discretionary together with handoff | Conflates durability with context control and weakens recovery and memory hygiene. |
| Trigger handoff at a fixed token percentage or compaction count | Pi does not expose the configured compaction threshold through the usage API, and one numeric policy cannot account for upcoming work or the relevance of retained context. |
| Put context facts in the handoff extension | Couples observation to a tool that may never be called and blurs the ownership of passive per-turn facts with session replacement. |
| Put context facts in the subagent extension | Mixes parent-session context observation with isolated child-process orchestration and model routing. |
| Inject once in `before_agent_start` | Misses later model calls after tool results, where token use or a compaction can change during the same agent run. |
| Persist a custom context-usage message or telemetry entry | Makes an ephemeral observation stale, grows history, and recreates state collection with no durable consumer. |
| Keep optional `memoryPath` validation in the handoff tool | Retains repository and effort policy in the replacement mechanism after the workflow has already established the checkpoint contract. |
| Accept arbitrary unbounded kickoff data | Enlarges the replacement and recovery surface and can inject excessive context into the new session. |

## Status history

- 2026-08-02: Proposed
- 2026-08-02: Implementing; content-sha256: 5e5a4c336cf6c9948c51e44bd7003555f0cfecc731c4cffda9f6f5fa076e76a6
- 2026-08-02: Applied; operations: update `rendering/workflow-skill-templates:memory-checkpoint-chain-coverage`
- 2026-08-02: Amended; content-sha256: 03f0a8150b3a0467dcb0dd01ea809c91ef80a06d95a534b58b88a7428224a254
- 2026-08-02: Applied; operations: update `rendering/pi-workflows:pi-session-handoff-workflow`
- 2026-08-02: Applied; operations: update `rendering/pi-workflows:pi-session-handoff-lifecycle`, update `rendering/pi-workflows:pi-session-handoff-public-contract`
- 2026-08-02: Applied; operations: add `rendering/pi-runtime:pi-context-usage-injection`, update `rendering/pi-runtime:pi-extension-target-render`, update `rendering/pi-runtime:pi-minimum-runtime`
- 2026-08-02: Applied; operations: update `rendering/pi-runtime:pi-real-runtime-smoke`
- 2026-08-02: Implemented; content-sha256: 03f0a8150b3a0467dcb0dd01ea809c91ef80a06d95a534b58b88a7428224a254
