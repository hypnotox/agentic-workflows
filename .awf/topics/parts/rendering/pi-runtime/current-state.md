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
Origin: ADR-0209
Backing: test

### `invariant: pi-extension-target-render`

The fixed Pi target renders the standalone context-usage and handoff entrypoints plus the subagent index, bounded model-routing module, and runner with provenance; `effort-workflow` additionally renders the Pi-target-owned `using-effort` skill and `awf-effort` index/client pair through the same output predicate. The effort client alone strictly invokes and decodes activity protocol v2 and owner-scoped memory protocol v1 through bounded transport; its index owns direct serialized association, fixed-path transient context, dynamic memory-tool activation, Pi file-queue participation, heartbeat/shutdown lifecycle, and Remote Pi translation. Context usage owns transient per-model-call usage facts, handoff owns parent-linked main-session replacement, model routing owns pure preference policy, and the subagent entrypoint retains tool registration, queueing, process lifecycle, and runtime integration. No telemetry or workflow-router output renders, and every file follows normal output-plan, drift, cleanup, target-sensitive hash, generated-checkout, adopter-example, editor-quiet, and container-coverage semantics.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-0209, ADR-0218, ADR-0225, ADR-0239, ADR-0251
Backing: test

### `invariant: pi-implementation-state-boundary`

Pi implementation subagent calls serialize against one another and enforce the caller-selected commit permission against an optional invocation-owned verification checkout, defaulting to the project root. An explicit identity resolves relative to the project root after one leading `@` is removed, canonicalizes filesystem aliases, and must be an exact live checkout root whose Git common directory matches the project root; invalid identities refuse before child dispatch without parsing worktree topology. Both snapshots and permission directions use the resolved checkout and expose it in structured details and diagnostics, while parent and child Pi CWD, role loading, effort association, and mutation routing remain rooted and unchanged. A changed selected HEAD under a no-commit permission is a policy violation without auto-reverting, and an unverifiable omitted-root checkout reports commit verification unavailable.
Origin: ADR-0148
Revised-by: ADR-0260
Backing: test

### `invariant: pi-minimum-runtime`

Generated Pi extension entrypoints require the retained lock-pinned fork-v0.81.1-awf.3 runtime (embedded version 0.81.1) APIs used by context usage, subagents, handoff, and effort memory tools. This includes `getActiveTools`, `setActiveTools`, tool prompt guidance, and `withFileMutationQueue`; one actionable incompatibility notice occurs before functional registration when any required API is absent. The direct `using_effort` companion needs no `changeCwd` capability; optional Remote Pi events remain advisory. Supported operation emits no compatibility warning.
Origin: ADR-0148
Revised-by: ADR-0162, ADR-0167, ADR-0209, ADR-0218, ADR-0219, ADR-0225, ADR-0239
Backing: test

### `invariant: pi-real-runtime-smoke`

Pinned Pi runtime smoke covers generated TypeScript loading, native Pi skill discovery, prose-only effort-independent handoff, before-agent-start routing-card delivery, and transient context-usage delivery into actual model requests with refresh after an active-branch compaction. Routing and context facts do not persist as session messages, and telemetry, workflow-router, selection, context-usage UI, and automatic pressure-action surfaces remain absent.
Origin: ADR-0148
Revised-by: ADR-0149, ADR-0161, ADR-0162, ADR-0164, ADR-0167, ADR-0173, ADR-0209
Backing: unbacked
Verify: Run `./x pi-test run` to exercise native Pi skill discovery, prose-only effort-independent handoff, routing-card delivery, and per-request context-usage refresh after compaction in the pinned Pi runtime without persisted routing or context messages, telemetry, workflow routing, selection, context-usage UI, or automatic pressure actions.
