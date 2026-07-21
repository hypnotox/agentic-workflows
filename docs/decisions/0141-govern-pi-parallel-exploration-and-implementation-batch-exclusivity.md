---
format: current-state-v1
status: Proposed
date: 2026-07-21
---
# ADR-0141: Govern Pi Parallel Exploration and Implementation Batch Exclusivity


## Context

Pi's structured exploration workflow currently describes parent-driven sequential refinement, and the
generated extension starts every exploration call immediately. Independent repository questions can be
investigated concurrently without weakening the fresh-context or one-information-need boundaries, but
unbounded sibling calls can exhaust model, subprocess, and provider capacity. The workflow needs a
small, deterministic concurrency boundary rather than either prohibiting useful parallelism or relying
on provider failure as backpressure.

Pi implementation children share the parent's checkout. ADR-0123 requires callers to place
`subagent_implement` alone in a parent tool batch, but the extension enforces only serialization among
implementation children. Prompt guidance cannot prevent an assistant from emitting implementation
beside a sibling `edit`, `write`, `bash`, or delegation call, so concurrent checkout mutation remains
possible inside the same assistant turn. Pi exposes the assistant tool-call batch to extension events;
the implementation tool can therefore enforce the existing isolation rule before any child starts.

The extension must retain its exact four-tool public contract and use Pi's native sibling tool calls.
The concurrency policy is local scheduling state, not a new public batching API or project
configuration surface. Grounding also established that implementation batch behavior belongs to the
`rendering/templates` topic: the existing
`rendering/catalog-and-targets:pi-implementation-state-boundary` claim remains true but its
`internal/catalog/**` scope does not own extension-template behavior.

## Decision

1. Permit a parent to dispatch independent `subagent_explore` calls concurrently through native Pi
   sibling tool calls. Keep one information need per call and keep refinement of an earlier result
   sequential. Do not add a batch tool, change exploration's `{task, breadth, detail}` schema, or change
   the exact four-tool public contract.

2. Limit each generated extension instance to four active exploration children. Queue additional
   exploration calls in FIFO order. The limiter is session-local extension state: grounding and review
   calls do not consume its slots, and implementation retains its separate serialization queue.

3. Make exploration queueing abort-aware. A call cancelled while queued is removed without spawning a
   child. Every acquired slot is released in `finally` on success, child failure, cancellation, and
   setup failure so later waiters cannot be stranded.

4. Enforce implementation batch exclusivity at Pi's real tool-call event seam. Correlate the current
   event's tool-call id with the current leaf assistant message, verify its tool-call content, and
   identify the complete sibling batch. If a batch contains `subagent_implement` and any other tool
   call, block every call in that batch before execution and return an actionable retry-alone error.
   Do not scan an older "latest assistant" message, because stale correlation could authorize or block
   the wrong turn.

5. Fail closed narrowly when batch context is malformed or unavailable: block
   `subagent_implement`, but do not reject unrelated non-implementation tools whose batch cannot be
   reconstructed. Preserve implementation serialization, commit-permission enforcement, git-state
   reporting, and no-rollback behavior after a call passes the batch guard.

6. Update Pi-only workflow guidance to permit parallel dispatch only for independent exploration calls
   and to retain the implementation-alone rule. Preserve non-Pi target-native wording and schemas.
   Record, without implementing in this effort, future candidates for a session dashboard, a
   fresh-session handoff command, role-specific child model routing, and phase-sensitive tool
   activation.

7. Test the exploration limiter's four-active maximum, FIFO ordering, queued cancellation, and slot
   release on every exit path in the TypeScript harness. Exercise the real Pi batch event seam with
   accepted singleton implementation, rejected mixed batches in which every call is blocked, stale or
   malformed context, and the narrow fail-closed fallback. Go generated-output tests prove Pi-only
   guidance, unchanged non-Pi rendering, unchanged public schemas, and the revised and added invariant
   claims. The implementation transaction runs sync, staged topic-transition checks, and the full
   gate.

## State changes

- update `rendering/templates:bounded-exploration-reporting`
- update `rendering/templates:pi-structured-exploration-contract`
- add `rendering/templates:pi-implementation-batch-exclusivity`

## Consequences

Independent investigations can reduce wall-clock exploration time while a fixed local cap bounds
subprocess and provider pressure. FIFO scheduling is predictable, but a slow group of four calls can
hold later work; the queue deliberately does not attempt priorities or cross-session coordination.

Implementation's shared-checkout boundary becomes mechanically enforced for assistant-generated tool
batches rather than advisory only. Blocking the whole malformed mixed batch prevents sibling mutation
from proceeding after implementation is rejected. The narrow fallback favors checkout safety without
making unrelated tools unavailable when Pi cannot provide trustworthy batch context.

The extension gains event-model coupling and concurrency state that require deterministic lifecycle
and cancellation tests. Native sibling calls preserve Pi's normal tool UI and exact awf API, but awf
does not provide one aggregate result or cancellation handle for a group. Other runtimes retain their
existing target-native behavior; this decision does not promise a cross-runtime concurrency limit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep all exploration sequential | It leaves independent, context-isolated investigations unnecessarily serialized. |
| Allow unbounded parallel exploration | Provider and subprocess pressure would be controlled only by environmental failure. |
| Add an awf exploration-batch tool | It expands the exact public contract and duplicates Pi's native sibling-call mechanism. |
| Configure the limit in project config | A fixed operational safety bound avoids a new schema and adopter tuning surface. |
| Reject only implementation in a mixed batch | Sibling checkout mutation could still proceed concurrently after the rejection. |
| Rely on implementation prompt guidance | Guidance is not enforcement and cannot prevent a model-emitted mixed batch. |
| Scan the latest assistant message without id correlation | A stale message can misclassify the current tool call and weaken fail-closed behavior. |
| Add batch exclusivity to `pi-implementation-state-boundary` | That claim's catalog topic does not own behavior implemented under `templates/**`. |

## Status history

- 2026-07-21: Proposed
