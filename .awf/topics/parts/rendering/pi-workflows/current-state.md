Pi workflow contracts: governed subagent tools, session handoff, the workflow dashboard, and structured exploration dispatch.

## Claims

### `invariant: pi-session-handoff-lifecycle`

The Pi handoff lifecycle queues a single-use continuation after model settlement, presents and cleans up a cancellable five-second countdown, revalidates the memory path, replaces with a persisted parent-linked session, submits kickoff only through the replacement context, retains an editor fallback, and states the truthful nontransactional teardown boundary without deleting sessions or memory.
Origin: ADR-0148
Backing: test

### `invariant: pi-dedicated-grounding-dispatch`

In the generated Pi extension and skills, brainstorming's grounding check dispatches through the dedicated grounding tool while general exploration and coupling audits use the exploration tool, and no non-Pi target's rendered output contains either Pi subagent tool name.
Origin: ADR-0148
Backing: test

### `invariant: pi-extension-editor-quiet-strip`

Every governed Pi extension file carries the ts-nocheck directive on the line immediately after the provenance banner, and the container test harness deterministically strips that exact directive from every extension TypeScript file in its ephemeral copy after source copy and before running the TypeScript compiler.
Origin: ADR-0148
Backing: test

### `invariant: pi-implementation-batch-exclusivity`

Pi correlates each tool preflight with the current leaf assistant tool-call id, blocks every member of a reconstructable batch that mixes implementation with siblings, and blocks only implementation when trustworthy batch context is unavailable.
Origin: ADR-0148
Backing: test

### `invariant: pi-session-handoff-public-contract`

The generated Pi handoff extension exposes exactly the closed memoryPath and bounded kickoff schema, confines canonical no-symlink paths to regular files below .awf/memory, requires a persisted TUI and an exclusive trustworthy tool batch, keeps one correlated pending request, queues its private command, and terminates the calling model turn.
Origin: ADR-0148
Backing: test

### `invariant: pi-session-handoff-workflow`

Pi-rendered checkpoint guidance automatically invokes handoff_session alone after the durable visible summary at phase and intermediate implementation checkpoints, while non-Pi targets retain the checkpoint and continue without naming the unsupported tool.
Origin: ADR-0148
Backing: test

### `invariant: pi-structured-exploration-contract`

The generated Pi extension exposes exactly four closed-schema roles, each with optional exact model routing; exploration retains required task, breadth, and detail and runs through the ten-active FIFO limiter without changing the other process boundaries.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-failure-details`

In the generated Pi extension, expected failures that occur after a child process has started return a marked error result that preserves bounded progress and diagnostics through a tool_result middleware hook instead of throwing, while retaining cancellation, cleanup, and implementation-commit-policy behavior.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-model-routing`

Every Pi subagent role accepts an optional exact provider/model-id, inherits the parent on omission, rejects unknown or unauthenticated explicit choices without fallback before queueing, inherits thinking for child clamping, and reports requested and actual models.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-bounds`

The generated Pi extension retains at most 20 display events of at most 2 KiB each, reports cumulative omitted-event counts and truncation explicitly, and never keeps a second raw child-transcript store.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-context-isolation`

The generated Pi extension carries intermediate child activity only in bounded tool details, never appending it to parent model-visible content or custom session messages, and a subagent tool's final content contains only the child report or a bounded failure summary.
Origin: ADR-0148
Backing: test

### `invariant: pi-subagent-progress-rendering`

In the generated Pi extension, every public subagent tool's collapsed view renders status, recent bounded activity, omission state, and available usage, and its expanded view additionally renders the task, retained activity, the final report, present diagnostics, and available usage from the same structured details without changing execution.
Origin: ADR-0148
Backing: test

### `invariant: pi-workflow-dashboard-public-contract`

The Pi templates publish one descriptor-derived telemetry vocabulary and a five-file extension surface whose three factories exchange only bounded versioned observations and validated active-branch association. The dashboard exposes explicit lifecycle and query-only metrics/doctor tools, a compact optional widget, an on-demand overview/phases/history/findings/maintenance overlay, controlled canonical refresh with visible stale or degraded state, and confirmed fixed-argument repair, waiver, retention, and purge actions; it provides no automatic score, blocking diagnosis, reconciliation, daemon, or local historical aggregation.
Origin: ADR-0148
Backing: test
