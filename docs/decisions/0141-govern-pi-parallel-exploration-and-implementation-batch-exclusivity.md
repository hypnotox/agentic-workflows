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

Every subagent child currently inherits the parent model and thinking level. That is a safe default but
can make broad exploration fanout unnecessarily expensive and prevents the orchestrator from matching
model capacity to role difficulty. Pi's child CLI accepts a selected provider/model while the parent
extension's model registry can reject an unknown or unauthenticated exact selection before scheduling.
The four existing role tools can therefore expose one consistent opt-in routing field without adding a
fifth public tool or a project configuration surface.

Pi implementation children share the parent's checkout. ADR-0123 requires callers to place
`subagent_implement` alone in a parent tool batch, but the extension enforces only serialization among
implementation children. Prompt guidance cannot prevent an assistant from emitting implementation
beside a sibling `edit`, `write`, `bash`, or delegation call, so concurrent checkout mutation remains
possible inside the same assistant turn. Pi exposes the assistant tool-call batch to extension events;
the implementation tool can therefore enforce the existing isolation rule before any child starts.

The extension must retain its exact four-tool public contract and use Pi's native sibling tool calls.
Concurrency policy is local scheduling state, not a new public batching API. Grounding established that
implementation batch behavior belongs to the `rendering/templates` topic: the existing
`rendering/catalog-and-targets:pi-implementation-state-boundary` claim remains true but its
`internal/catalog/**` scope does not own extension-template behavior. Model selection also requires
narrowing the catalog-scoped child-boundaries claim, whose unconditional parent-model inheritance
clause will no longer be true, and adding template-scoped routing authority.

## Decision

1. Add an optional `model` string to each of `subagent_grounding`, `subagent_explore`,
   `subagent_review`, and `subagent_implement`. The value is an exact canonical
   `provider/model-id` reference; model ids may contain further slashes. When omitted, the child
   inherits the parent model exactly as today. This changes all four parameter schemas but preserves
   their existing required role fields, rejects additional properties, and retains exactly four
   public tools.

2. Resolve an explicit selection against the parent Pi model registry and require configured
   authentication before the call enters a role queue or starts a child. Reject an empty, malformed,
   unknown, or unauthenticated selection with an actionable error and never silently fall back to the
   parent or another model. Pass a valid canonical selection to the child CLI. Continue to inherit the
   parent's thinking level; the child Pi runtime applies its normal capability clamp for the selected
   model. Preserve the requested and actual model in diagnostics and usage reporting.

3. Permit a parent to dispatch independent `subagent_explore` calls concurrently through native Pi
   sibling tool calls. Keep one information need per call and keep refinement of an earlier result
   sequential. Do not add a batch tool or change exploration's required task, breadth, and detail
   semantics.

4. Limit each generated extension instance to ten active exploration children. Queue additional
   exploration calls in FIFO order. Ten is a fixed operational safety ceiling chosen to support broad
   fanout on lower-cost models while still bounding local subprocess and provider pressure; it is not
   claimed as a measured optimum. The limiter is session-local extension state: grounding and review
   calls do not consume its slots, and implementation retains its separate serialization queue.

5. Make exploration queueing abort-aware. A call cancelled while queued is removed without spawning a
   child. Every acquired slot is released in `finally` on success, child failure, cancellation, and
   setup failure so later waiters cannot be stranded.

6. Enforce implementation batch exclusivity at Pi's real `tool_call` event seam. Correlate the current
   event's tool-call id with the current leaf assistant message, verify its tool-call content, and
   identify the complete sibling batch. If a reconstructable batch contains `subagent_implement` and
   any other tool call, block every call in that batch before execution and return an actionable
   retry-alone error. This deliberately blocks harmless read-only siblings as well as mutating ones so
   the rule is structural and deterministic. Do not scan an older "latest assistant" message, because
   stale correlation could authorize or block the wrong turn.

7. Fail closed narrowly when batch context is malformed or unavailable: block
   `subagent_implement`, but do not reject unrelated non-implementation tools whose batch cannot be
   reconstructed. This fallback can allow a sibling mutation because its relationship to the blocked
   implementation call is unknowable; implementation itself remains unable to start. Preserve
   implementation serialization, commit-permission enforcement, git-state reporting, and no-rollback
   behavior after a call passes the batch guard.

8. Update Pi-only workflow guidance to permit parallel dispatch only for independent exploration
   calls, explain optional per-call model selection, and retain the implementation-alone rule. Update
   `templates/agents-doc/AGENTS.md.tmpl` and its synced generated `AGENTS.md` in the same
   behavior-changing transaction. Preserve non-Pi target-native wording and schemas.

9. Record, without implementing in this effort, a session dashboard, a fresh-session handoff command,
   and phase-sensitive tool activation in `.awf/docs/parts/roadmap/ideas.md`, then sync the generated
   `docs/roadmap.md`. Model routing is no longer a roadmap candidate because this decision implements
   it.

10. Test exact model routing on all four roles, inherited fallback, canonical ids containing slashes,
    registry and authentication rejection, thinking-level forwarding, and requested/actual-model
    diagnostics. Test the exploration limiter's ten-active maximum, FIFO ordering, queued
    cancellation, and slot release on every exit path in the TypeScript harness. Exercise the real Pi
    batch event seam with accepted singleton implementation, rejected reconstructable mixed batches
    in which every call is blocked, stale or malformed context, and the narrow fail-closed fallback.
    Go generated-output tests prove Pi-only guidance, unchanged non-Pi rendering, exact revised public
    schemas, and the revised and added invariant claims. Every modified template must render coherent
    prose with empty variables and emit no unresolved or no-value token. The implementation transaction
    runs sync, staged topic-transition checks, and the full gate.

11. Every Accepted or Implemented status transaction runs `./x sync` and commits the regenerated
    `docs/decisions/INDEX.md` with the ADR and any applied current-state claim changes.

## State changes

- update `rendering/templates:bounded-exploration-reporting`
- update `rendering/templates:pi-structured-exploration-contract`
- update `rendering/catalog-and-targets:pi-child-tool-boundaries`
- add `rendering/templates:pi-subagent-model-routing`
- add `rendering/templates:pi-implementation-batch-exclusivity`

## Consequences

Independent investigations can reduce wall-clock exploration time, and explicit lower-cost model
selection can reduce fanout cost. A ten-child ceiling permits substantial parallel load, so adopters
remain responsible for provider rate limits and for choosing models capable of their assigned tasks.
FIFO scheduling is predictable, but a slow group of ten calls can hold later work; the queue
deliberately does not attempt priorities or cross-session coordination.

All roles gain consistent model routing at the cost of changing all four public schemas and placing
model suitability in the orchestrator's hands. Omission is backward-compatible at the semantic level,
while strict explicit resolution prevents a cost-sensitive request from silently running on a more
expensive fallback. A selected model may clamp inherited thinking or produce worse work; actual-model
reporting makes the executed choice visible.

Implementation's shared-checkout boundary becomes mechanically enforced for reconstructable
assistant-generated tool batches rather than advisory only. Blocking the whole disallowed batch
prevents its siblings from executing, including harmless reads. When trustworthy batch context is
unavailable, the narrow fallback guarantees only that implementation cannot start; an unrelated
sibling may still mutate the checkout.

The extension gains event-model coupling, model-registry coupling, and concurrency state that require
deterministic lifecycle and cancellation tests. Native sibling calls preserve Pi's normal tool UI and
exact tool count, but awf does not provide one aggregate result or cancellation handle for a group.
Other runtimes retain their existing target-native behavior; this decision does not promise
cross-runtime model routing or a concurrency limit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep all exploration sequential | It leaves independent, context-isolated investigations unnecessarily serialized. |
| Retain a cap of four | It is conservative for inherited expensive models but unnecessarily restricts deliberate lower-cost fanout. |
| Use another fixed cap such as two or eight | Lower caps provide less useful fanout; ten is still a simple bounded ceiling and matches the intended maximum breadth. |
| Allow unbounded parallel exploration | Provider and subprocess pressure would be controlled only by environmental failure. |
| Add an awf exploration-batch tool | It expands the exact tool count and duplicates Pi's native sibling-call mechanism. |
| Configure the limit in project config | A fixed operational safety bound avoids a new schema and adopter tuning surface. |
| Keep parent-model inheritance only | It preserves schemas but prevents deliberate cost and role-capability routing. |
| Add model selection only to exploration | It addresses the immediate fanout cost but leaves an inconsistent role API and blocks useful routing for grounding, review, and implementation. |
| Configure one model per role in project config | Static routing is less flexible than per-call task judgment and adds a project schema surface. |
| Silently fall back when an explicit model is unavailable | It can violate the caller's cost or capability intent without making the executed choice clear. |
| Reject only implementation in a reconstructable mixed batch | Sibling checkout mutation could still proceed concurrently after the rejection. |
| Rely on implementation prompt guidance | Guidance is not enforcement and cannot prevent a model-emitted mixed batch. |
| Scan the latest assistant message without id correlation | A stale message can misclassify the current tool call and weaken fail-closed behavior. |
| Add batch exclusivity to `pi-implementation-state-boundary` | That claim's catalog topic does not own behavior implemented under `templates/**`. |

## Status history

- 2026-07-21: Proposed
