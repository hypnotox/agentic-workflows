The Pi runtime floor and its boundaries: child-process safety, tool boundaries, target rendering, and real-runtime smoke coverage.

## Claims

### `invariant: pi-child-process-safety`

In the generated Pi subagent extension, every child exit path removes the temporary role prompt and its listeners, cancellation escalates from TERM to KILL based on the observed process exit, and child errors preserve bounded diagnostics.
Origin: ADR-0148
Backing: test

### `invariant: pi-child-tool-boundaries`

Pi subagent children use an explicitly selected validated model, a validated configured preference, or inherit the parent; inherit the parent's thinking level; receive fixed role allowlists excluding extension tools; and enforce fixed retained-output limits with explicit truncation diagnostics.
Origin: ADR-0148
Revised-by: ADR-0151
Backing: test

### `invariant: pi-context-usage-injection`

Before every Pi model call, including tool-follow-up calls, the standalone context-usage extension appends exactly one non-persisted model-facing line reporting current tokens against the active model window and the compaction count from the active session branch. It formats finite values below 1,000 as rounded integers, values from 1,000 in trimmed one-decimal base-1,000 `k` units, and values from 1,000,000 in trimmed one-decimal base-1,000,000 `m` units; computes percentage by rounding `tokens / contextWindow * 100`; and emits the deterministic unknown-token or unavailable-window form. The extension never persists a message or entry, writes a file or telemetry record, changes UI, triggers a model turn, compaction, warning, or handoff, or recommends a pressure threshold.
Origin: ADR-context-aware-discretionary-pi-handoffs
Backing: test

### `invariant: pi-extension-target-render`

Enabling Pi renders the standalone context-usage and handoff entrypoints plus the subagent index, bounded model-routing module, and runner with provenance. Context usage owns transient per-model-call fact injection, handoff owns parent-linked main-session replacement, model routing owns pure preference policy, and the subagent entrypoint retains tool registration, queueing, process lifecycle, and runtime integration. No telemetry or workflow-router output renders, and every file follows normal output-plan, drift, cleanup, target-sensitive hash, generated-checkout, adopter-example, editor-quiet, and container-coverage semantics; a target set without Pi renders none of them.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-context-aware-discretionary-pi-handoffs
Backing: test

### `invariant: pi-implementation-state-boundary`

Pi implementation subagent calls serialize against one another, enforce the caller-selected commit permission - reporting a changed HEAD under a no-commit permission as a policy violation without auto-reverting - and report starting and ending git state, marking commit verification unavailable outside a git checkout.
Origin: ADR-0148
Backing: test

### `invariant: pi-minimum-runtime`

Every generated Pi extension entrypoint requires the minimum Pi runtime APIs used by its retained contract, reports the shared single actionable incompatibility notice, and fails before registering functional hooks when required APIs are absent. Supported context-usage, handoff, and subagent operation emits no compatibility, pressure, or handoff warning.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0167, ADR-context-aware-discretionary-pi-handoffs
Backing: test

### `invariant: pi-real-runtime-smoke`

Pinned Pi runtime smoke covers generated TypeScript loading, native Pi skill discovery, effort-independent handoff, and before-agent-start routing-card delivery into the model request without a persisted session message, and verifies telemetry, router, and selection surfaces are absent.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167, ADR-0173
Backing: unbacked
Verify: Run `./x pi-test run` to exercise native Pi skill discovery, effort-independent handoff, and routing-card delivery into a real pinned Pi model request without session-message persistence, with no telemetry, router, or selection.
